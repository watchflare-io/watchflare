package services

import (
	"fmt"
	"time"

	"watchflare/backend/models"
)

// buildAlertContent formats the title and body of an outgoing alert
// notification. The same content is broadcast to every notification channel
// regardless of service: per-service rendering is now handled by Shoutrrr.
func buildAlertContent(hostName, metricType string, threshold, currentValue float64, startedAt time.Time) (title, body string) {
	if metricType == models.MetricTypeHostDown {
		title = fmt.Sprintf("%s is offline", hostName)
		body = fmt.Sprintf("Host %q has been offline since %s.", hostName, startedAt.Format(time.RFC1123))
		return
	}
	label := metricLabel(metricType)
	valueDesc := formatMetricValue(metricType, threshold, currentValue)
	title = fmt.Sprintf("%s: %s exceeded", hostName, label)
	body = fmt.Sprintf("Alert for host %q: %s is %s\nStarted at %s.", hostName, label, valueDesc, startedAt.Format(time.RFC1123))
	return
}

// buildResolutionContent formats the title and body of an alert-resolution
// notification.
func buildResolutionContent(hostName, metricType string, startedAt, resolvedAt time.Time) (title, body string) {
	duration := resolvedAt.Sub(startedAt).Round(time.Second)
	if metricType == models.MetricTypeHostDown {
		title = fmt.Sprintf("%s is back online", hostName)
		body = fmt.Sprintf("Host %q is back online.\nWas offline for %s (since %s).", hostName, duration, startedAt.Format(time.RFC1123))
		return
	}
	label := metricLabel(metricType)
	title = fmt.Sprintf("%s: %s back to normal", hostName, label)
	body = fmt.Sprintf("Alert resolved for host %q: %s is back to normal.\nDuration: %s.", hostName, label, duration)
	return
}

func metricLabel(metricType string) string {
	switch metricType {
	case models.MetricTypeCPUUsage:
		return "CPU usage"
	case models.MetricTypeMemoryUsage:
		return "Memory usage"
	case models.MetricTypeDiskUsage:
		return "Disk usage"
	case models.MetricTypeLoadAvg:
		return "Load average (1m)"
	case models.MetricTypeLoadAvg5:
		return "Load average (5m)"
	case models.MetricTypeLoadAvg15:
		return "Load average (15m)"
	case models.MetricTypeTemperature:
		return "CPU temperature"
	}
	return metricType
}

func formatMetricValue(metricType string, threshold, currentValue float64) string {
	switch metricType {
	case models.MetricTypeCPUUsage, models.MetricTypeMemoryUsage, models.MetricTypeDiskUsage:
		return fmt.Sprintf("%.1f%% (threshold: %.0f%%)", currentValue, threshold)
	case models.MetricTypeTemperature:
		return fmt.Sprintf("%.1f°C (threshold: %.0f°C)", currentValue, threshold)
	case models.MetricTypeLoadAvg, models.MetricTypeLoadAvg5, models.MetricTypeLoadAvg15:
		return fmt.Sprintf("%.2f (threshold: %.2f)", currentValue, threshold)
	}
	return fmt.Sprintf("%.2f (threshold: %.2f)", currentValue, threshold)
}
