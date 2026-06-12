package main

import (
	"context"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"watchflare/backend/cache"
	"watchflare/backend/config"
	"watchflare/backend/database"
	grpcservice "watchflare/backend/grpc"
	"watchflare/backend/handlers"
	"watchflare/backend/logger"
	"watchflare/backend/middleware"
	"watchflare/backend/pki"
	"watchflare/backend/services"
	pb "watchflare/shared/proto/agent/v1"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const httpPort = "8080"

func main() {
	// Initialize logger first so all subsequent output uses the clean format
	logger.Init()

	// Load configuration
	config.Load()

	// Ensure data directory exists (required for FROM scratch container)
	dataDir := "/var/lib/watchflare"
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		slog.Warn("failed to create data directory", "path", dataDir, "error", err)
	}

	// Connect to database
	if err := database.Connect(config.AppConfig.DatabaseURL); err != nil {
		logger.Fatal("failed to connect to database", "error", err)
	}

	// Initialize PKI (auto-generate or validate custom certs)
	pkiConfig := &pki.Config{
		Mode:   pki.Mode(config.AppConfig.TLSMode),
		PKIDir: config.AppConfig.TLSPKIDir,

		// Custom mode fields
		CertFile: config.AppConfig.TLSCertFile,
		KeyFile:  config.AppConfig.TLSKeyFile,
		CAFile:   config.AppConfig.TLSCAFile,
	}

	pkiInstance, err := pki.New(pkiConfig)
	if err != nil {
		logger.Fatal("failed to create PKI instance", "error", err)
	}

	if err := pkiInstance.Initialize(); err != nil {
		logger.Fatal("failed to initialize PKI", "error", err)
	}

	// Store PKI instance in context for gRPC server and handlers
	grpcservice.SetPKI(pkiInstance)

	// Start heartbeat cache workers
	// Sync worker: writes cache to DB every 5 minutes
	syncWorker := cache.NewSyncWorker(5 * time.Minute)
	go syncWorker.Start()

	// Stale checker: marks agents offline if no heartbeat for 15s (3x 5s interval)
	staleChecker := cache.NewStaleChecker(10*time.Second, 15*time.Second)
	go staleChecker.Start()

	// Start aggregated metrics scheduler (broadcasts via SSE every 30s)
	aggregatedMetricsScheduler := services.NewAggregatedMetricsScheduler(30 * time.Second)
	go aggregatedMetricsScheduler.Start()

	// Start version checker (fetches latest agent version from GitHub every 6h)
	versionCtx, versionCancel := context.WithCancel(context.Background())
	defer versionCancel()
	services.StartVersionChecker(versionCtx)

	// Start alert worker (evaluates alert rules every 30s)
	alertWorker := services.NewAlertWorker(30 * time.Second)
	go alertWorker.Start()

	// Setup HTTP server
	router := setupRouter()
	httpServer := &http.Server{
		Addr:    ":" + httpPort,
		Handler: router,
	}

	// Start HTTP server
	go func() {
		slog.Info("HTTP server starting", "port", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed", "error", err)
		}
	}()

	// Start gRPC server
	grpcServer, err := createGRPCServer(config.AppConfig, pkiInstance)
	if err != nil {
		logger.Fatal("failed to create gRPC server", "error", err)
	}

	grpcListener, err := net.Listen("tcp", ":"+config.AppConfig.GRPCPort)
	if err != nil {
		logger.Fatal("failed to listen on gRPC port", "error", err)
	}

	go func() {
		slog.Info("gRPC server starting", "port", config.AppConfig.GRPCPort)
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Fatal("gRPC server failed", "error", err)
		}
	}()

	slog.Info("Watchflare backend started")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	sig := <-sigCh
	slog.Info("shutting down gracefully", "signal", sig)

	// Graceful shutdown with 10s timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop workers
	syncWorker.Stop()
	staleChecker.Stop()
	aggregatedMetricsScheduler.Stop()
	alertWorker.Stop()

	// Stop servers
	grpcServer.GracefulStop()
	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	slog.Info("shutdown complete")
}

