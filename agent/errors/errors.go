package errors

import (
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// timestampErrorMsg is the exact message returned by the Hub gRPC interceptor
// when the agent clock is more than 5 minutes out of sync.
const timestampErrorMsg = "Timestamp outside acceptable window"

// IsTimestampError checks if an error is a timestamp synchronization issue.
// The Hub returns codes.InvalidArgument with a specific message when the
// agent clock is more than 5 minutes out of sync.
func IsTimestampError(err error) bool {
	if err == nil {
		return false
	}
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return st.Code() == codes.InvalidArgument &&
		strings.Contains(st.Message(), timestampErrorMsg)
}

// FormatError formats an error with helpful context
func FormatError(err error, context string) string {
	if IsTimestampError(err) {
		return fmt.Sprintf("%s failed: CLOCK SYNC ERROR - System time is out of sync with the Hub (>5min difference). "+
			"Ensure the system clock is synchronized and restart the agent. Original error: %v", context, err)
	}
	return fmt.Sprintf("%s failed: %v", context, err)
}
