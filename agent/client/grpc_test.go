package client

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"watchflare-agent/metrics"
	pb "watchflare/shared/proto/agent/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// mockAgentServer is a configurable in-process gRPC server for testing
type mockAgentServer struct {
	pb.UnimplementedAgentServiceServer

	registerFn         func(*pb.RegisterHostRequest) (*pb.RegisterHostResponse, error)
	heartbeatFn        func(*pb.HeartbeatRequest) (*pb.HeartbeatResponse, error)
	sendMetricsFn      func(*pb.SendMetricsRequest) (*pb.SendMetricsResponse, error)
	sendPackageInvFn   func(*pb.SendPackageInventoryRequest) (*pb.SendPackageInventoryResponse, error)
}

func (m *mockAgentServer) RegisterHost(_ context.Context, req *pb.RegisterHostRequest) (*pb.RegisterHostResponse, error) {
	if m.registerFn != nil {
		return m.registerFn(req)
	}
	return nil, status.Error(codes.Unimplemented, "not configured")
}

func (m *mockAgentServer) Heartbeat(_ context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if m.heartbeatFn != nil {
		return m.heartbeatFn(req)
	}
	return nil, status.Error(codes.Unimplemented, "not configured")
}

func (m *mockAgentServer) SendMetrics(_ context.Context, req *pb.SendMetricsRequest) (*pb.SendMetricsResponse, error) {
	if m.sendMetricsFn != nil {
		return m.sendMetricsFn(req)
	}
	return nil, status.Error(codes.Unimplemented, "not configured")
}

func (m *mockAgentServer) SendPackageInventory(_ context.Context, req *pb.SendPackageInventoryRequest) (*pb.SendPackageInventoryResponse, error) {
	if m.sendPackageInvFn != nil {
		return m.sendPackageInvFn(req)
	}
	return nil, status.Error(codes.Unimplemented, "not configured")
}

// startMockServer starts a local gRPC server and returns a Client connected to it.
// The server is stopped automatically when the test ends.
func startMockServer(t *testing.T, srv *mockAgentServer) *Client {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}

	grpcSrv := grpc.NewServer()
	pb.RegisterAgentServiceServer(grpcSrv, srv)

	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(grpcSrv.Stop)

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create test gRPC client: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return &Client{conn: conn, client: pb.NewAgentServiceClient(conn)}
}

// --- SaveCACertificate ---

func TestSaveCACertificate_CreatesFileAndDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pki", "sub")
	path := filepath.Join(dir, "ca.pem")
	pem := "-----BEGIN CERTIFICATE-----\nfakecert\n-----END CERTIFICATE-----\n"

	if err := SaveCACertificate(pem, path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(data) != pem {
		t.Fatalf("content mismatch: got %q", string(data))
	}
}

func TestSaveCACertificate_FilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ca.pem")
	pem := "-----BEGIN CERTIFICATE-----\nfakecert\n-----END CERTIFICATE-----\n"

	if err := SaveCACertificate(pem, path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if got := info.Mode().Perm(); got != 0640 {
		t.Errorf("file permissions: got %04o, want 0640", got)
	}
}