func setupRouter() *gin.Engine {
	// Set Gin mode
	if config.AppConfig.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// Configure CORS
	corsConfig := cors.Config{
		AllowOrigins:     config.AppConfig.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}
	router.Use(cors.New(corsConfig))

	// API routes under /api/v1 prefix
	api := router.Group("/api/v1")

	// Health check endpoint
	api.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	// Public app configuration (used by frontend at startup)
	api.GET("/config", handlers.GetAppConfig)

	// Auth routes (public)
	authGroup := api.Group("/auth")
	{
		authGroup.GET("/setup-required", handlers.SetupRequired)
		authGroup.POST("/register", handlers.Register)
		authGroup.POST("/login", handlers.Login)
		authGroup.POST("/logout", handlers.Logout)
		authGroup.POST("/verify-totp", handlers.VerifyTOTP)
	}

	// Protected routes (require JWT)
	protectedGroup := api.Group("/auth")
	protectedGroup.Use(middleware.AuthMiddleware())
	{
		protectedGroup.GET("/user", handlers.GetCurrentUser)
		protectedGroup.PUT("/preferences", handlers.UpdatePreferences)
		protectedGroup.PUT("/change-password", handlers.ChangePassword)
		protectedGroup.PUT("/change-email", handlers.ChangeEmail)
		protectedGroup.PUT("/change-username", handlers.ChangeUsername)
	}

	// 2FA routes (protected)
	twoFAGroup := api.Group("/2fa")
	twoFAGroup.Use(middleware.AuthMiddleware())
	{
		twoFAGroup.POST("/setup", handlers.SetupTOTP)
		twoFAGroup.POST("/enable", handlers.EnableTOTPHandler)
		twoFAGroup.DELETE("", handlers.DisableTOTPHandler)
		twoFAGroup.POST("/backup-codes/regenerate", handlers.RegenerateBackupCodesHandler)
	}

	// Settings routes (protected)
	settingsGroup := api.Group("/settings")
	settingsGroup.Use(middleware.AuthMiddleware())
	{
		settingsGroup.GET("/smtp", handlers.GetSMTPSettings)
		settingsGroup.PUT("/smtp", handlers.UpdateSMTPSettings)
		settingsGroup.POST("/smtp/test", handlers.TestSMTPConnection)
		settingsGroup.GET("/alerts", handlers.GetAlertRules)
		settingsGroup.PUT("/alerts", handlers.UpdateAlertRules)
		settingsGroup.GET("/alerts/active", handlers.GetActiveIncidents)
		settingsGroup.GET("/alerts/incidents", handlers.GetAllIncidents)
		settingsGroup.GET("/webhooks", handlers.GetWebhooks)
		settingsGroup.POST("/webhooks", handlers.AddWebhook)
		settingsGroup.DELETE("/webhooks/:id", handlers.DeleteWebhook)
		settingsGroup.PATCH("/webhooks/:id/enabled", handlers.SetWebhookEnabled)
		settingsGroup.POST("/webhooks/:id/test", handlers.TestWebhook)
	}

	// Agent info routes (protected)
	agentGroup := api.Group("/agent")
	agentGroup.Use(middleware.AuthMiddleware())
	{
		agentGroup.GET("/latest-version", handlers.GetLatestAgentVersion)
	}

	// Packages routes (protected) — global view across all hosts
	packagesGroup := api.Group("/packages")
	packagesGroup.Use(middleware.AuthMiddleware())
	{
		packagesGroup.GET("", handlers.ListAllPackages)
	}

	// Host routes (protected)
	hostGroup := api.Group("/hosts")
	hostGroup.Use(middleware.AuthMiddleware())
	{
		// Collection routes (no :id param)
		hostGroup.POST("", handlers.CreateAgent)
		hostGroup.GET("", handlers.ListHosts)
		hostGroup.GET("/events", handlers.HostEvents)
		hostGroup.GET("/:id/events", handlers.HostDetailEvents)
		hostGroup.GET("/stats", handlers.GetHostStats)
		hostGroup.GET("/dropped-metrics", handlers.GetDroppedMetrics)
		hostGroup.GET("/metrics/aggregated", handlers.GetAggregatedMetrics)

		// Per-host routes
		hostGroup.GET("/:id", handlers.GetHost)
		hostGroup.GET("/:id/metrics", handlers.GetMetrics)
		hostGroup.GET("/:id/sensor-readings", handlers.GetSensorReadings)
		hostGroup.GET("/:id/container-metrics", handlers.GetContainerMetrics)
		hostGroup.PUT("/:id/validate-ip", handlers.ValidateIP)
		hostGroup.PUT("/:id/rename", handlers.RenameHost)
		hostGroup.PUT("/:id/change-ip", handlers.UpdateConfiguredIP)
		hostGroup.PUT("/:id/ignore-ip-mismatch", handlers.IgnoreIPMismatch)
		hostGroup.PUT("/:id/dismiss-reactivation", handlers.DismissReactivation)
		hostGroup.PUT("/:id/pause", handlers.PauseHost)
		hostGroup.PUT("/:id/resume", handlers.ResumeHost)
		hostGroup.POST("/:id/regenerate-token", handlers.RegenerateToken)
		hostGroup.DELETE("/:id", handlers.DeleteHost)
		hostGroup.GET("/:id/packages", handlers.GetHostPackages)
		hostGroup.GET("/:id/packages/history", handlers.GetHostPackageHistory)
		hostGroup.GET("/:id/packages/collections", handlers.GetHostPackageCollections)
		hostGroup.GET("/:id/packages/stats", handlers.GetPackageStats)
		hostGroup.POST("/:id/packages/collect", handlers.TriggerPackageCollect)
		hostGroup.POST("/:id/agent/update", handlers.TriggerAgentUpdate)
		hostGroup.GET("/:id/incidents", handlers.GetHostIncidents)
		hostGroup.GET("/:id/alerts", handlers.GetHostAlertRules)
		hostGroup.PUT("/:id/alerts/:metric_type", handlers.UpsertHostAlertRule)
		hostGroup.DELETE("/:id/alerts/:metric_type", handlers.DeleteHostAlertRule)
	}

	// Serve embedded frontend (SPA with fallback to index.html)
	frontendFiles, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		slog.Warn("frontend files not found (dev mode?)", "error", err)
	} else {
		fileServer := http.FileServer(http.FS(frontendFiles))
		router.NoRoute(func(c *gin.Context) {
			// Try to serve the exact file first
			path := c.Request.URL.Path
			f, err := frontendFiles.Open(path[1:]) // strip leading /
			if err == nil {
				f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			// SPA fallback: serve index.html for all non-file routes
			c.Request.URL.Path = "/"
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
		slog.Info("frontend embedded and served from /")
	}

	return router
}

// createGRPCServer initializes the gRPC server (does not start serving)
func createGRPCServer(cfg *config.Config, pkiInstance *pki.PKI) (*grpc.Server, error) {
	var opts []grpc.ServerOption

	// TLS configuration (mandatory)
	tlsConfig, err := pkiInstance.GetTLSConfig()
	if err != nil {
		return nil, err
	}

	creds := credentials.NewTLS(tlsConfig)
	opts = append(opts, grpc.Creds(creds))
	slog.Info("gRPC TLS enabled", "version", "TLS 1.3", "mode", cfg.TLSMode)

	// Authentication interceptor (HMAC mandatory)
	opts = append(opts, grpc.UnaryInterceptor(
		grpcservice.AuthInterceptor(cfg.GRPCTimestampWindow),
	))

	slog.Info("gRPC HMAC validation enabled", "timestamp_window_s", cfg.GRPCTimestampWindow)

	grpcServer := grpc.NewServer(opts...)
	agentService := grpcservice.NewAgentServer()
	pb.RegisterAgentServiceServer(grpcServer, agentService)

	return grpcServer, nil
}
