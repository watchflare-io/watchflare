package grpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"watchflare/backend/cache"
	"watchflare/backend/database"
	"watchflare/backend/models"
	"watchflare/backend/pki"
	"watchflare/backend/sse"
	pb "watchflare/shared/proto/agent/v1"

	"gorm.io/gorm"
)

// AgentServer implements the AgentService gRPC server
type AgentServer struct {
	pb.UnimplementedAgentServiceServer
}

// Global PKI instance (set during startup, protected by mutex)
var (
	pkiInstance *pki.PKI
	pkiMu       sync.RWMutex
)

// SetPKI stores the PKI instance for use in gRPC handlers.
func SetPKI(p *pki.PKI) {
	pkiMu.Lock()
	defer pkiMu.Unlock()
	pkiInstance = p
}

// NewAgentServer creates a new AgentServer instance
func NewAgentServer() *AgentServer {
	return &AgentServer{}
}

// RegisterHost handles initial agent registration
func (s *AgentServer) RegisterHost(ctx context.Context, req *pb.RegisterHostRequest) (*pb.RegisterHostResponse, error) {
	// Step 1: Validate token and find the pending agent
	hashedToken := hashToken(req.RegistrationToken)
	var pendingAgent models.Host
	result := database.DB.Where("registration_token = ?", hashedToken).First(&pendingAgent)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return &pb.RegisterHostResponse{
				Success: false,
				Message: "Invalid registration token",
			}, nil
		}
		return nil, result.Error
	}

	// Step 2: Validate token hasn't expired
	if pendingAgent.ExpiresAt != nil && time.Now().After(*pendingAgent.ExpiresAt) {
		return &pb.RegisterHostResponse{
			Success: false,
			Message: "Registration token has expired",
		}, nil
	}

	// Step 3: Validate token is for a pending agent
	if pendingAgent.Status != models.StatusPending && pendingAgent.Status != models.StatusExpired {
		return &pb.RegisterHostResponse{
			Success: false,
			Message: "Host is already registered",
		}, nil
	}

	// Step 4: Validate IP if required
	if !pendingAgent.AllowAnyIPRegistration {
		if pendingAgent.ConfiguredIP != nil && *pendingAgent.ConfiguredIP != "" {
			if req.IpAddressV4 != *pendingAgent.ConfiguredIP {
				return &pb.RegisterHostResponse{
					Success: false,
					Message: "IP address mismatch. Expected: " + *pendingAgent.ConfiguredIP + ", Got: " + req.IpAddressV4,
				}, nil
			}
		}
	}

	// Step 5: Check if this is a re-registration (existing UUID provided).
	// The UUID must match the pending agent's own AgentID to prevent a token holder
	// from hijacking an arbitrary existing agent.
	var agentToUse *models.Host
	var deletePending bool

	if req.ExistingAgentUuid != "" {
		if req.ExistingAgentUuid != pendingAgent.AgentID {
			return &pb.RegisterHostResponse{
				Success: false,
				Message: "Invalid registration request",
			}, nil
		}

		// Try to find existing agent by UUID
		var existingAgent models.Host
		result := database.DB.Where("agent_id = ?", req.ExistingAgentUuid).First(&existingAgent)
		if result.Error == nil {
			// Found existing agent - reactivate it instead of using pending
			slog.Warn("re-registration: reactivating existing agent", "agent_id", existingAgent.AgentID, "hostname", req.Hostname)
			agentToUse = &existingAgent
			deletePending = true // We'll delete the unused pending agent
		} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Database error (not "not found")
			return nil, result.Error
		}
		// If UUID not found, fall through to use pending agent
	}

	// Step 6: If no existing agent found, use the pending agent
	if agentToUse == nil {
		slog.Info("new registration", "agent_id", pendingAgent.AgentID, "hostname", req.Hostname)
		agentToUse = &pendingAgent
		deletePending = false
	}

	// Step 7: Update the agent with registration information
	now := time.Now()
	cpuPhysical := int(req.CpuPhysicalCount)
	cpuLogical := int(req.CpuLogicalCount)
	updates := map[string]interface{}{
		"hostname":              req.Hostname,
		"ip_address_v4":         req.IpAddressV4,
		"ip_address_v6":         req.IpAddressV6,
		"os":                    req.Os,
		"platform":              req.Platform,
		"platform_version":      req.PlatformVersion,
		"platform_family":       req.PlatformFamily,
		"kernel_version":        req.KernelVersion,
		"kernel_arch":           req.KernelArch,
		"environment_type":      req.EnvironmentType,
		"virtualization_system": req.VirtualizationSystem,
		"virtualization_role":   req.VirtualizationRole,
		"container_runtime":     req.ContainerRuntime,
		"host_id":               req.HostId,
		"cpu_model_name":        req.CpuModelName,
		"cpu_physical_count":    &cpuPhysical,
		"cpu_logical_count":     &cpuLogical,
		"cpu_mhz":               req.CpuMhz,
		"agent_version":         req.AgentVersion,
		"status":                models.StatusOffline,
		"last_seen":             &now,
		"registration_token":    nil, // Always clear token after successful registration
		"expires_at":            nil, // Clear expiration
	}

	// If this is a reactivation, set reactivated_at timestamp
	if deletePending {
		updates["reactivated_at"] = &now
	}

	if err := database.DB.Model(agentToUse).Updates(updates).Error; err != nil {
		return nil, err
	}

	// Step 8: Delete the pending agent if we reactivated an existing one.
	// Skip deletion if both point to the same record (token was regenerated on the existing agent).
	if deletePending && pendingAgent.ID != agentToUse.ID {
		if err := database.DB.Delete(&pendingAgent).Error; err != nil {
			slog.Warn("failed to delete pending agent", "agent_id", pendingAgent.AgentID, "error", err)
			// Not fatal - continue with registration
		} else {
			slog.Info("deleted unused pending agent", "agent_id", pendingAgent.AgentID)
		}
	}

	// Step 9: Broadcast SSE event for host update
	broker := sse.GetBroker()
	configuredIP := ""
	if agentToUse.ConfiguredIP != nil {
		configuredIP = *agentToUse.ConfiguredIP
	}
	broker.BroadcastHostUpdate(sse.HostUpdate{
		ID:               agentToUse.ID,
		Status:           models.StatusOffline,
		IPv4Address:      req.IpAddressV4,
		IPv6Address:      req.IpAddressV6,
		ConfiguredIP:     configuredIP,
		IgnoreIPMismatch: agentToUse.IgnoreIPMismatch,
		LastSeen:         now.Format(time.RFC3339),
		Reactivated:      deletePending, // True if existing agent was reactivated
		Hostname:         req.Hostname,  // For notification message
	})

	// Step 10: Get CA certificate for agent TLS verification
	pkiMu.RLock()
	pki := pkiInstance
	pkiMu.RUnlock()
	caCertPEM, err := pki.GetCACertPEM()
	if err != nil {
		return nil, fmt.Errorf("failed to get CA certificate: %w", err)
	}

	return &pb.RegisterHostResponse{
		Success:     true,
		Message:     "Host registered successfully",
		AgentId:     agentToUse.AgentID,
		AgentKey:    agentToUse.AgentKey,
		CaCert:      string(caCertPEM),
		ServerName:  "watchflare",
		Reactivated: deletePending, // True if we reactivated existing agent
	}, nil
}

