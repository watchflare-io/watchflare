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
