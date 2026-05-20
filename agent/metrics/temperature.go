package metrics

import (
	"strings"

	"github.com/shirou/gopsutil/v4/sensors"
)

// SensorReading represents a single temperature sensor reading
type SensorReading struct {
	Key                string
	TemperatureCelsius float64
}

// collectTemperatures reads all sensors once and returns the CPU temperature
// (first matching CPU sensor) and all valid readings in a single syscall.
func collectTemperatures() (cpuTemp float64, readings []SensorReading, err error) {
	temps, err := sensors.SensorsTemperatures()
	if err != nil {
		return 0, nil, err
	}

	// Valid range: -50–120°C (excludes gopsutil macOS stubs returning ~-9200°C)
	var cpuFound bool
	for _, temp := range temps {
		if temp.Temperature < -50 || temp.Temperature > 120 {
			continue
		}
		readings = append(readings, SensorReading{
			Key:                temp.SensorKey,
			TemperatureCelsius: temp.Temperature,
		})
		if !cpuFound && isCPUSensor(temp.SensorKey) {
			cpuTemp = temp.Temperature
			cpuFound = true
		}
	}
	return cpuTemp, readings, nil
}

// isCPUSensor checks if a sensor key corresponds to a CPU temperature sensor
func isCPUSensor(sensorKey string) bool {
	lower := strings.ToLower(sensorKey)
	cpuPatterns := []string{
		"coretemp",      // Intel Linux
		"k10temp",       // AMD Linux
		"cpu_thermal",   // ARM / Raspberry Pi
		"package id 0",  // Intel package temp
		"tctl",          // AMD Ryzen (Tctl)
		"cpu temp",      // Generic fallback
		"cpu die",       // Some systems
		"pmu tdie",      // Apple Silicon (M1/M2/M3) — PMU tdie1..N
	}

	for _, pattern := range cpuPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