// Heartbeat handles periodic heartbeats from agents
func (s *AgentServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	// Verify agent credentials (read-only DB query)
	var host models.Host
	result := database.DB.Where("agent_id = ? AND agent_key = ?", req.AgentId, req.AgentKey).First(&host)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return &pb.HeartbeatResponse{
				Success: false,
				Message: "Invalid agent credentials",
			}, nil
		}
		return nil, result.Error
	}

	// If host is paused, acknowledge but don't update cache or broadcast
	if host.Status == models.StatusPaused {
		return &pb.HeartbeatResponse{
			Success: true,
			Message: "Host is paused",
		}, nil
	}

	// If host is pending (newly registered or just resumed), promote it to online
	// in the DB. The alert worker filters pending out at the SQL level, so without
	// this the worker would skip the host until the next sync (up to 5 min) and
	// any open incident (e.g. host_down) would stay open even after the agent
	// came back.
	if host.Status == models.StatusPending {
		if err := database.DB.Model(&host).Update("status", models.StatusOnline).Error; err != nil {
			slog.Warn("failed to promote pending host to online on heartbeat", "host_id", host.ID, "error", err)
		}
	}

	// Update agent version in DB if it changed (e.g. immediately after a self-update + restart)
	if req.AgentVersion != "" {
		currentVersion := ""
		if host.AgentVersion != nil {
			currentVersion = *host.AgentVersion
		}
		if req.AgentVersion != currentVersion {
			if err := database.DB.Model(&host).Update("agent_version", req.AgentVersion).Error; err != nil {
				slog.Warn("failed to update agent version", "host_id", host.ID, "error", err)
			} else {
				host.AgentVersion = &req.AgentVersion
			}
		}
	}

	// Update heartbeat cache (in-memory, no DB write)
	// Fall back to DB values if heartbeat sends empty IPs (agent uses Heartbeat() not SendHeartbeat())
	ipv4 := req.IpAddressV4
	if ipv4 == "" && host.IPAddressV4 != nil {
		ipv4 = *host.IPAddressV4
	}
	ipv6 := req.IpAddressV6
	if ipv6 == "" && host.IPAddressV6 != nil {
		ipv6 = *host.IPAddressV6
	}
	heartbeatCache := cache.GetCache()
	heartbeatCache.Update(req.AgentId, ipv4, ipv6)

	// Consume any pending commands for this agent
	pending := heartbeatCache.ConsumeCommands(req.AgentId)
	var protoCommands []*pb.PendingCommand
	for _, cmd := range pending {
		protoCommands = append(protoCommands, &pb.PendingCommand{
			CommandId: cmd.CommandID,
			Type:      cmd.Type,
		})
	}

	// Broadcast SSE event for real-time dashboard
	broker := sse.GetBroker()
	configuredIP := ""
	if host.ConfiguredIP != nil {
		configuredIP = *host.ConfiguredIP
	}
	agentVersion := ""
	if host.AgentVersion != nil {
		agentVersion = *host.AgentVersion
	}
	broker.BroadcastHostUpdate(sse.HostUpdate{
		ID:               host.ID,
		Status:           models.StatusOnline,
		IPv4Address:      ipv4,
		IPv6Address:      ipv6,
		ConfiguredIP:     configuredIP,
		IgnoreIPMismatch: host.IgnoreIPMismatch,
		LastSeen:         time.Now().Format(time.RFC3339),
		AgentVersion:     agentVersion,
	})

	return &pb.HeartbeatResponse{
		Success:  true,
		Message:  "Heartbeat acknowledged",
		Commands: protoCommands,
	}, nil
}

