package grpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"watchflare/backend/database"
	"watchflare/backend/models"
	"watchflare/backend/pki"
	"watchflare/backend/sse"
	pb "watchflare/shared/proto/agent/v1"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDSN() string {
	get := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}
	return "host=" + get("POSTGRES_HOST", "localhost") +
		" port=" + get("POSTGRES_PORT", "5432") +
		" user=" + get("POSTGRES_USER", "watchflare") +
		" password=" + get("POSTGRES_PASSWORD", "watchflare_dev") +
		" dbname=" + get("POSTGRES_TEST_DB", "watchflare_test") +
		" sslmode=" + get("POSTGRES_SSLMODE", "disable")
}

// setupGRPCTestDB connects to the local PostgreSQL database for testing.
func setupGRPCTestDB(t *testing.T) {
	t.Helper()
	if err := database.Connect(testDSN()); err != nil {
		t.Skipf("skipping grpc tests: database unavailable: %v", err)
	}
}

// setupTestPKI generates a real auto-mode PKI in a temporary directory and
// initializes it so GetCACertificate returns a valid CA cert during tests.
func setupTestPKI(t *testing.T) {
	t.Helper()
	p, err := pki.New(&pki.Config{
		Mode:   pki.ModeAuto,
		PKIDir: t.TempDir(),
	})
	require.NoError(t, err)
	require.NoError(t, p.Initialize())
	SetPKI(p)
}

// hashTestToken computes SHA-256 of a registration token (matches hashToken in agent_service.go).
func hashTestToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// createPendingHost inserts a pending host with a registration token into the DB.
func createPendingHost(t *testing.T, token string) *models.Host {
	t.Helper()
	expiry := time.Now().Add(24 * time.Hour)
	host := &models.Host{
		ID:                     uuid.New().String(),
		AgentID:                uuid.New().String(),
		DisplayName:            "test-host-" + token[:8],
		Status:                 models.StatusPending,
		RegistrationToken:      strPtr(hashTestToken(token)),
		ExpiresAt:              &expiry,
		AllowAnyIPRegistration: true,
		AgentKey:               "test-agent-key-" + token[:8],
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() {
		database.DB.Unscoped().Delete(host)
	})
	return host
}

func strPtr(s string) *string { return &s }

// --- Tests ---

func TestRegisterHost_InvalidToken(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	req := &pb.RegisterHostRequest{
		RegistrationToken: "wf_reg_doesnotexist",
		Hostname:          "test-host",
	}
	resp, err := s.RegisterHost(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "Invalid registration token", resp.Message)
}

