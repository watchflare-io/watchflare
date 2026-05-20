package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"watchflare-agent/client"
	"watchflare-agent/config"
	"watchflare-agent/errors"
	"watchflare-agent/logger"
	"watchflare-agent/metrics"
	"watchflare-agent/packages"
	"watchflare-agent/sysinfo"
	"watchflare-agent/update"
	"watchflare-agent/wal"

	pb "watchflare/shared/proto/agent/v1"
)

const (
	inventoryTypeFull  = "full"
	inventoryTypeDelta = "delta"
)

// Run starts the agent in normal operation mode
func Run() {
	slog.Info("Watchflare Agent starting", "version", AgentVersion)

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		logger.Fatal("configuration error", "error", err)
	}

	// Apply log level from config (WATCHFLARE_DEBUG env var takes priority)
	if cfg.LogFile != "" {
		if err := logger.InitWithFile(cfg.LogFile, cfg.LogLevel); err != nil {
			logger.Fatal("failed to open log file", "error", err)
		}
	} else {
		logger.SetLevel(cfg.LogLevel)
	}

	// Ensure directories exist
	if err := config.EnsureDirectories(); err != nil {
		logger.Fatal("failed to create directories", "error", err)
	}

	// Create gRPC client
	grpcClient, err := client.New(cfg.ServerHost, cfg.ServerPort, cfg.CACertFile, cfg.ServerName)
	if err != nil {
		logger.Fatal("failed to create gRPC client", "error", err)
	}
	defer grpcClient.Close()

	slog.Info("connected to backend", "host", cfg.ServerHost, "port", cfg.ServerPort)
	if cfg.CACertFile != "" {
		slog.Info("TLS enabled", "ca_cert", cfg.CACertFile)
	}

	// Initialize WAL
	var walInstance *wal.WAL
	if cfg.WALEnabled != nil && *cfg.WALEnabled {
		walInstance, err = wal.New(cfg.WALPath, cfg.WALMaxSizeMB)
		if err != nil {
			logger.Fatal("failed to initialize WAL", "error", err)
		}
		defer walInstance.Close()

		slog.Info("WAL enabled", "path", cfg.WALPath, "max_size_mb", cfg.WALMaxSizeMB)
	} else {
		slog.Warn("WAL disabled, metrics will be lost if send fails")
	}

	// Detect environment and create metrics config
	env := sysinfo.DetectEnvironment()
	metricsConfig := sysinfo.GetMetricsConfig(env, *cfg.ContainerMetrics)
	metricsConfig.ContainerRuntime = env.ContainerRuntime
	slog.Info("environment detected", "type", env.String())
	if *cfg.ContainerMetrics {
		slog.Info("container metrics enabled")
	}

	// Initialize metrics collector (important for macOS CPU metrics)
	slog.Debug("initializing metrics collector")
	metrics.Initialize()

	// Create sender with metrics config
	sender := wal.NewSender(walInstance, grpcClient, cfg.AgentID, cfg.AgentKey, AgentVersion, cfg.MetricsInterval, cfg.WALMaxSizeMB, metricsConfig)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handler
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Channels for command dispatch from heartbeat
	forceCollectCh := make(chan struct{}, 1)
	forceUpdateCh := make(chan struct{}, 1)

	// Start heartbeat in background
	go runHeartbeat(ctx, grpcClient, cfg, forceCollectCh, forceUpdateCh)

	// Start sender in background
	senderDone := make(chan struct{})
	go func() {
		defer close(senderDone)
		if err := sender.Run(ctx); err != nil {
			slog.Error("sender error", "error", err)
		}
	}()

	// Start package collector in background
	go runPackageCollector(ctx, grpcClient, cfg, forceCollectCh)

	// Start update checker in background
	go runUpdateChecker(ctx, forceUpdateCh)

	// Wait for signal
	sig := <-sigCh
	signal.Stop(sigCh)
	slog.Info("shutting down", "signal", sig.String())

	// Cancel context (triggers shutdown in sender and heartbeat)
	cancel()

	// Wait for sender to finish flushing (up to 6s: sender has internal 5s timeout)
	select {
	case <-senderDone:
		slog.Info("sender stopped cleanly")
	case <-time.After(6 * time.Second):
		slog.Warn("sender shutdown timed out")
	}

	slog.Info("shutdown complete")
}