// SendMetrics handles incoming system metrics from agents
func (s *AgentServer) SendMetrics(ctx context.Context, req *pb.SendMetricsRequest) (*pb.SendMetricsResponse, error) {
	// Find host by agent ID and verify agent key
	var host models.Host
	result := database.DB.Where("agent_id = ? AND agent_key = ?", req.AgentId, req.AgentKey).First(&host)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return &pb.SendMetricsResponse{
				Success: false,
				Message: "Invalid agent credentials",
			}, nil
		}
		return nil, result.Error
	}

	// Update agent version if it changed (e.g. after an upgrade + restart)
	if req.AgentVersion != "" {
		currentVersion := ""
		if host.AgentVersion != nil {
			currentVersion = *host.AgentVersion
		}
		if req.AgentVersion != currentVersion {
			if err := database.DB.Model(&host).Update("agent_version", req.AgentVersion).Error; err != nil {
				slog.Warn("failed to update agent version", "host_id", host.ID, "error", err)
			}
		}
	}

	// Update slowly-changing host info if any field changed
	if req.HostInfo != nil {
		hostInfoUpdates := map[string]interface{}{}

		derefStr := func(p *string) string {
			if p == nil {
				return ""
			}
			return *p
		}
		derefInt := func(p *int) int {
			if p == nil {
				return 0
			}
			return *p
		}
		derefFloat := func(p *float64) float64 {
			if p == nil {
				return 0
			}
			return *p
		}

		if req.HostInfo.PlatformVersion != "" && req.HostInfo.PlatformVersion != derefStr(host.PlatformVersion) {
			hostInfoUpdates["platform_version"] = req.HostInfo.PlatformVersion
		}
		if req.HostInfo.KernelVersion != "" && req.HostInfo.KernelVersion != derefStr(host.KernelVersion) {
			hostInfoUpdates["kernel_version"] = req.HostInfo.KernelVersion
		}
		if req.HostInfo.KernelArch != "" && req.HostInfo.KernelArch != derefStr(host.KernelArch) {
			hostInfoUpdates["kernel_arch"] = req.HostInfo.KernelArch
		}
		if req.HostInfo.CpuModelName != "" && req.HostInfo.CpuModelName != derefStr(host.CPUModelName) {
			hostInfoUpdates["cpu_model_name"] = req.HostInfo.CpuModelName
		}
		if req.HostInfo.CpuPhysicalCount != 0 && int(req.HostInfo.CpuPhysicalCount) != derefInt(host.CPUPhysicalCount) {
			v := int(req.HostInfo.CpuPhysicalCount)
			hostInfoUpdates["cpu_physical_count"] = &v
		}
		if req.HostInfo.CpuLogicalCount != 0 && int(req.HostInfo.CpuLogicalCount) != derefInt(host.CPULogicalCount) {
			v := int(req.HostInfo.CpuLogicalCount)
			hostInfoUpdates["cpu_logical_count"] = &v
		}
		if req.HostInfo.CpuMhz != 0 && req.HostInfo.CpuMhz != derefFloat(host.CPUMhz) {
			hostInfoUpdates["cpu_mhz"] = req.HostInfo.CpuMhz
		}
		// ContainerRuntime can be empty string (host not in container) — always compare
		if req.HostInfo.ContainerRuntime != derefStr(host.ContainerRuntime) {
			hostInfoUpdates["container_runtime"] = req.HostInfo.ContainerRuntime
		}

		if len(hostInfoUpdates) > 0 {
			if err := database.DB.Model(&host).Updates(hostInfoUpdates).Error; err != nil {
				slog.Warn("failed to update host info", "host_id", host.ID, "error", err)
			}
		}
	}

	if req.Metrics == nil {
		return &pb.SendMetricsResponse{
			Success: false,
			Message: "metrics payload is required",
		}, nil
	}

	// If host is paused, acknowledge but don't store metrics
	if host.Status == models.StatusPaused {
		slog.Info("metrics discarded for paused host", "name", host.DisplayName, "host_id", host.ID)
		return &pb.SendMetricsResponse{
			Success: true,
			Message: "Host is paused, metrics discarded",
		}, nil
	}

	// Convert proto sensor readings to model type and SSE minified format in one pass
	var sensorReadings models.SensorReadings
	var sseSensorReadings []sse.SensorReadingMinified
	for _, sr := range req.Metrics.SensorReadings {
		sensorReadings = append(sensorReadings, models.SensorReading{
			Key:                sr.Key,
			TemperatureCelsius: sr.TemperatureCelsius,
		})
		sseSensorReadings = append(sseSensorReadings, sse.SensorReadingMinified{
			K: sr.Key,
			V: sr.TemperatureCelsius,
		})
	}

	// Create metric record
	metric := &models.Metric{
		HostID:                host.ID,
		Timestamp:             time.Unix(req.Metrics.Timestamp, 0),
		CPUUsagePercent:       req.Metrics.CpuUsagePercent,
		CPUIowaitPercent:      req.Metrics.CpuIowaitPercent,
		CPUStealPercent:       req.Metrics.CpuStealPercent,
		MemoryTotalBytes:      req.Metrics.MemoryTotalBytes,
		MemoryUsedBytes:       req.Metrics.MemoryUsedBytes,
		MemoryAvailableBytes:  req.Metrics.MemoryAvailableBytes,
		MemoryBuffersBytes:    req.Metrics.MemoryBuffersBytes,
		MemoryCachedBytes:     req.Metrics.MemoryCachedBytes,
		SwapTotalBytes:        req.Metrics.SwapTotalBytes,
		SwapUsedBytes:         req.Metrics.SwapUsedBytes,
		LoadAvg1Min:           req.Metrics.LoadAvg_1Min,
		LoadAvg5Min:           req.Metrics.LoadAvg_5Min,
		LoadAvg15Min:          req.Metrics.LoadAvg_15Min,
		DiskTotalBytes:        req.Metrics.DiskTotalBytes,
		DiskUsedBytes:         req.Metrics.DiskUsedBytes,
		DiskReadBytesPerSec:   req.Metrics.DiskReadBytesPerSec,
		DiskWriteBytesPerSec:  req.Metrics.DiskWriteBytesPerSec,
		NetworkRxBytesPerSec:  req.Metrics.NetworkRxBytesPerSec,
		NetworkTxBytesPerSec:  req.Metrics.NetworkTxBytesPerSec,
		CPUTemperatureCelsius: req.Metrics.CpuTemperatureCelsius,
		SensorReadings:        sensorReadings,
		UptimeSeconds:         req.Metrics.UptimeSeconds,
		ProcessesCount:        req.Metrics.ProcessesCount,
	}

	// Broadcast SSE first for low-latency real-time display (Netdata/Prometheus pattern)
	broker := sse.GetBroker()
	broker.BroadcastMetricsUpdate(sse.MetricsUpdate{
		HostID:                host.ID,
		Timestamp:             metric.Timestamp.Format(time.RFC3339),
		CPUUsagePercent:       metric.CPUUsagePercent,
		CPUIowaitPercent:      metric.CPUIowaitPercent,
		CPUStealPercent:       metric.CPUStealPercent,
		MemoryTotalBytes:      metric.MemoryTotalBytes,
		MemoryUsedBytes:       metric.MemoryUsedBytes,
		MemoryAvailableBytes:  metric.MemoryAvailableBytes,
		MemoryBuffersBytes:    metric.MemoryBuffersBytes,
		MemoryCachedBytes:     metric.MemoryCachedBytes,
		SwapTotalBytes:        metric.SwapTotalBytes,
		SwapUsedBytes:         metric.SwapUsedBytes,
		LoadAvg1Min:           metric.LoadAvg1Min,
		LoadAvg5Min:           metric.LoadAvg5Min,
		LoadAvg15Min:          metric.LoadAvg15Min,
		DiskTotalBytes:        metric.DiskTotalBytes,
		DiskUsedBytes:         metric.DiskUsedBytes,
		DiskReadBytesPerSec:   metric.DiskReadBytesPerSec,
		DiskWriteBytesPerSec:  metric.DiskWriteBytesPerSec,
		NetworkRxBytesPerSec:  metric.NetworkRxBytesPerSec,
		NetworkTxBytesPerSec:  metric.NetworkTxBytesPerSec,
		CPUTemperatureCelsius: metric.CPUTemperatureCelsius,
		SensorReadings:        sseSensorReadings,
		UptimeSeconds:         metric.UptimeSeconds,
		ProcessesCount:        metric.ProcessesCount,
	})

	if len(req.ContainerMetrics) > 0 {
		var containerModels []models.ContainerMetric
		var sseContainerMetrics []sse.ContainerMetricMinified

		for _, cm := range req.ContainerMetrics {
			containerModels = append(containerModels, models.ContainerMetric{
				HostID:               host.ID,
				Timestamp:            metric.Timestamp,
				ContainerID:          cm.ContainerId,
				ContainerName:        cm.ContainerName,
				Image:                cm.Image,
				CPUPercent:           cm.CpuPercent,
				MemoryUsedBytes:      cm.MemoryUsedBytes,
				MemoryLimitBytes:     cm.MemoryLimitBytes,
				NetworkRxBytesPerSec: cm.NetworkRxBytesPerSec,
				NetworkTxBytesPerSec: cm.NetworkTxBytesPerSec,
				Runtime:              cm.ContainerRuntime,
				Status:               cm.Status,
				Health:               cm.Health,
				Ports:                cm.Ports,
			})

			sseContainerMetrics = append(sseContainerMetrics, sse.ContainerMetricMinified{
				ID:      cm.ContainerId,
				Name:    cm.ContainerName,
				CPU:     cm.CpuPercent,
				MU:      cm.MemoryUsedBytes,
				ML:      cm.MemoryLimitBytes,
				NR:      cm.NetworkRxBytesPerSec,
				NT:      cm.NetworkTxBytesPerSec,
				Runtime: cm.ContainerRuntime,
				Status:  cm.Status,
				Health:  cm.Health,
				Ports:   cm.Ports,
			})
		}

		broker.BroadcastContainerMetricsUpdate(sse.ContainerMetricsUpdate{
			HostID:    host.ID,
			Timestamp: metric.Timestamp.Unix(),
			Metrics:   sseContainerMetrics,
		})

		// Persist container metrics to DB (after SSE for lower latency)
		if err := database.DB.Create(&containerModels).Error; err != nil {
			slog.Warn("failed to save container metrics", "host_id", host.ID, "error", err)
		}

		containerStates := make([]models.ContainerState, 0, len(req.ContainerMetrics))
		for _, cm := range req.ContainerMetrics {
			containerStates = append(containerStates, models.ContainerState{
				HostID:               host.ID,
				ContainerID:          cm.ContainerId,
				ContainerName:        cm.ContainerName,
				Image:                cm.Image,
				CPUPercent:           cm.CpuPercent,
				MemoryUsedBytes:      cm.MemoryUsedBytes,
				MemoryLimitBytes:     cm.MemoryLimitBytes,
				NetworkRxBytesPerSec: cm.NetworkRxBytesPerSec,
				NetworkTxBytesPerSec: cm.NetworkTxBytesPerSec,
				Runtime:              cm.ContainerRuntime,
				Status:               cm.Status,
				Health:               cm.Health,
				Ports:                cm.Ports,
				UpdatedAt:            time.Now(),
			})
		}

		if err := database.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("host_id = ?", host.ID).Delete(&models.ContainerState{}).Error; err != nil {
				return err
			}
			return tx.Create(&containerStates).Error
		}); err != nil {
			slog.Warn("failed to replace container states", "host_id", host.ID, "error", err)
		}
	}

	// Persist system metric to DB (after SSE for lower latency)
	if err := database.DB.Create(metric).Error; err != nil {
		return nil, fmt.Errorf("failed to save metrics: %w", err)
	}

	// Persist normalized sensor readings for multi-range aggregation
	if len(sensorReadings) > 0 {
		sensorMetrics := make([]models.SensorMetric, len(sensorReadings))
		for i, sr := range sensorReadings {
			sensorMetrics[i] = models.SensorMetric{
				Time:        metric.Timestamp,
				HostID:      host.ID,
				SensorKey:   sr.Key,
				Temperature: sr.TemperatureCelsius,
			}
		}
		if err := database.DB.Create(&sensorMetrics).Error; err != nil {
			slog.Warn("failed to save sensor metrics", "host_id", host.ID, "error", err)
		}
	}

	return &pb.SendMetricsResponse{
		Success: true,
		Message: "Metrics received successfully",
	}, nil
}