func TestRegisterHost_AlreadyRegistered(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	const token = "wf_reg_alreadyregistered01"

	expiry := time.Now().Add(24 * time.Hour)
	host := &models.Host{
		ID:                     uuid.New().String(),
		AgentID:                uuid.New().String(),
		DisplayName:            "already-registered",
		Status:                 models.StatusOnline,
		RegistrationToken:      strPtr(hashTestToken(token)),
		ExpiresAt:              &expiry,
		AllowAnyIPRegistration: true,
		AgentKey:               "some-key",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	req := &pb.RegisterHostRequest{
		RegistrationToken: token,
		Hostname:          "test-host",
	}
	resp, err := s.RegisterHost(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "Host is already registered", resp.Message)
}

func TestRegisterHost_ExpiredToken(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	const token = "wf_reg_expiredtoken00001"
	expiry := time.Now().Add(-1 * time.Hour) // already expired
	host := &models.Host{
		ID:                     uuid.New().String(),
		AgentID:                uuid.New().String(),
		DisplayName:            "expired-host",
		Status:                 models.StatusPending,
		RegistrationToken:      strPtr(hashTestToken(token)),
		ExpiresAt:              &expiry,
		AllowAnyIPRegistration: true,
		AgentKey:               "some-key",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	req := &pb.RegisterHostRequest{
		RegistrationToken: token,
		Hostname:          "test-host",
	}
	resp, err := s.RegisterHost(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "Registration token has expired", resp.Message)
}

func TestRegisterHost_Success(t *testing.T) {
	setupGRPCTestDB(t)
	setupTestPKI(t)
	s := NewAgentServer()

	const token = "wf_reg_successtoken0001"
	host := createPendingHost(t, token)

	cpuPhys := int32(8)
	cpuLog := int32(16)
	req := &pb.RegisterHostRequest{
		RegistrationToken:    token,
		Hostname:             "my-host",
		IpAddressV4:          "1.2.3.4",
		Os:                   "linux",
		Platform:             "fedora",
		PlatformVersion:      "43",
		PlatformFamily:       "rhel",
		KernelVersion:        "6.17.1-300.fc43.aarch64",
		KernelArch:           "aarch64",
		VirtualizationSystem: "kvm",
		VirtualizationRole:   "guest",
		HostId:               "test-host-id-abc",
		CpuModelName:         "Intel Core i9",
		CpuPhysicalCount:     cpuPhys,
		CpuLogicalCount:      cpuLog,
		CpuMhz:               3600.0,
		AgentVersion:         "0.28.0",
	}
	resp, err := s.RegisterHost(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.AgentId)
	assert.Equal(t, host.AgentKey, resp.AgentKey)

	// Verify all new gopsutil fields were persisted correctly
	var updated models.Host
	require.NoError(t, database.DB.First(&updated, "id = ?", host.ID).Error)
	assert.Nil(t, updated.RegistrationToken)
	assert.Equal(t, "offline", updated.Status)
	require.NotNil(t, updated.OS)
	assert.Equal(t, "linux", *updated.OS)
	require.NotNil(t, updated.Platform)
	assert.Equal(t, "fedora", *updated.Platform)
	require.NotNil(t, updated.PlatformVersion)
	assert.Equal(t, "43", *updated.PlatformVersion)
	require.NotNil(t, updated.PlatformFamily)
	assert.Equal(t, "rhel", *updated.PlatformFamily)
	require.NotNil(t, updated.KernelVersion)
	assert.Equal(t, "6.17.1-300.fc43.aarch64", *updated.KernelVersion)
	require.NotNil(t, updated.KernelArch)
	assert.Equal(t, "aarch64", *updated.KernelArch)
	require.NotNil(t, updated.VirtualizationSystem)
	assert.Equal(t, "kvm", *updated.VirtualizationSystem)
	require.NotNil(t, updated.VirtualizationRole)
	assert.Equal(t, "guest", *updated.VirtualizationRole)
	require.NotNil(t, updated.HostID)
	assert.Equal(t, "test-host-id-abc", *updated.HostID)
	require.NotNil(t, updated.CPUModelName)
	assert.Equal(t, "Intel Core i9", *updated.CPUModelName)
	require.NotNil(t, updated.CPUPhysicalCount)
	assert.Equal(t, 8, *updated.CPUPhysicalCount)
	require.NotNil(t, updated.CPULogicalCount)
	assert.Equal(t, 16, *updated.CPULogicalCount)
	require.NotNil(t, updated.CPUMhz)
	assert.Equal(t, 3600.0, *updated.CPUMhz)
}

func TestSendMetrics_InvalidCredentials(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	req := &pb.SendMetricsRequest{
		AgentId:  "00000000-0000-0000-0000-000000000000",
		AgentKey: "invalid-key",
		Metrics:  &pb.Metrics{Timestamp: time.Now().Unix()},
	}
	resp, err := s.SendMetrics(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "Invalid agent credentials", resp.Message)
}

func TestSendMetrics_PausedHost(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "paused-host",
		Status:      models.StatusPaused,
		AgentKey:    "paused-agent-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	req := &pb.SendMetricsRequest{
		AgentId:  host.AgentID,
		AgentKey: host.AgentKey,
		Metrics:  &pb.Metrics{Timestamp: time.Now().Unix()},
	}
	resp, err := s.SendMetrics(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Contains(t, resp.Message, "paused")
}

func TestSendMetrics_NilMetrics(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "online-host",
		Status:      models.StatusOnline,
		AgentKey:    "online-agent-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	req := &pb.SendMetricsRequest{
		AgentId:  host.AgentID,
		AgentKey: host.AgentKey,
		Metrics:  nil,
	}
	resp, err := s.SendMetrics(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "required")
}

func TestSendMetrics_Success(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "metrics-host",
		Status:      models.StatusOnline,
		AgentKey:    "metrics-agent-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	req := &pb.SendMetricsRequest{
		AgentId:  host.AgentID,
		AgentKey: host.AgentKey,
		Metrics: &pb.Metrics{
			Timestamp:            time.Now().Unix(),
			CpuUsagePercent:      42.5,
			CpuIowaitPercent:     3.2,
			CpuStealPercent:      1.1,
			MemoryTotalBytes:     8000000000,
			MemoryUsedBytes:      4000000000,
			MemoryAvailableBytes: 4000000000,
			MemoryBuffersBytes:   512000000,
			MemoryCachedBytes:    1024000000,
			SwapTotalBytes:       2000000000,
			SwapUsedBytes:        500000000,
			ProcessesCount:       142,
		},
	}
	resp, err := s.SendMetrics(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Verify all new fields were persisted
	var stored models.Metric
	require.NoError(t, database.DB.Where("host_id = ?", host.ID).Last(&stored).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(&stored) })
	assert.Equal(t, 42.5, stored.CPUUsagePercent)
	assert.Equal(t, 3.2, stored.CPUIowaitPercent)
	assert.Equal(t, 1.1, stored.CPUStealPercent)
	assert.Equal(t, uint64(512000000), stored.MemoryBuffersBytes)
	assert.Equal(t, uint64(1024000000), stored.MemoryCachedBytes)
	assert.Equal(t, uint64(2000000000), stored.SwapTotalBytes)
	assert.Equal(t, uint64(500000000), stored.SwapUsedBytes)
	assert.Equal(t, uint64(142), stored.ProcessesCount)
}

