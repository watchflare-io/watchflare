package cmd

import "testing"

func TestHasClockSyncError_Detected(t *testing.T) {
	logContent := `2026/03/27 10:00:00 INFO   heartbeat sent
2026/03/27 10:00:05 ERROR  clock out of sync with Hub  delta=6m30s
2026/03/27 10:00:10 INFO   heartbeat sent`

	if !hasClockSyncError(logContent) {
		t.Error("expected clock sync error to be detected")
	}
}

func TestHasClockSyncError_NotPresent(t *testing.T) {
	logContent := `2026/03/27 10:00:00 INFO   heartbeat sent
2026/03/27 10:00:05 INFO   metrics sent
2026/03/27 10:00:10 INFO   heartbeat sent`

	if hasClockSyncError(logContent) {
		t.Error("expected no clock sync error")
	}
}

func TestHasClockSyncError_EmptyLog(t *testing.T) {
	if hasClockSyncError("") {
		t.Error("expected no clock sync error for empty log")
	}
}

func TestHasClockSyncError_PartialMatch(t *testing.T) {
	// "clock out of sync" without "with Hub" should not match
	logContent := "clock out of sync with other system"
	if hasClockSyncError(logContent) {
		t.Error("expected no match for partial string")
	}
}

func TestHasClockSyncError_ExactPhrase(t *testing.T) {
	if !hasClockSyncError("clock out of sync with Hub") {
		t.Error("expected match for exact phrase")
	}
}