// loadConfig loads and validates configuration
func loadConfig() (*config.Config, error) {
	if !config.Exists() {
		return nil, fmt.Errorf("config file not found, run 'watchflare-agent register' first")
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// Validate required fields
	if cfg.ServerHost == "" {
		return nil, fmt.Errorf("server_host is required")
	}
	if cfg.ServerPort == "" {
		return nil, fmt.Errorf("server_port is required")
	}
	if cfg.AgentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	if cfg.AgentKey == "" {
		return nil, fmt.Errorf("agent_key is required")
	}

	return cfg, nil
}

// runHeartbeat sends periodic heartbeats to the backend and dispatches any commands received.
func runHeartbeat(ctx context.Context, grpcClient *client.Client, cfg *config.Config, forceCollectCh, forceUpdateCh chan<- struct{}) {
	ticker := time.NewTicker(time.Duration(cfg.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	slog.Info("heartbeat started", "interval_sec", cfg.HeartbeatInterval)

	for {
		select {
		case <-ticker.C:
			ipv4, ipv6 := sysinfo.GetIPAddresses()
			cmds, err := grpcClient.SendHeartbeat(cfg.AgentID, cfg.AgentKey, ipv4, ipv6, AgentVersion)
			if err != nil {
				slog.Warn("heartbeat failed", "error", errors.FormatError(err, "Heartbeat"))
			} else {
				slog.Debug("heartbeat sent")
				for _, cmd := range cmds {
					slog.Info("command received", "type", cmd.Type, "id", cmd.CommandId)
					switch cmd.Type {
					case "collect_packages":
						select {
						case forceCollectCh <- struct{}{}:
						default: // already pending
						}
					case "update_agent":
						select {
						case forceUpdateCh <- struct{}{}:
						default: // already pending
						}
					default:
						slog.Warn("unknown command type", "type", cmd.Type)
					}
				}
			}

		case <-ctx.Done():
			slog.Info("heartbeat stopped")
			return
		}
	}
}

// runPackageCollector collects and sends package inventory.
// forceCollectCh triggers an immediate full collection when a signal is received.
func runPackageCollector(ctx context.Context, grpcClient *client.Client, cfg *config.Config, forceCollectCh <-chan struct{}) {
	statePath := filepath.Join(config.GetDataDir(), "packages.state.json")

	slog.Info("package collector started")

	// Wait 60 seconds before initial collection (let system stabilize)
	slog.Info("waiting before initial package collection", "delay_sec", 60)
	select {
	case <-time.After(60 * time.Second):
		collectAndSendPackages(ctx, grpcClient, cfg, statePath, false)
	case <-forceCollectCh:
		slog.Info("forced package collection triggered")
		collectAndSendPackages(ctx, grpcClient, cfg, statePath, true)
	case <-ctx.Done():
		slog.Info("package collector stopped before initial collection")
		return
	}

	// Schedule daily collection at 3 AM
	now := time.Now()
	next3AM := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
	if now.Hour() >= 3 {
		next3AM = next3AM.Add(24 * time.Hour)
	}

	timeUntil3AM := time.Until(next3AM)
	slog.Info("next package collection scheduled",
		"at", next3AM.Format("2006-01-02 15:04:05"),
		"in", timeUntil3AM.Round(time.Minute).String())

	timer := time.NewTimer(timeUntil3AM)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			collectAndSendPackages(ctx, grpcClient, cfg, statePath, false)
			timer.Reset(24 * time.Hour)

		case <-forceCollectCh:
			slog.Info("forced package collection triggered")
			collectAndSendPackages(ctx, grpcClient, cfg, statePath, true)

		case <-ctx.Done():
			slog.Info("package collector stopped")
			return
		}
	}
}

// runUpdateChecker periodically checks for available agent updates.
// forceUpdateCh triggers an immediate update attempt when a signal is received.
func runUpdateChecker(ctx context.Context, forceUpdateCh <-chan struct{}) {
	select {
	case <-time.After(5 * time.Minute):
	case <-forceUpdateCh:
		slog.Info("forced agent update triggered")
		performUpdate()
	case <-ctx.Done():
		return
	}

	checkAndLogUpdate()

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			checkAndLogUpdate()
		case <-forceUpdateCh:
			slog.Info("forced agent update triggered")
			performUpdate()
		case <-ctx.Done():
			return
		}
	}
}

func checkAndLogUpdate() {
	if AgentVersion == "dev" {
		return
	}
	info, err := update.CheckForUpdate(AgentVersion)
	if err != nil {
		slog.Warn("update check failed", "error", err)
		return
	}
	if info.UpdateAvailable {
		if runtime.GOOS == "darwin" {
			slog.Info("update available",
				"current", info.CurrentVersion,
				"latest", info.LatestVersion,
				"hint", "brew upgrade watchflare-agent && brew services restart watchflare-agent")
		} else {
			slog.Info("update available",
				"current", info.CurrentVersion,
				"latest", info.LatestVersion,
				"hint", "sudo watchflare-agent update")
		}
	}
}