func TestSendMetrics_HostInfoUpdate(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "hostinfo-update-host",
		Status:      models.StatusOnline,
		AgentKey:    "hostinfo-agent-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	req := &pb.SendMetricsRequest{
		AgentId:  host.AgentID,
		AgentKey: host.AgentKey,
		Metrics:  &pb.Metrics{Timestamp: time.Now().Unix()},
		HostInfo: &pb.HostInfo{
			PlatformVersion:  "22.04",
			KernelVersion:    "6.1.0-amd64",
			KernelArch:       "x86_64",
			CpuModelName:     "Intel Xeon E5",
			CpuPhysicalCount: 4,
			CpuLogicalCount:  8,
			CpuMhz:           2400.0,
			ContainerRuntime: "docker",
		},
	}
	resp, err := s.SendMetrics(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Verify HostInfo fields were persisted
	var updated models.Host
	require.NoError(t, database.DB.First(&updated, "id = ?", host.ID).Error)
	require.NotNil(t, updated.PlatformVersion)
	assert.Equal(t, "22.04", *updated.PlatformVersion)
	require.NotNil(t, updated.KernelVersion)
	assert.Equal(t, "6.1.0-amd64", *updated.KernelVersion)
	require.NotNil(t, updated.KernelArch)
	assert.Equal(t, "x86_64", *updated.KernelArch)
	require.NotNil(t, updated.CPUModelName)
	assert.Equal(t, "Intel Xeon E5", *updated.CPUModelName)
	require.NotNil(t, updated.CPUPhysicalCount)
	assert.Equal(t, 4, *updated.CPUPhysicalCount)
	require.NotNil(t, updated.CPULogicalCount)
	assert.Equal(t, 8, *updated.CPULogicalCount)
	require.NotNil(t, updated.CPUMhz)
	assert.Equal(t, 2400.0, *updated.CPUMhz)
	require.NotNil(t, updated.ContainerRuntime)
	assert.Equal(t, "docker", *updated.ContainerRuntime)
}

func TestHeartbeat_InvalidCredentials(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	req := &pb.HeartbeatRequest{
		AgentId:  "00000000-0000-0000-0000-000000000000",
		AgentKey: "invalid-key",
	}
	resp, err := s.Heartbeat(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "Invalid agent credentials", resp.Message)
}

