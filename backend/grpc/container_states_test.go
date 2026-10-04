package grpc

import (
	"context"
	"testing"
	"time"

	"watchflare/backend/database"
	"watchflare/backend/models"
	pb "watchflare/shared/proto/agent/v1"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestContainerStatesTableExists(t *testing.T) {
	setupGRPCTestDB(t)
	if !database.DB.Migrator().HasTable("container_states") {
		t.Fatal("container_states table was not created by migration")
	}
}

func TestSendMetrics_ReplacesContainerStates(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "container-states-host",
		Status:      models.StatusOnline,
		AgentKey:    "container-states-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() {
		database.DB.Where("host_id = ?", host.ID).Delete(&models.ContainerState{})
		database.DB.Unscoped().Delete(host)
	})

	ctx := context.Background()

	// First report: two containers.
	_, err := s.SendMetrics(ctx, &pb.SendMetricsRequest{
		AgentId:  host.AgentID,
		AgentKey: host.AgentKey,
		Metrics:  &pb.Metrics{Timestamp: time.Now().Unix()},
		ContainerMetrics: []*pb.ContainerMetric{
			{ContainerId: "aaa", ContainerName: "web", Image: "nginx", CpuPercent: 5, ContainerRuntime: "docker", Status: "Up 2 hours", Health: "healthy"},
			{ContainerId: "bbb", ContainerName: "db", Image: "postgres", CpuPercent: 10, ContainerRuntime: "docker", Status: "Up 1 hour"},
		},
	})
	require.NoError(t, err)

	// Second report: a different set (web gone, cache added). Must fully replace.
	_, err = s.SendMetrics(ctx, &pb.SendMetricsRequest{
		AgentId:  host.AgentID,
		AgentKey: host.AgentKey,
		Metrics:  &pb.Metrics{Timestamp: time.Now().Unix()},
		ContainerMetrics: []*pb.ContainerMetric{
			{ContainerId: "bbb", ContainerName: "db", Image: "postgres", CpuPercent: 12, ContainerRuntime: "docker", Status: "Up 1 hour"},
			{ContainerId: "ccc", ContainerName: "cache", Image: "redis", CpuPercent: 1, ContainerRuntime: "docker", Status: "Up 5 minutes"},
		},
	})
	require.NoError(t, err)

	var states []models.ContainerState
	require.NoError(t, database.DB.Where("host_id = ?", host.ID).Order("container_id").Find(&states).Error)
	require.Len(t, states, 2)
	require.Equal(t, "bbb", states[0].ContainerID)
	require.Equal(t, "ccc", states[1].ContainerID)
	require.Equal(t, "cache", states[1].ContainerName)
	require.Equal(t, "redis", states[1].Image)
}

func TestSendContainerImageUpdates_ReplacesAndValidates(t *testing.T) {
	setupGRPCTestDB(t)
	s := NewAgentServer()

	host := &models.Host{
		ID:          uuid.New().String(),
		AgentID:     uuid.New().String(),
		DisplayName: "image-updates-host",
		Status:      models.StatusOnline,
		AgentKey:    "image-updates-key-abc123",
	}
	require.NoError(t, database.DB.Create(host).Error)
	t.Cleanup(func() {
		database.DB.Where("host_id = ?", host.ID).Delete(&models.ContainerImageUpdate{})
		database.DB.Unscoped().Delete(host)
	})

	ctx := context.Background()
	checkedAt := time.Now().Add(-time.Minute).Unix()

	resp, err := s.SendContainerImageUpdates(ctx, &pb.SendContainerImageUpdatesRequest{
		AgentId:  host.AgentID,
		AgentKey: host.AgentKey,
		Updates: []*pb.ContainerImageUpdate{
			{ContainerId: "aaa", ContainerName: "web", Image: "nginx:latest", LocalDigest: "sha256:old", RemoteDigest: "sha256:new", Status: "update_available", CheckedAt: checkedAt,
				LocalVersion: "2025-08-05+277", RemoteVersion: "2025-08-05+279", CommitsBehind: 2},
			{ContainerId: "bbb", ContainerName: "db", Image: "postgres:16", Status: "up_to_date", CheckedAt: checkedAt},
			{ContainerId: "ccc", ContainerName: "bogus", Image: "x", Status: "not-a-status", CheckedAt: checkedAt},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.Success)

	var rows []models.ContainerImageUpdate
	require.NoError(t, database.DB.Where("host_id = ?", host.ID).Order("container_id").Find(&rows).Error)
	require.Len(t, rows, 2, "row with invalid status must be dropped")
	require.Equal(t, "update_available", rows[0].Status)
	require.Equal(t, "sha256:new", rows[0].RemoteDigest)
	require.Equal(t, checkedAt, rows[0].CheckedAt.Unix())
	require.Equal(t, "2025-08-05+277", rows[0].LocalVersion)
	require.Equal(t, "2025-08-05+279", rows[0].RemoteVersion)
	require.Equal(t, 2, rows[0].CommitsBehind)

	// An empty report clears the host's rows (e.g. all containers stopped).
	resp, err = s.SendContainerImageUpdates(ctx, &pb.SendContainerImageUpdatesRequest{AgentId: host.AgentID, AgentKey: host.AgentKey})
	require.NoError(t, err)
	require.True(t, resp.Success)
	var count int64
	require.NoError(t, database.DB.Model(&models.ContainerImageUpdate{}).Where("host_id = ?", host.ID).Count(&count).Error)
	require.Zero(t, count)
}

func TestSendContainerImageUpdates_InvalidCredentials(t *testing.T) {
	setupGRPCTestDB(t)
	resp, err := NewAgentServer().SendContainerImageUpdates(context.Background(), &pb.SendContainerImageUpdatesRequest{
		AgentId:  uuid.New().String(),
		AgentKey: "wrong",
	})
	require.NoError(t, err)
	require.False(t, resp.Success)
}
