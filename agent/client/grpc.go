package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"watchflare-agent/metrics"
	"watchflare-agent/security"
	pb "watchflare/shared/proto/agent/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Client handles gRPC communication with the backend
type Client struct {
	conn   *grpc.ClientConn
	client pb.AgentServiceClient
}

// New creates a new gRPC client with strict TLS verification
// Requires a valid CA certificate file for TLS verification
func New(host, port, caCertFile, serverName string) (*Client, error) {
	addr := fmt.Sprintf("%s:%s", host, port)

	// Load CA certificate (mandatory for TLS)
	caCert, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	// Create cert pool and add CA cert
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	// Create TLS config with strict verification
	tlsConfig := &tls.Config{
		RootCAs:    certPool,
		ServerName: serverName, // For SNI and certificate verification
		MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13,
	}

	creds := credentials.NewTLS(tlsConfig)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	return &Client{
		conn:   conn,
		client: pb.NewAgentServiceClient(conn),
	}, nil
}

// NewForRegistration creates a gRPC client for initial registration with permissive TLS
// This allows the agent to connect without prior knowledge of the CA certificate
// The CA cert will be received during registration and used for strict verification afterward
func NewForRegistration(host, port string) (*Client, error) {
	addr := fmt.Sprintf("%s:%s", host, port)

	// Use permissive TLS for bootstrap (accepts any certificate)
	// This is safe because registration requires a secret token as root of trust
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // Only for initial registration
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
	}

	creds := credentials.NewTLS(tlsConfig)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	return &Client{
		conn:   conn,
		client: pb.NewAgentServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// RegisterRequest contains all parameters needed to register the agent
type RegisterRequest struct {
	Token                string
	Hostname             string
	IPv4                 string
	IPv6                 string
	OS                   string
	Platform             string
	PlatformFamily       string
	PlatformVersion      string
	KernelVersion        string
	KernelArch           string
	VirtualizationSystem string
	VirtualizationRole   string
	HostID               string
	EnvironmentType      string
	ContainerRuntime     string
	CPUModelName         string
	CPUPhysicalCount     int32
	CPULogicalCount      int32
	CPUMhz               float64
	ExistingUUID         string
	AgentVersion         string
}

// RegistrationResponse contains the result of a successful registration
type RegistrationResponse struct {
	AgentID     string
	AgentKey    string
	CACert      string // CA certificate in PEM format
	ServerName  string // Server name for TLS verification
	Reactivated bool   // True if existing agent was reactivated (UUID reused)
}

// Register attempts to register the agent with the backend
// Returns registration credentials and TLS information
func (c *Client) Register(r RegisterRequest) (*RegistrationResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Note: Registration uses token-based auth, not HMAC
	// HMAC is only used after successful registration
	req := &pb.RegisterHostRequest{
		RegistrationToken:    r.Token,
		Hostname:             r.Hostname,
		IpAddressV4:          r.IPv4,
		IpAddressV6:          r.IPv6,
		Os:                   r.OS,
		Platform:             r.Platform,
		PlatformFamily:       r.PlatformFamily,
		PlatformVersion:      r.PlatformVersion,
		KernelVersion:        r.KernelVersion,
		KernelArch:           r.KernelArch,
		VirtualizationSystem: r.VirtualizationSystem,
		VirtualizationRole:   r.VirtualizationRole,
		HostId:               r.HostID,
		EnvironmentType:      r.EnvironmentType,
		ContainerRuntime:     r.ContainerRuntime,
		CpuModelName:         r.CPUModelName,
		CpuPhysicalCount:     r.CPUPhysicalCount,
		CpuLogicalCount:      r.CPULogicalCount,
		CpuMhz:               r.CPUMhz,
		Timestamp:            time.Now().Unix(), // Anti-replay
		ExistingAgentUuid:    r.ExistingUUID,
		AgentVersion:         r.AgentVersion,
	}

	resp, err := c.client.RegisterHost(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("registration failed: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("registration rejected: %s", resp.Message)
	}

	return &RegistrationResponse{
		AgentID:     resp.AgentId,
		AgentKey:    resp.AgentKey,
		CACert:      resp.CaCert,
		ServerName:  resp.ServerName,
		Reactivated: resp.Reactivated,
	}, nil
}

// SaveCACertificate saves the CA certificate to disk.
// The directory will be created if it doesn't exist.
// When running as root, the file is chowned to the appropriate service group so
// the unprivileged service user can read it (Linux: root:watchflare, macOS: root:staff).
func SaveCACertificate(caCertPEM, certPath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(certPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create PKI directory: %w", err)
	}

	// CA certificate: 640 so only root and the service group can read it
	if err := os.WriteFile(certPath, []byte(caCertPEM), 0640); err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}

	// Fix ownership so the service user can read the certificate.
	// Linux: root:watchflare (service runs as unprivileged watchflare user)
	// macOS: root:staff (Homebrew service runs as the invoking user, who is in staff)
	if os.Geteuid() == 0 {
		var groupName string
		switch runtime.GOOS {
		case "linux":
			groupName = "watchflare"
		case "darwin":
			groupName = "staff"
		}
		if groupName != "" {
			if grp, err := user.LookupGroup(groupName); err == nil {
				if gid, err := strconv.Atoi(grp.Gid); err == nil {
					_ = os.Chown(certPath, 0, gid)
				}
			}
		}
	}

	return nil
}

// SendHeartbeat sends a heartbeat to the backend and returns any pending commands.
func (c *Client) SendHeartbeat(agentID, agentKey, ipv4, ipv6, agentVersion string) ([]*pb.PendingCommand, error) {
	timestamp := time.Now().Unix()

	req := &pb.HeartbeatRequest{
		AgentId:      agentID,
		AgentKey:     agentKey,
		IpAddressV4:  ipv4,
		IpAddressV6:  ipv6,
		Timestamp:    timestamp,
		AgentVersion: agentVersion,
	}

	// Attach HMAC authentication metadata
	ctx := context.Background()
	ctx, err := security.AttachAuthMetadata(ctx, agentID, agentKey, timestamp, req)
	if err != nil {
		return nil, fmt.Errorf("failed to attach auth metadata: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.Heartbeat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("heartbeat failed: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("heartbeat rejected: %s", resp.Message)
	}

	return resp.Commands, nil
}

// SendMetrics sends system metrics to the backend
func (c *Client) SendMetrics(agentID, agentKey, agentVersion string, m *metrics.SystemMetrics) error {
	timestamp := time.Now().Unix()

	var pbSensorReadings []*pb.SensorReading
	for _, sr := range m.SensorReadings {
		pbSensorReadings = append(pbSensorReadings, &pb.SensorReading{
			Key:                sr.Key,
			TemperatureCelsius: sr.TemperatureCelsius,
		})
	}

	req := &pb.SendMetricsRequest{
		AgentId:  agentID,
		AgentKey: agentKey,
		Metrics: &pb.Metrics{
			CpuUsagePercent:      m.CPUUsagePercent,
			CpuIowaitPercent:     m.CPUIowaitPercent,
			CpuStealPercent:      m.CPUStealPercent,
			MemoryTotalBytes:     m.MemoryTotalBytes,
			MemoryUsedBytes:      m.MemoryUsedBytes,
			MemoryAvailableBytes: m.MemoryAvailableBytes,
			MemoryBuffersBytes:   m.MemoryBuffersBytes,
			MemoryCachedBytes:    m.MemoryCachedBytes,
			SwapTotalBytes:       m.SwapTotalBytes,
			SwapUsedBytes:        m.SwapUsedBytes,
			LoadAvg_1Min:         m.LoadAvg1Min,
			LoadAvg_5Min:         m.LoadAvg5Min,
			LoadAvg_15Min:        m.LoadAvg15Min,
			DiskTotalBytes:       m.DiskTotalBytes,
			DiskUsedBytes:        m.DiskUsedBytes,
			UptimeSeconds:        m.UptimeSeconds,
			ProcessesCount:       m.ProcessesCount,
			Timestamp:            m.Timestamp,

			DiskReadBytesPerSec:   m.DiskReadBytesPerSec,
			DiskWriteBytesPerSec:  m.DiskWriteBytesPerSec,
			NetworkRxBytesPerSec:  m.NetworkRxBytesPerSec,
			NetworkTxBytesPerSec:  m.NetworkTxBytesPerSec,
			CpuTemperatureCelsius: m.CPUTemperatureCelsius,
			SensorReadings:        pbSensorReadings,
		},
		Timestamp:    timestamp, // Request-level timestamp for anti-replay
		AgentVersion: agentVersion,
		HostInfo: &pb.HostInfo{
			PlatformVersion:  m.HostInfo.PlatformVersion,
			KernelVersion:    m.HostInfo.KernelVersion,
			KernelArch:       m.HostInfo.KernelArch,
			CpuModelName:     m.HostInfo.CPUModelName,
			CpuPhysicalCount: m.HostInfo.CPUPhysicalCount,
			CpuLogicalCount:  m.HostInfo.CPULogicalCount,
			CpuMhz:           m.HostInfo.CPUMhz,
			ContainerRuntime: m.HostInfo.ContainerRuntime,
		},
	}

	// Map container metrics if present
	for _, cm := range m.ContainerMetrics {
		req.ContainerMetrics = append(req.ContainerMetrics, &pb.ContainerMetric{
			ContainerId:          cm.ContainerID,
			ContainerName:        cm.ContainerName,
			Image:                cm.Image,
			CpuPercent:           cm.CPUPercent,
			MemoryUsedBytes:      cm.MemoryUsedBytes,
			MemoryLimitBytes:     cm.MemoryLimitBytes,
			NetworkRxBytesPerSec: cm.NetworkRxBytesPerSec,
			NetworkTxBytesPerSec: cm.NetworkTxBytesPerSec,
		})
	}

	// Attach HMAC authentication metadata
	ctx := context.Background()
	ctx, err := security.AttachAuthMetadata(ctx, agentID, agentKey, timestamp, req)
	if err != nil {
		return fmt.Errorf("failed to attach auth metadata: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.client.SendMetrics(ctx, req)
	if err != nil {
		return fmt.Errorf("send metrics failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("metrics rejected: %s", resp.Message)
	}

	return nil
}

// PackageInventoryData contains the package inventory to send
type PackageInventoryData struct {
	InventoryType        string
	AddedPackages        []*pb.Package
	RemovedPackages      []*pb.Package
	UpdatedPackages      []*pb.Package
	AllPackages          []*pb.Package
	CollectionDurationMs int64
	TotalPackageCount    int32
}

// SendPackageInventory sends package inventory to the backend
func (c *Client) SendPackageInventory(agentID, agentKey string, data *PackageInventoryData) error {
	timestamp := time.Now().Unix()

	req := &pb.SendPackageInventoryRequest{
		AgentId:              agentID,
		AgentKey:             agentKey,
		Timestamp:            timestamp,
		InventoryType:        data.InventoryType,
		AddedPackages:        data.AddedPackages,
		RemovedPackages:      data.RemovedPackages,
		UpdatedPackages:      data.UpdatedPackages,
		AllPackages:          data.AllPackages,
		CollectionDurationMs: data.CollectionDurationMs,
		TotalPackageCount:    data.TotalPackageCount,
	}

	// Attach HMAC authentication metadata
	ctx := context.Background()
	ctx, err := security.AttachAuthMetadata(ctx, agentID, agentKey, timestamp, req)
	if err != nil {
		return fmt.Errorf("failed to attach auth metadata: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second) // Longer timeout for package inventory
	defer cancel()

	resp, err := c.client.SendPackageInventory(ctx, req)
	if err != nil {
		return fmt.Errorf("send package inventory failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("package inventory rejected: %s", resp.Message)
	}

	return nil
}