// ReportDroppedMetrics handles reports of metrics that were dropped by agents
func (s *AgentServer) ReportDroppedMetrics(ctx context.Context, req *pb.ReportDroppedMetricsRequest) (*pb.ReportDroppedMetricsResponse, error) {
	// Verify agent credentials
	var host models.Host
	result := database.DB.Where("agent_id = ? AND agent_key = ?", req.AgentId, req.AgentKey).First(&host)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return &pb.ReportDroppedMetricsResponse{
				Success: false,
				Message: "Invalid agent credentials",
			}, nil
		}
		return nil, result.Error
	}

	// Insert dropped metrics report into database
	err := database.DB.Exec(`
		INSERT INTO dropped_metrics
		(host_id, count, first_dropped_at, last_dropped_at, reason)
		VALUES ($1, $2, $3, $4, $5)
	`,
		host.ID,
		req.Count,
		time.Unix(req.FirstDroppedAt, 0),
		time.Unix(req.LastDroppedAt, 0),
		req.Reason,
	).Error

	if err != nil {
		slog.Error("failed to insert dropped metrics report", "host_id", host.ID, "error", err)
		return nil, fmt.Errorf("failed to save dropped metrics report: %w", err)
	}

	// Calculate downtime duration for logging
	downtimeDuration := time.Unix(req.LastDroppedAt, 0).Sub(time.Unix(req.FirstDroppedAt, 0))

	slog.Warn("agent reported dropped metrics",
		"name", host.DisplayName,
		"agent_id", req.AgentId,
		"count", req.Count,
		"downtime", downtimeDuration.Round(time.Second),
		"reason", req.Reason,
	)

	return &pb.ReportDroppedMetricsResponse{
		Success: true,
		Message: "Dropped metrics report received",
	}, nil
}