func TestSaveCACertificate_InvalidPath(t *testing.T) {
	// Use a file as a directory component — cannot be created
	tmp := t.TempDir()
	blockingFile := filepath.Join(tmp, "notadir")
	if err := os.WriteFile(blockingFile, []byte("x"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := SaveCACertificate("pem", filepath.Join(blockingFile, "ca.pem"))
	if err == nil {
		t.Fatal("expected error for invalid path, got nil")
	}
}

// --- New() ---

func TestNew_MissingCACertFile(t *testing.T) {
	_, err := New("localhost", "50051", "/nonexistent/ca.pem", "server")
	if err == nil {
		t.Fatal("expected error for missing CA cert file, got nil")
	}
}

func TestNew_InvalidPEM(t *testing.T) {
	tmp := t.TempDir()
	badPEM := filepath.Join(tmp, "bad.pem")
	if err := os.WriteFile(badPEM, []byte("this is not valid PEM"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := New("localhost", "50051", badPEM, "server")
	if err == nil {
		t.Fatal("expected error for invalid PEM, got nil")
	}
}

// --- NewForRegistration() ---

func TestNewForRegistration_Succeeds(t *testing.T) {
	// gRPC connection is lazy — no actual network dial happens here
	c, err := NewForRegistration("localhost", "50051")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c.Close()
}

// --- Register() ---

func TestRegister_Success(t *testing.T) {
	mock := &mockAgentServer{
		registerFn: func(req *pb.RegisterHostRequest) (*pb.RegisterHostResponse, error) {
			if req.RegistrationToken != "tok" {
				return nil, status.Error(codes.InvalidArgument, "bad token")
			}
			return &pb.RegisterHostResponse{
				Success:    true,
				AgentId:    "agent-1",
				AgentKey:   "key-1",
				CaCert:     "pem-data",
				ServerName: "backend.local",
				Reactivated: false,
			}, nil
		},
	}

	c := startMockServer(t, mock)

	resp, err := c.Register(RegisterRequest{
		Token:    "tok",
		Hostname: "host1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.AgentID != "agent-1" {
		t.Errorf("AgentID: got %q, want %q", resp.AgentID, "agent-1")
	}
	if resp.AgentKey != "key-1" {
		t.Errorf("AgentKey: got %q, want %q", resp.AgentKey, "key-1")
	}
	if resp.CACert != "pem-data" {
		t.Errorf("CACert: got %q, want %q", resp.CACert, "pem-data")
	}
	if resp.ServerName != "backend.local" {
		t.Errorf("ServerName: got %q, want %q", resp.ServerName, "backend.local")
	}
}

func TestRegister_Rejected(t *testing.T) {
	mock := &mockAgentServer{
		registerFn: func(_ *pb.RegisterHostRequest) (*pb.RegisterHostResponse, error) {
			return &pb.RegisterHostResponse{
				Success: false,
				Message: "token expired",
			}, nil
		},
	}

	c := startMockServer(t, mock)

	_, err := c.Register(RegisterRequest{Token: "bad"})
	if err == nil {
		t.Fatal("expected error for rejected registration, got nil")
	}
}

func TestRegister_ServerError(t *testing.T) {
	mock := &mockAgentServer{
		registerFn: func(_ *pb.RegisterHostRequest) (*pb.RegisterHostResponse, error) {
			return nil, status.Error(codes.Internal, "internal error")
		},
	}

	c := startMockServer(t, mock)

	_, err := c.Register(RegisterRequest{Token: "tok"})
	if err == nil {
		t.Fatal("expected error for server failure, got nil")
	}
}

func TestRegister_FieldsMapping(t *testing.T) {
	var received *pb.RegisterHostRequest

	mock := &mockAgentServer{
		registerFn: func(req *pb.RegisterHostRequest) (*pb.RegisterHostResponse, error) {
			received = req
			return &pb.RegisterHostResponse{Success: true, AgentId: "x", AgentKey: "y"}, nil
		},
	}

	c := startMockServer(t, mock)

	_, err := c.Register(RegisterRequest{
		Token:                "token123",
		Hostname:             "myhost",
		IPv4:                 "1.2.3.4",
		IPv6:                 "::1",
		OS:                   "linux",
		Platform:             "fedora",
		PlatformVersion:      "43",
		PlatformFamily:       "rhel",
		KernelVersion:        "6.1.0",
		KernelArch:           "aarch64",
		EnvironmentType:      "vm",
		VirtualizationSystem: "kvm",
		VirtualizationRole:   "guest",
		HostID:               "test-host-uuid-123",
		ContainerRuntime:     "docker",
		CPUModelName:         "Apple M1",
		CPUPhysicalCount:     8,
		CPULogicalCount:      8,
		CPUMhz:               3200.0,
		ExistingUUID:         "old-uuid",
		AgentVersion:         "1.2.3",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"RegistrationToken", received.RegistrationToken, "token123"},
		{"Hostname", received.Hostname, "myhost"},
		{"IpAddressV4", received.IpAddressV4, "1.2.3.4"},
		{"IpAddressV6", received.IpAddressV6, "::1"},
		{"Os", received.Os, "linux"},
		{"Platform", received.Platform, "fedora"},
		{"PlatformVersion", received.PlatformVersion, "43"},
		{"PlatformFamily", received.PlatformFamily, "rhel"},
		{"KernelVersion", received.KernelVersion, "6.1.0"},
		{"KernelArch", received.KernelArch, "aarch64"},
		{"EnvironmentType", received.EnvironmentType, "vm"},
		{"VirtualizationSystem", received.VirtualizationSystem, "kvm"},
		{"VirtualizationRole", received.VirtualizationRole, "guest"},
		{"HostId", received.HostId, "test-host-uuid-123"},
		{"ContainerRuntime", received.ContainerRuntime, "docker"},
		{"CpuModelName", received.CpuModelName, "Apple M1"},
		{"ExistingAgentUuid", received.ExistingAgentUuid, "old-uuid"},
		{"AgentVersion", received.AgentVersion, "1.2.3"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}

	if received.CpuPhysicalCount != 8 {
		t.Errorf("CpuPhysicalCount: got %d, want 8", received.CpuPhysicalCount)
	}
	if received.CpuLogicalCount != 8 {
		t.Errorf("CpuLogicalCount: got %d, want 8", received.CpuLogicalCount)
	}
	if received.CpuMhz != 3200.0 {
		t.Errorf("CpuMhz: got %f, want 3200.0", received.CpuMhz)
	}
}

// --- SendHeartbeat ---

func TestSendHeartbeat_SendsAgentVersion(t *testing.T) {
	var receivedVersion string

	mock := &mockAgentServer{
		heartbeatFn: func(req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
			receivedVersion = req.AgentVersion
			return &pb.HeartbeatResponse{Success: true}, nil
		},
	}

	c := startMockServer(t, mock)

	if _, err := c.SendHeartbeat("agent-1", "key-1", "", "", "0.32.1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedVersion != "0.32.1" {
		t.Errorf("AgentVersion: got %q, want %q", receivedVersion, "0.32.1")
	}
}

func TestSendHeartbeat_Success(t *testing.T) {
	mock := &mockAgentServer{
		heartbeatFn: func(req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
			if req.AgentId != "agent-1" || req.AgentKey != "key-1" {
				return nil, status.Error(codes.InvalidArgument, "bad credentials")
			}
			return &pb.HeartbeatResponse{Success: true}, nil
		},
	}

	c := startMockServer(t, mock)

	if _, err := c.SendHeartbeat("agent-1", "key-1", "1.2.3.4", "::1", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendHeartbeat_Rejected(t *testing.T) {
	mock := &mockAgentServer{
		heartbeatFn: func(_ *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
			return &pb.HeartbeatResponse{Success: false, Message: "agent not found"}, nil
		},
	}

	c := startMockServer(t, mock)

	_, err := c.SendHeartbeat("agent-1", "key-1", "", "", "")
	if err == nil {
		t.Fatal("expected error for rejected heartbeat, got nil")
	}
}

func TestSendHeartbeat_ReturnsPendingCommands(t *testing.T) {
	mock := &mockAgentServer{
		heartbeatFn: func(_ *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
			return &pb.HeartbeatResponse{
				Success: true,
				Commands: []*pb.PendingCommand{
					{CommandId: "cmd-1", Type: "collect_packages"},
					{CommandId: "cmd-2", Type: "update_agent"},
				},
			}, nil
		},
	}

	c := startMockServer(t, mock)

	cmds, err := c.SendHeartbeat("agent-1", "key-1", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
	if cmds[0].Type != "collect_packages" {
		t.Errorf("commands[0].Type: got %q, want %q", cmds[0].Type, "collect_packages")
	}
	if cmds[1].Type != "update_agent" {
		t.Errorf("commands[1].Type: got %q, want %q", cmds[1].Type, "update_agent")
	}
}

// --- SendMetrics ---

func TestSendMetrics_Success(t *testing.T) {
	var received *pb.SendMetricsRequest

	mock := &mockAgentServer{
		sendMetricsFn: func(req *pb.SendMetricsRequest) (*pb.SendMetricsResponse, error) {
			received = req
			return &pb.SendMetricsResponse{Success: true}, nil
		},
	}

	c := startMockServer(t, mock)

	m := &metrics.SystemMetrics{
		CPUUsagePercent:    45.5,
		CPUIowaitPercent:   3.2,
		CPUStealPercent:    1.1,
		MemoryTotalBytes:   8 * 1024 * 1024 * 1024,
		MemoryUsedBytes:    4 * 1024 * 1024 * 1024,
		MemoryBuffersBytes: 512 * 1024 * 1024,
		MemoryCachedBytes:  1024 * 1024 * 1024,
		SwapTotalBytes:     2 * 1024 * 1024 * 1024,
		SwapUsedBytes:      512 * 1024 * 1024,
		ProcessesCount:     200,
		SensorReadings: []metrics.SensorReading{
			{Key: "cpu0", TemperatureCelsius: 55.0},
		},
		ContainerMetrics: []metrics.ContainerMetric{
			{ContainerID: "c1", ContainerName: "app", CPUPercent: 10.0},
		},
		HostInfo: metrics.HostInfoSnapshot{
			PlatformVersion:  "22.04",
			KernelVersion:    "6.1.0-amd64",
			KernelArch:       "x86_64",
			CPUModelName:     "Intel Xeon E5",
			CPUPhysicalCount: 4,
			CPULogicalCount:  8,
			CPUMhz:           2400.0,
			ContainerRuntime: "docker",
		},
	}

	if err := c.SendMetrics("agent-1", "key-1", "1.0.0", m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received.Metrics.CpuUsagePercent != 45.5 {
		t.Errorf("CpuUsagePercent: got %v, want 45.5", received.Metrics.CpuUsagePercent)
	}
	if received.Metrics.CpuIowaitPercent != 3.2 {
		t.Errorf("CpuIowaitPercent: got %v, want 3.2", received.Metrics.CpuIowaitPercent)
	}
	if received.Metrics.CpuStealPercent != 1.1 {
		t.Errorf("CpuStealPercent: got %v, want 1.1", received.Metrics.CpuStealPercent)
	}
	if received.Metrics.MemoryBuffersBytes != 512*1024*1024 {
		t.Errorf("MemoryBuffersBytes: got %d, want %d", received.Metrics.MemoryBuffersBytes, 512*1024*1024)
	}
	if received.Metrics.MemoryCachedBytes != 1024*1024*1024 {
		t.Errorf("MemoryCachedBytes: got %d, want %d", received.Metrics.MemoryCachedBytes, 1024*1024*1024)
	}
	if received.Metrics.SwapTotalBytes != 2*1024*1024*1024 {
		t.Errorf("SwapTotalBytes: got %d, want %d", received.Metrics.SwapTotalBytes, 2*1024*1024*1024)
	}
	if received.Metrics.SwapUsedBytes != 512*1024*1024 {
		t.Errorf("SwapUsedBytes: got %d, want %d", received.Metrics.SwapUsedBytes, 512*1024*1024)
	}
	if received.Metrics.ProcessesCount != 200 {
		t.Errorf("ProcessesCount: got %d, want 200", received.Metrics.ProcessesCount)
	}
	if len(received.Metrics.SensorReadings) != 1 {
		t.Errorf("SensorReadings: got %d, want 1", len(received.Metrics.SensorReadings))
	} else if received.Metrics.SensorReadings[0].Key != "cpu0" {
		t.Errorf("SensorReadings[0].Key: got %q, want %q", received.Metrics.SensorReadings[0].Key, "cpu0")
	}
	if len(received.ContainerMetrics) != 1 {
		t.Errorf("ContainerMetrics: got %d, want 1", len(received.ContainerMetrics))
	}
	if received.AgentVersion != "1.0.0" {
		t.Errorf("AgentVersion: got %q, want %q", received.AgentVersion, "1.0.0")
	}
	if received.HostInfo == nil {
		t.Fatal("HostInfo should not be nil")
	}
	if received.HostInfo.PlatformVersion != "22.04" {
		t.Errorf("HostInfo.PlatformVersion: got %q, want %q", received.HostInfo.PlatformVersion, "22.04")
	}
	if received.HostInfo.KernelVersion != "6.1.0-amd64" {
		t.Errorf("HostInfo.KernelVersion: got %q, want %q", received.HostInfo.KernelVersion, "6.1.0-amd64")
	}
	if received.HostInfo.CpuModelName != "Intel Xeon E5" {
		t.Errorf("HostInfo.CpuModelName: got %q, want %q", received.HostInfo.CpuModelName, "Intel Xeon E5")
	}
	if received.HostInfo.CpuPhysicalCount != 4 {
		t.Errorf("HostInfo.CpuPhysicalCount: got %d, want 4", received.HostInfo.CpuPhysicalCount)
	}
	if received.HostInfo.CpuLogicalCount != 8 {
		t.Errorf("HostInfo.CpuLogicalCount: got %d, want 8", received.HostInfo.CpuLogicalCount)
	}
	if received.HostInfo.CpuMhz != 2400.0 {
		t.Errorf("HostInfo.CpuMhz: got %f, want 2400.0", received.HostInfo.CpuMhz)
	}
	if received.HostInfo.ContainerRuntime != "docker" {
		t.Errorf("HostInfo.ContainerRuntime: got %q, want %q", received.HostInfo.ContainerRuntime, "docker")
	}
}

func TestSendMetrics_Rejected(t *testing.T) {
	mock := &mockAgentServer{
		sendMetricsFn: func(_ *pb.SendMetricsRequest) (*pb.SendMetricsResponse, error) {
			return &pb.SendMetricsResponse{Success: false, Message: "invalid agent"}, nil
		},
	}

	c := startMockServer(t, mock)

	err := c.SendMetrics("agent-1", "key-1", "1.0.0", &metrics.SystemMetrics{})
	if err == nil {
		t.Fatal("expected error for rejected metrics, got nil")
	}
}

func TestSendMetrics_NoContainerMetrics(t *testing.T) {
	var received *pb.SendMetricsRequest

	mock := &mockAgentServer{
		sendMetricsFn: func(req *pb.SendMetricsRequest) (*pb.SendMetricsResponse, error) {
			received = req
			return &pb.SendMetricsResponse{Success: true}, nil
		},
	}

	c := startMockServer(t, mock)

	if err := c.SendMetrics("agent-1", "key-1", "1.0.0", &metrics.SystemMetrics{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(received.ContainerMetrics) != 0 {
		t.Errorf("expected no container metrics, got %d", len(received.ContainerMetrics))
	}
}

// --- SendPackageInventory ---

func TestSendPackageInventory_Success(t *testing.T) {
	var received *pb.SendPackageInventoryRequest

	mock := &mockAgentServer{
		sendPackageInvFn: func(req *pb.SendPackageInventoryRequest) (*pb.SendPackageInventoryResponse, error) {
			received = req
			return &pb.SendPackageInventoryResponse{Success: true}, nil
		},
	}

	c := startMockServer(t, mock)

	data := &PackageInventoryData{
		InventoryType:        "full",
		TotalPackageCount:    3,
		CollectionDurationMs: 120,
		AllPackages: []*pb.Package{
			{Name: "curl", Version: "7.0", PackageManager: "apt"},
		},
	}

	if err := c.SendPackageInventory("agent-1", "key-1", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if received.InventoryType != "full" {
		t.Errorf("InventoryType: got %q, want %q", received.InventoryType, "full")
	}
	if received.TotalPackageCount != 3 {
		t.Errorf("TotalPackageCount: got %d, want 3", received.TotalPackageCount)
	}
	if len(received.AllPackages) != 1 || received.AllPackages[0].Name != "curl" {
		t.Errorf("AllPackages: unexpected value")
	}
}

func TestSendPackageInventory_Rejected(t *testing.T) {
	mock := &mockAgentServer{
		sendPackageInvFn: func(_ *pb.SendPackageInventoryRequest) (*pb.SendPackageInventoryResponse, error) {
			return &pb.SendPackageInventoryResponse{Success: false, Message: "quota exceeded"}, nil
		},
	}

	c := startMockServer(t, mock)

	err := c.SendPackageInventory("agent-1", "key-1", &PackageInventoryData{})
	if err == nil {
		t.Fatal("expected error for rejected package inventory, got nil")
	}
}
