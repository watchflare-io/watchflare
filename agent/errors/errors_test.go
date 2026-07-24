package errors

import (
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- IsTimestampError ---

func TestIsTimestampError_Nil(t *testing.T) {
	if IsTimestampError(nil) {
		t.Error("expected false for nil error")
	}
}

func TestIsTimestampError_NonGRPC(t *testing.T) {
	if IsTimestampError(fmt.Errorf("some random error")) {
		t.Error("expected false for non-gRPC error")
	}
}

func TestIsTimestampError_WrongCode(t *testing.T) {
	err := status.Error(codes.Unauthenticated, timestampErrorMsg)
	if IsTimestampError(err) {
		t.Error("expected false for wrong gRPC code")
	}
}

func TestIsTimestampError_WrongMessage(t *testing.T) {
	err := status.Error(codes.InvalidArgument, "some other validation error")
	if IsTimestampError(err) {
		t.Error("expected false for wrong message")
	}
}

func TestIsTimestampError_True(t *testing.T) {
	err := status.Error(codes.InvalidArgument, timestampErrorMsg)
	if !IsTimestampError(err) {
		t.Error("expected true for exact timestamp error")
	}
}

func TestIsTimestampError_MessageContains(t *testing.T) {
	// Hub may include extra context, Contains() should still match
	err := status.Error(codes.InvalidArgument, "error: "+timestampErrorMsg+", delta=6m")
	if !IsTimestampError(err) {
		t.Error("expected true when message contains the timestamp error phrase")
	}
}

// --- FormatError ---

func TestFormatError_TimestampError(t *testing.T) {
	err := status.Error(codes.InvalidArgument, timestampErrorMsg)
	result := FormatError(err, "Heartbeat")

	if !strings.Contains(result, "CLOCK SYNC ERROR") {
		t.Errorf("expected CLOCK SYNC ERROR in output, got: %s", result)
	}
	if !strings.Contains(result, "Heartbeat") {
		t.Errorf("expected context 'Heartbeat' in output, got: %s", result)
	}
}

func TestFormatError_RegularError(t *testing.T) {
	err := fmt.Errorf("connection refused")
	result := FormatError(err, "SendMetrics")

	if strings.Contains(result, "CLOCK SYNC ERROR") {
		t.Errorf("unexpected CLOCK SYNC ERROR for regular error, got: %s", result)
	}
	if !strings.Contains(result, "SendMetrics") {
		t.Errorf("expected context 'SendMetrics' in output, got: %s", result)
	}
	if !strings.Contains(result, "connection refused") {
		t.Errorf("expected error message in output, got: %s", result)
	}
}