// SendPackageInventory handles package inventory updates from agents
func (s *AgentServer) SendPackageInventory(ctx context.Context, req *pb.SendPackageInventoryRequest) (*pb.SendPackageInventoryResponse, error) {
	// Verify agent credentials
	var host models.Host
	result := database.DB.Where("agent_id = ? AND agent_key = ?", req.AgentId, req.AgentKey).First(&host)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return &pb.SendPackageInventoryResponse{
				Success: false,
				Message: "Invalid agent credentials",
			}, nil
		}
		return nil, result.Error
	}

	// If host is paused, acknowledge but don't process the inventory
	if host.Status == models.StatusPaused {
		slog.Info("package inventory discarded for paused host", "name", host.DisplayName, "host_id", host.ID)
		return &pb.SendPackageInventoryResponse{
			Success: true,
			Message: "Host is paused, inventory discarded",
		}, nil
	}

	// Process package inventory
	packagesProcessed, changesDetected, err := processPackageInventory(host.ID, req)
	if err != nil {
		slog.Error("failed to process package inventory", "host_id", host.ID, "error", err)
		return &pb.SendPackageInventoryResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to process package inventory: %v", err),
		}, nil
	}

	slog.Info("package inventory processed",
		"name", host.DisplayName,
		"host_id", host.ID,
		"packages", packagesProcessed,
		"changes", changesDetected,
		"type", req.InventoryType,
		"duration_ms", req.CollectionDurationMs,
	)

	// Bulk-mark packages from package managers that have update checkers as update_checked = true.
	// This is done using a static list of checkable package managers so that packages
	// not included in a delta (unchanged, up-to-date packages) are also marked correctly.
	if err := database.DB.Model(&models.Package{}).
		Where("host_id = ? AND package_manager IN ?", host.ID, models.CheckablePackageManagers).
		Update("update_checked", true).Error; err != nil {
		slog.Warn("failed to bulk-mark packages as update_checked", "host_id", host.ID, "error", err)
	}

	sse.GetBroker().BroadcastPackageInventoryUpdate(sse.PackageInventoryUpdate{
		HostID:         host.ID,
		CollectionType: req.InventoryType,
		PackagesCount:  packagesProcessed,
		ChangesCount:   changesDetected,
	})

	return &pb.SendPackageInventoryResponse{
		Success:           true,
		Message:           "Package inventory received successfully",
		PackagesProcessed: int32(packagesProcessed),
		ChangesDetected:   int32(changesDetected),
	}, nil
}