// performUpdate checks for an update and applies it if available.
func performUpdate() {
	if AgentVersion == "dev" {
		slog.Info("skipping update: running dev build")
		return
	}
	if runtime.GOOS == "darwin" {
		slog.Info("skipping auto-update: use 'brew upgrade watchflare-agent' on macOS")
		return
	}
	info, err := update.CheckForUpdate(AgentVersion)
	if err != nil {
		slog.Warn("update check failed", "error", err)
		return
	}
	if !info.UpdateAvailable {
		slog.Info("agent is already up to date", "version", AgentVersion)
		return
	}
	slog.Info("applying update", "current", info.CurrentVersion, "latest", info.LatestVersion)
	if err := update.ApplyUpdate(info); err != nil {
		slog.Error("update failed", "error", err)
	}
}

// collectAndSendPackages performs package collection, delta calculation, and sending.
// When forceFull is true, always sends a full inventory regardless of cached state.
func collectAndSendPackages(ctx context.Context, grpcClient *client.Client, cfg *config.Config, statePath string, forceFull bool) {
	startTime := time.Now()
	slog.Info("starting package collection", "force_full", forceFull)

	allPackages, err := packages.CollectAll()
	if err != nil {
		slog.Error("package collection failed", "error", err)
		return
	}

	collectionDurationMs := time.Since(startTime).Milliseconds()
	slog.Info("packages collected", "count", len(allPackages), "duration_ms", collectionDurationMs)

	// Load previous state
	state, err := packages.LoadState(statePath)
	if err != nil {
		slog.Warn("failed to load package state", "error", err)
		state = &packages.PackageState{Packages: make([]*packages.Package, 0)}
	}

	// Compute delta
	added, removed, updated := state.ComputeDelta(allPackages)

	isFirstRun := len(state.Packages) == 0
	hasChanges := packages.HasChanges(added, removed, updated)

	// If any package has an available update, always send a full inventory so the
	// backend has current available_version / has_security_update for every package —
	// not just the ones that changed in this delta.
	if !forceFull && !isFirstRun {
		for _, pkg := range allPackages {
			if pkg.AvailableVersion != "" {
				slog.Info("updates available, sending full inventory to refresh update statuses")
				forceFull = true
				break
			}
		}
	}

	if !forceFull && !isFirstRun && !hasChanges {
		slog.Info("no package changes detected, skipping send")
		return
	}

	var inventoryType string
	if isFirstRun || forceFull {
		inventoryType = inventoryTypeFull
		if forceFull {
			slog.Info("forced full inventory", "count", len(allPackages))
		} else {
			slog.Info("first run: sending full inventory", "count", len(allPackages))
		}
	} else {
		inventoryType = inventoryTypeDelta
		slog.Info("package changes detected", "added", len(added), "removed", len(removed), "updated", len(updated))
	}

	var addedProto, removedProto, updatedProto, allProto []*pb.Package
	if inventoryType == inventoryTypeFull {
		allProto = convertPackagesToProto(allPackages)
	} else {
		addedProto = convertPackagesToProto(added)
		removedProto = convertPackagesToProto(removed)
		updatedProto = convertPackagesToProto(updated)
	}

	inventoryData := &client.PackageInventoryData{
		InventoryType:        inventoryType,
		AddedPackages:        addedProto,
		RemovedPackages:      removedProto,
		UpdatedPackages:      updatedProto,
		AllPackages:          allProto,
		CollectionDurationMs: collectionDurationMs,
		TotalPackageCount:    int32(len(allPackages)),
	}

	if err := grpcClient.SendPackageInventory(cfg.AgentID, cfg.AgentKey, inventoryData); err != nil {
		slog.Error("failed to send package inventory", "error", err)
		return
	}

	slog.Info("package inventory sent",
		"type", inventoryType,
		"added", len(added),
		"removed", len(removed),
		"updated", len(updated))

	state.Packages = allPackages
	state.LastScan = time.Now()
	state.PackageCount = len(allPackages)

	if err := state.Save(statePath); err != nil {
		slog.Warn("failed to save package state", "error", err)
	}
}

// convertPackagesToProto converts agent Package structs to protobuf Package structs
func convertPackagesToProto(pkgs []*packages.Package) []*pb.Package {
	protoPackages := make([]*pb.Package, len(pkgs))

	for i, pkg := range pkgs {
		var installedAt int64
		if !pkg.InstalledAt.IsZero() {
			installedAt = pkg.InstalledAt.Unix()
		}

		protoPackages[i] = &pb.Package{
			Name:              pkg.Name,
			Version:           pkg.Version,
			Architecture:      pkg.Architecture,
			PackageManager:    pkg.PackageManager,
			Source:            pkg.Source,
			InstalledAt:       installedAt,
			PackageSize:       pkg.PackageSize,
			Description:       pkg.Description,
			AvailableVersion:  pkg.AvailableVersion,
			HasSecurityUpdate: pkg.HasSecurityUpdate,
		}
	}

	return protoPackages
}