func TestHeartbeat_PausedHost(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "paused-hb-host",
		Status:      models.StatusPaused,
		AgentKey:    "paused-hb-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	req := &pb.HeartbeatRequest{
		AgentId:  host.AgentID,
		AgentKey: host.AgentKey,
	}
	resp, err := s.Heartbeat(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Contains(t, resp.Message, "paused")
}

func TestHeartbeat_Online(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "online-hb-host",
		Status:      models.StatusOnline,
		AgentKey:    "online-hb-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	req := &pb.HeartbeatRequest{
		AgentId:     host.AgentID,
		AgentKey:    host.AgentKey,
		IpAddressV4: "10.0.0.1",
	}
	resp, err := s.Heartbeat(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestHeartbeat_UpdatesAgentVersion(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	oldVersion := "0.31.0"
	host := &models.Host{
		ID:           uuid.New().String(),
		AgentID:      uuid.New().String(),
		DisplayName:  "version-update-host",
		Status:       models.StatusOnline,
		AgentKey:     "version-update-key-abc123",
		AgentVersion: &oldVersion,
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	req := &pb.HeartbeatRequest{
		AgentId:      host.AgentID,
		AgentKey:     host.AgentKey,
		AgentVersion: "0.32.1",
	}
	resp, err := s.Heartbeat(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	var updated models.Host
	require.NoError(t, database.DB.First(&updated, "id = ?", host.ID).Error)
	require.NotNil(t, updated.AgentVersion)
	assert.Equal(t, "0.32.1", *updated.AgentVersion)
}

func TestHeartbeat_SkipsVersionUpdateWhenUnchanged(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	version := "0.32.1"
	host := &models.Host{
		ID:           uuid.New().String(),
		AgentID:      uuid.New().String(),
		DisplayName:  "version-unchanged-host",
		Status:       models.StatusOnline,
		AgentKey:     "version-unchanged-key-abc123",
		AgentVersion: &version,
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	req := &pb.HeartbeatRequest{
		AgentId:      host.AgentID,
		AgentKey:     host.AgentKey,
		AgentVersion: "0.32.1",
	}
	resp, err := s.Heartbeat(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	var updated models.Host
	require.NoError(t, database.DB.First(&updated, "id = ?", host.ID).Error)
	require.NotNil(t, updated.AgentVersion)
	assert.Equal(t, "0.32.1", *updated.AgentVersion)
}

func TestReportDroppedMetrics_InvalidCredentials(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	req := &pb.ReportDroppedMetricsRequest{
		AgentId:  "00000000-0000-0000-0000-000000000000",
		AgentKey: "invalid-key",
	}
	resp, err := s.ReportDroppedMetrics(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Success)
}

func TestReportDroppedMetrics_Success(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "drop-host",
		Status:      models.StatusOnline,
		AgentKey:    "drop-agent-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	now := time.Now().Unix()
	req := &pb.ReportDroppedMetricsRequest{
		AgentId:        host.AgentID,
		AgentKey:       host.AgentKey,
		Count:          5,
		FirstDroppedAt: now - 60,
		LastDroppedAt:  now,
		Reason:         "max_retries_exceeded",
	}
	resp, err := s.ReportDroppedMetrics(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestSendPackageInventory_InvalidCredentials(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	req := &pb.SendPackageInventoryRequest{
		AgentId:  "00000000-0000-0000-0000-000000000000",
		AgentKey: "invalid-key",
	}
	resp, err := s.SendPackageInventory(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, resp.Success)
}

func TestSendPackageInventory_PausedHost(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "paused-pkg-host",
		Status:      models.StatusPaused,
		AgentKey:    "paused-pkg-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	req := &pb.SendPackageInventoryRequest{
		AgentId:       host.AgentID,
		AgentKey:      host.AgentKey,
		InventoryType: models.CollectionTypeFull,
		AllPackages: []*pb.Package{
			{Name: "curl", Version: "7.88.0", PackageManager: "apt"},
		},
		TotalPackageCount: 1,
	}
	resp, err := s.SendPackageInventory(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Contains(t, resp.Message, "paused")
}

func TestProcessPackageInventory_UnknownType(t *testing.T) {
	setupGRPCTestDB(t)

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "pkg-inv-host",
		Status:      models.StatusOnline,
		AgentKey:    "pkg-inv-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	req := &pb.SendPackageInventoryRequest{
		AgentId:       host.AgentID,
		AgentKey:      host.AgentKey,
		InventoryType: "unknown_type",
	}
	_, _, err := processPackageInventory(host.ID, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown inventory_type")
}

func TestProcessPackageInventory_FullInventory(t *testing.T) {
	setupGRPCTestDB(t)

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "pkg-full-host",
		Status:      models.StatusOnline,
		AgentKey:    "pkg-full-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	req := &pb.SendPackageInventoryRequest{
		InventoryType: models.CollectionTypeFull,
		AllPackages: []*pb.Package{
			{Name: "curl", Version: "7.88.0", PackageManager: "apt"},
			{Name: "git", Version: "2.39.0", PackageManager: "apt"},
		},
		TotalPackageCount: 2,
	}
	processed, changes, err := processPackageInventory(host.ID, req)
	require.NoError(t, err)
	assert.Equal(t, 2, processed)
	assert.Equal(t, 2, changes)
}

func TestProcessPackageInventory_DeltaInventory(t *testing.T) {
	setupGRPCTestDB(t)

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "pkg-delta-host",
		Status:      models.StatusOnline,
		AgentKey:    "pkg-delta-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	// Seed an existing package to remove/update
	fullReq := &pb.SendPackageInventoryRequest{
		InventoryType: models.CollectionTypeFull,
		AllPackages: []*pb.Package{
			{Name: "curl", Version: "7.88.0", PackageManager: "apt"},
			{Name: "vim", Version: "8.2", PackageManager: "apt"},
		},
		TotalPackageCount: 2,
	}
	_, _, err := processPackageInventory(host.ID, fullReq)
	require.NoError(t, err)

	deltaReq := &pb.SendPackageInventoryRequest{
		InventoryType:     models.CollectionTypeDelta,
		AddedPackages:     []*pb.Package{{Name: "htop", Version: "3.2.0", PackageManager: "apt"}},
		RemovedPackages:   []*pb.Package{{Name: "vim", Version: "8.2", PackageManager: "apt"}},
		UpdatedPackages:   []*pb.Package{{Name: "curl", Version: "8.0.0", PackageManager: "apt"}},
		TotalPackageCount: 2,
	}
	processed, changes, err := processPackageInventory(host.ID, deltaReq)
	require.NoError(t, err)
	assert.Equal(t, 2, processed) // TotalPackageCount
	assert.Equal(t, 3, changes)   // 1 added + 1 removed + 1 updated
}

func cleanupServices(t *testing.T) {
	t.Helper()
	database.DB.Exec("DELETE FROM services")
}

func TestReportServiceHealth_UpdatesStateAndIgnoresUnknown(t *testing.T) {
	setupGRPCTestDB(t)
	defer cleanupServices(t)

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "service-health-host",
		Status:      models.StatusOnline,
		AgentKey:    "service-health-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	database.DB.Create(&models.Service{HostID: host.ID, Name: "x.service", ActiveState: "active", SubState: "running", CollectedAt: time.Unix(0, 0)})

	sseClient := sse.GetBroker().AddClientWithHostFilter("test-report-service-health", host.ID)
	defer sse.GetBroker().RemoveClient("test-report-service-health")

	s := NewAgentServer()
	_, err := s.ReportServiceHealth(context.Background(), &pb.ReportServiceHealthRequest{
		AgentId:     host.AgentID,
		AgentKey:    host.AgentKey,
		CollectedAt: time.Now().Unix(),
		Services: []*pb.ServiceHealth{
			{Name: "x.service", ActiveState: "failed", SubState: "failed"},
			{Name: "ghost.service", ActiveState: "failed", SubState: "failed"},
		},
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}

	select {
	case ev := <-sseClient.Channel:
		if ev.Type != sse.EventTypeServiceHealthUpdate {
			t.Fatalf("unexpected SSE event type: %s", ev.Type)
		}
		update, ok := ev.Data.(sse.ServiceHealthUpdate)
		if !ok {
			t.Fatalf("unexpected SSE event data type: %T", ev.Data)
		}
		broadcastNames := make(map[string]bool, len(update.Services))
		for _, p := range update.Services {
			broadcastNames[p.Name] = true
		}
		if !broadcastNames["x.service"] {
			t.Fatalf("expected x.service in SSE payload, got %+v", update.Services)
		}
		if broadcastNames["ghost.service"] {
			t.Fatalf("ghost.service (unknown) must not be in SSE payload, got %+v", update.Services)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no service_health_update broadcast received")
	}

	var rows []models.Service
	database.DB.Where("host_id = ?", host.ID).Find(&rows)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row (ghost ignored), got %d", len(rows))
	}
	if rows[0].ActiveState != "failed" || rows[0].SubState != "failed" {
		t.Fatalf("state not updated: %+v", rows[0])
	}
	if !rows[0].CollectedAt.After(time.Unix(1, 0)) {
		t.Fatalf("collected_at not refreshed: %v", rows[0].CollectedAt)
	}
}

func TestReportServiceHealth_PrunesDroppedServices(t *testing.T) {
	setupGRPCTestDB(t)
	defer cleanupServices(t)

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "service-prune-host",
		Status:      models.StatusOnline,
		AgentKey:    "service-prune-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	database.DB.Create(&models.Service{HostID: host.ID, Name: "a.service", ActiveState: "active", SubState: "running"})
	database.DB.Create(&models.Service{HostID: host.ID, Name: "b.service", ActiveState: "active", SubState: "running"})

	s := NewAgentServer()
	_, err := s.ReportServiceHealth(context.Background(), &pb.ReportServiceHealthRequest{
		AgentId:     host.AgentID,
		AgentKey:    host.AgentKey,
		CollectedAt: time.Now().Unix(),
		Services: []*pb.ServiceHealth{
			{Name: "a.service", ActiveState: "active", SubState: "running"},
		},
	})
	if err != nil {
		t.Fatalf("report: %v", err)
	}

	var rows []models.Service
	database.DB.Where("host_id = ?", host.ID).Find(&rows)
	if len(rows) != 1 || rows[0].Name != "a.service" {
		t.Fatalf("expected only a.service to remain (b.service pruned), got %+v", rows)
	}
}

func TestSendServiceInventory_ReplaceAll(t *testing.T) {
	setupGRPCTestDB(t)
	defer cleanupServices(t)

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "service-inv-host",
		Status:      models.StatusOnline,
		AgentKey:    "service-inv-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() { database.DB.Unscoped().Delete(host) })

	s := NewAgentServer()
	ctx := context.Background()

	_, err := s.SendServiceInventory(ctx, &pb.SendServiceInventoryRequest{
		AgentId:  host.AgentID,
		AgentKey: host.AgentKey,
		Services: []*pb.Service{
			{Name: "a.service", ActiveState: "active", SubState: "running", EnabledState: "enabled"},
			{Name: "b.service", ActiveState: "failed", SubState: "failed", EnabledState: "enabled"},
		},
	})
	if err != nil {
		t.Fatalf("first inventory: %v", err)
	}

	// Second inventory with a different set must fully replace the first.
	_, err = s.SendServiceInventory(ctx, &pb.SendServiceInventoryRequest{
		AgentId:  host.AgentID,
		AgentKey: host.AgentKey,
		Services: []*pb.Service{
			{Name: "c.service", ActiveState: "active", SubState: "running", EnabledState: "enabled"},
		},
	})
	if err != nil {
		t.Fatalf("second inventory: %v", err)
	}

	var rows []models.Service
	database.DB.Where("host_id = ?", host.ID).Order("name").Find(&rows)
	if len(rows) != 1 || rows[0].Name != "c.service" {
		t.Fatalf("expected only c.service, got %+v", rows)
	}
}