// pkgFields returns the mutable fields map used for package upsert/update.
func pkgFields(pkg *pb.Package, installedAt *time.Time, now time.Time) map[string]interface{} {
	return map[string]interface{}{
		"version":             pkg.Version,
		"architecture":        pkg.Architecture,
		"source":              pkg.Source,
		"installed_at":        installedAt,
		"package_size":        pkg.PackageSize,
		"description":         pkg.Description,
		"available_version":   pkg.AvailableVersion,
		"has_security_update": pkg.HasSecurityUpdate,
		"last_seen":           now,
	}
}

// writeHistory inserts a package history record within a transaction.
func writeHistory(tx *gorm.DB, hostID string, pkg *pb.Package, changeType string, now time.Time) error {
	return tx.Create(&models.PackageHistory{
		Timestamp:      now,
		HostID:         hostID,
		Name:           pkg.Name,
		Version:        pkg.Version,
		Architecture:   pkg.Architecture,
		PackageManager: pkg.PackageManager,
		Source:         pkg.Source,
		PackageSize:    pkg.PackageSize,
		Description:    pkg.Description,
		ChangeType:     changeType,
	}).Error
}

// processPackageInventory handles the business logic for package inventory updates
func processPackageInventory(hostID string, req *pb.SendPackageInventoryRequest) (int, int, error) {
	tx := database.DB.Begin()
	if tx.Error != nil {
		return 0, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	now := time.Now()
	packagesProcessed := 0
	changesDetected := 0

	if req.InventoryType != models.CollectionTypeFull && req.InventoryType != models.CollectionTypeDelta {
		tx.Rollback()
		return 0, 0, fmt.Errorf("unknown inventory_type: %q", req.InventoryType)
	}

	if req.InventoryType == models.CollectionTypeFull {
		for _, pkg := range req.AllPackages {
			installedAt := convertTimestamp(pkg.InstalledAt)
			model := models.Package{
				HostID: hostID, Name: pkg.Name, Version: pkg.Version,
				Architecture: pkg.Architecture, PackageManager: pkg.PackageManager,
				Source: pkg.Source, InstalledAt: installedAt, PackageSize: pkg.PackageSize,
				Description: pkg.Description, AvailableVersion: pkg.AvailableVersion,
				HasSecurityUpdate: pkg.HasSecurityUpdate, FirstSeen: now, LastSeen: now,
			}
			result := tx.Where("host_id = ? AND name = ? AND package_manager = ?", hostID, pkg.Name, pkg.PackageManager).Assign(pkgFields(pkg, installedAt, now)).FirstOrCreate(&model)
			if result.Error != nil {
				tx.Rollback()
				return 0, 0, fmt.Errorf("failed to upsert package %s: %w", pkg.Name, result.Error)
			}
			// RowsAffected == 1 means GORM just inserted the row (new package).
			// RowsAffected == 0 means the row already existed (found, then updated via Assign).
			if result.RowsAffected == 1 {
				if err := writeHistory(tx, hostID, pkg, models.ChangeTypeInitial, now); err != nil {
					tx.Rollback()
					return 0, 0, fmt.Errorf("failed to create history record for %s: %w", pkg.Name, err)
				}
			}
			packagesProcessed++
		}
		changesDetected = packagesProcessed

	} else if req.InventoryType == models.CollectionTypeDelta {
		for _, pkg := range req.AddedPackages {
			installedAt := convertTimestamp(pkg.InstalledAt)
			model := models.Package{
				HostID: hostID, Name: pkg.Name, Version: pkg.Version,
				Architecture: pkg.Architecture, PackageManager: pkg.PackageManager,
				Source: pkg.Source, InstalledAt: installedAt, PackageSize: pkg.PackageSize,
				Description: pkg.Description, AvailableVersion: pkg.AvailableVersion,
				HasSecurityUpdate: pkg.HasSecurityUpdate, FirstSeen: now, LastSeen: now,
			}
			if err := tx.Where("host_id = ? AND name = ? AND package_manager = ?", hostID, pkg.Name, pkg.PackageManager).Assign(pkgFields(pkg, installedAt, now)).FirstOrCreate(&model).Error; err != nil {
				tx.Rollback()
				return 0, 0, fmt.Errorf("failed to upsert added package %s: %w", pkg.Name, err)
			}
			if err := writeHistory(tx, hostID, pkg, models.ChangeTypeAdded, now); err != nil {
				tx.Rollback()
				return 0, 0, fmt.Errorf("failed to create history for added package %s: %w", pkg.Name, err)
			}
			changesDetected++
		}

		for _, pkg := range req.RemovedPackages {
			if err := tx.Where("host_id = ? AND name = ? AND package_manager = ?", hostID, pkg.Name, pkg.PackageManager).Delete(&models.Package{}).Error; err != nil {
				tx.Rollback()
				return 0, 0, fmt.Errorf("failed to delete removed package %s: %w", pkg.Name, err)
			}
			if err := writeHistory(tx, hostID, pkg, models.ChangeTypeRemoved, now); err != nil {
				tx.Rollback()
				return 0, 0, fmt.Errorf("failed to create history for removed package %s: %w", pkg.Name, err)
			}
			changesDetected++
		}

		for _, pkg := range req.UpdatedPackages {
			installedAt := convertTimestamp(pkg.InstalledAt)
			if err := tx.Model(&models.Package{}).Where("host_id = ? AND name = ? AND package_manager = ?", hostID, pkg.Name, pkg.PackageManager).Updates(pkgFields(pkg, installedAt, now)).Error; err != nil {
				tx.Rollback()
				return 0, 0, fmt.Errorf("failed to update package %s: %w", pkg.Name, err)
			}
			if err := writeHistory(tx, hostID, pkg, models.ChangeTypeUpdated, now); err != nil {
				tx.Rollback()
				return 0, 0, fmt.Errorf("failed to create history for updated package %s: %w", pkg.Name, err)
			}
			changesDetected++
		}

		packagesProcessed = int(req.TotalPackageCount)
	}

	if err := tx.Create(&models.PackageCollection{
		HostID:         hostID,
		Timestamp:      now,
		CollectionType: req.InventoryType,
		PackageCount:   int(req.TotalPackageCount),
		ChangesCount:   changesDetected,
		DurationMs:     int(req.CollectionDurationMs),
		Status:         models.PackageCollectionStatusSuccess,
	}).Error; err != nil {
		tx.Rollback()
		return 0, 0, fmt.Errorf("failed to create collection record: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return packagesProcessed, changesDetected, nil
}

// SendServiceInventory handles systemd service inventory updates from agents (replace-all)
func (s *AgentServer) SendServiceInventory(ctx context.Context, req *pb.SendServiceInventoryRequest) (*pb.SendServiceInventoryResponse, error) {
	var host models.Host
	result := database.DB.Where("agent_id = ? AND agent_key = ?", req.AgentId, req.AgentKey).First(&host)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return &pb.SendServiceInventoryResponse{Success: false, Message: "Invalid agent credentials"}, nil
		}
		return nil, result.Error
	}
	if host.Status == models.StatusPaused {
		slog.Info("service inventory discarded for paused host", "name", host.DisplayName, "host_id", host.ID)
		return &pb.SendServiceInventoryResponse{Success: true, Message: "Host is paused, inventory discarded"}, nil
	}

	count, err := processServiceInventory(host.ID, req)
	if err != nil {
		slog.Error("failed to process service inventory", "host_id", host.ID, "error", err)
		return &pb.SendServiceInventoryResponse{Success: false, Message: fmt.Sprintf("Failed to process service inventory: %v", err)}, nil
	}
	slog.Info("service inventory processed", "host_id", host.ID, "services", count)
	return &pb.SendServiceInventoryResponse{Success: true, Message: "OK"}, nil
}

func processServiceInventory(hostID string, req *pb.SendServiceInventoryRequest) (int, error) {
	now := time.Now()
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("host_id = ?", hostID).Delete(&models.Service{}).Error; err != nil {
			return err
		}
		if len(req.Services) == 0 {
			return nil
		}
		rows := make([]models.Service, 0, len(req.Services))
		for _, sv := range req.Services {
			rows = append(rows, models.Service{
				HostID:       hostID,
				Name:         sv.Name,
				Description:  sv.Description,
				EnabledState: sv.EnabledState,
				ActiveState:  sv.ActiveState,
				SubState:     sv.SubState,
				CollectedAt:  now,
			})
		}
		return tx.Create(&rows).Error
	})
	if err != nil {
		return 0, err
	}
	return len(req.Services), nil
}

// ReportServiceHealth handles live service state updates from agents.
func (s *AgentServer) ReportServiceHealth(ctx context.Context, req *pb.ReportServiceHealthRequest) (*pb.ReportServiceHealthResponse, error) {
	var host models.Host
	result := database.DB.Where("agent_id = ? AND agent_key = ?", req.AgentId, req.AgentKey).First(&host)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return &pb.ReportServiceHealthResponse{Success: false, Message: "Invalid agent credentials"}, nil
		}
		return nil, result.Error
	}
	if host.Status == models.StatusPaused {
		return &pb.ReportServiceHealthResponse{Success: true, Message: "Host is paused"}, nil
	}
	if len(req.Services) == 0 {
		return &pb.ReportServiceHealthResponse{Success: true, Message: "OK"}, nil
	}

	collectedAt := time.Unix(req.CollectedAt, 0)
	names := make([]string, 0, len(req.Services))
	payload := make([]sse.ServiceHealthPayload, 0, len(req.Services))
	for _, h := range req.Services {
		names = append(names, h.Name)
		res := database.DB.Model(&models.Service{}).
			Where("host_id = ? AND name = ?", host.ID, h.Name).
			Updates(map[string]interface{}{
				"active_state": h.ActiveState,
				"sub_state":    h.SubState,
				"collected_at": collectedAt,
			})
		if res.Error != nil {
			return nil, fmt.Errorf("update service %s: %w", h.Name, res.Error)
		}
		if res.RowsAffected == 0 {
			continue
		}
		payload = append(payload, sse.ServiceHealthPayload{Name: h.Name, ActiveState: h.ActiveState, SubState: h.SubState})
	}

	prune := database.DB.Where("host_id = ?", host.ID)
	if len(names) > 0 {
		prune = prune.Where("name NOT IN ?", names)
	}
	if err := prune.Delete(&models.Service{}).Error; err != nil {
		return nil, fmt.Errorf("prune services: %w", err)
	}

	sse.GetBroker().BroadcastServiceHealthUpdate(sse.ServiceHealthUpdate{HostID: host.ID, Services: payload})

	return &pb.ReportServiceHealthResponse{Success: true, Message: "OK"}, nil
}

// convertTimestamp converts Unix timestamp to *time.Time (nil if 0)
func convertTimestamp(ts int64) *time.Time {
	if ts == 0 {
		return nil
	}
	t := time.Unix(ts, 0)
	return &t
}

// hashToken creates a SHA-256 hash of a token
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
