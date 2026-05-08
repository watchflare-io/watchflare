<script lang="ts">
	import UPlotChart from '$lib/components/UPlotChart.svelte';
	import { dynamicChartColor } from '$lib/chart-utils';
	import { userStore } from '$lib/stores/user';
	import * as api from '$lib/api';
	import type { Metric, SensorDataPoint, TimeRange } from '$lib/types';
	import type uPlot from 'uplot';

	let { data = [], hostId, timeRange }: {
		data: Metric[];
		hostId: string;
		timeRange?: TimeRange;
	} = $props();

	const tempUnit = $derived($userStore.user?.temperature_unit ?? 'celsius');

	function fmtTemp(v: number | null): string {
		if (v == null) return '—';
		if (tempUnit === 'fahrenheit') return `${(v * 9 / 5 + 32).toFixed(1)}°F`;
		return `${v.toFixed(1)}°C`;
	}


	// CPU-related sensor patterns (sorted first in the legend)
	const CPU_PATTERNS = ['tdie', 'coretemp', 'k10temp', 'cpu', 'package', 'tctl'];

	function isCPUKey(key: string): boolean {
		const lower = key.toLowerCase();
		return CPU_PATTERNS.some(p => lower.includes(p));
	}

	// For aggregated views (>1h), fetch sensor readings from the dedicated endpoint
	let fetchedSensorData: SensorDataPoint[] | null = $state(null);

	$effect(() => {
		if (!hostId || !timeRange || timeRange === '1h') {
			fetchedSensorData = null;
			return;
		}
		api.getSensorReadings(hostId, timeRange).then(res => {
			fetchedSensorData = res.data ?? null;
		}).catch(() => {
			fetchedSensorData = null;
		});
	});

	// Active source: fetched data for >1h, inline sensor_readings for 1h
	const activeData = $derived(fetchedSensorData ?? null);

	// Sorted unique sensor keys: CPU sensors first, then alphabetical
	const sensorKeys = $derived.by(() => {
		const keys = new Set<string>();
		if (activeData) {
			for (const d of activeData) {
				for (const sr of d.sensor_readings) keys.add(sr.key);
			}
		} else {
			for (const d of data) {
				if (d.sensor_readings) {
					for (const sr of d.sensor_readings) keys.add(sr.key);
				}
			}
		}
		return [...keys].sort((a, b) => {
			const aCPU = isCPUKey(a);
			const bCPU = isCPUKey(b);
			if (aCPU !== bCPU) return aCPU ? -1 : 1;
			return a.localeCompare(b);
		});
	});

	const hasSensorReadings = $derived(sensorKeys.length > 0);

	const chartData = $derived.by((): uPlot.AlignedData => {
		if (activeData) {
			if (activeData.length === 0) return [[], []] as uPlot.AlignedData;
			const timestamps: number[] = [];
			const seriesArrays: (number | null)[][] = sensorKeys.map(() => []);
			for (const d of activeData) {
				timestamps.push(new Date(d.timestamp).getTime() / 1000);
				const readingMap = new Map(d.sensor_readings.map(sr => [sr.key, sr.temperature_celsius]));
				for (let i = 0; i < sensorKeys.length; i++) {
					const val = readingMap.get(sensorKeys[i]);
					seriesArrays[i].push(val != null ? val : null);
				}
			}
			return [timestamps, ...seriesArrays] as uPlot.AlignedData;
		}

		// 1h path: use inline sensor_readings from Metric[]
		if (data.length === 0) return [[], []] as uPlot.AlignedData;
		const timestamps: number[] = [];

		if (hasSensorReadings) {
			const seriesArrays: (number | null)[][] = sensorKeys.map(() => []);
			for (const d of data) {
				timestamps.push(new Date(d.timestamp).getTime() / 1000);
				if (d.sensor_readings && d.sensor_readings.length > 0) {
					const readingMap = new Map(d.sensor_readings.map(sr => [sr.key, sr.temperature_celsius]));
					for (let i = 0; i < sensorKeys.length; i++) {
						const val = readingMap.get(sensorKeys[i]);
						seriesArrays[i].push(val != null ? val : null);
					}
				} else {
					for (let i = 0; i < sensorKeys.length; i++) {
						seriesArrays[i].push(i === 0 && d.cpu_temperature_celsius > 0 ? d.cpu_temperature_celsius : null);
					}
				}
			}
			return [timestamps, ...seriesArrays] as uPlot.AlignedData;
		}

		// Fallback: single cpu_temperature_celsius curve
		const temp: (number | null)[] = [];
		for (const d of data) {
			if (d.cpu_temperature_celsius > 0) {
				timestamps.push(new Date(d.timestamp).getTime() / 1000);
				temp.push(d.cpu_temperature_celsius);
			}
		}
		return [timestamps, temp] as uPlot.AlignedData;
	});

	const series = $derived(
		hasSensorReadings
			? sensorKeys.map((key, i): uPlot.Series => ({
				label: key,
				stroke: dynamicChartColor(i, sensorKeys.length),
				width: 2,
				value: (_u: uPlot, v: number | null) => fmtTemp(v),
			}))
			: [{
				label: 'CPU Temp',
				stroke: dynamicChartColor(0, 1),
				fill: dynamicChartColor(0, 1),
				width: 2,
				value: (_u: uPlot, v: number | null) => fmtTemp(v),
			}] as uPlot.Series[]
	);

	const axes = $derived<uPlot.Axis[]>([
		{},
		{
			size: 68,
			values: (_u: uPlot, ticks: number[]) => ticks.map(v => fmtTemp(v)),
		}
	]);

	const scales: uPlot.Scales = {
		y: {
			range: (_u: uPlot, min: number, max: number): uPlot.Range.MinMax => {
				const padding = Math.max((max - min) * 0.1, 2);
				return [Math.floor(min - padding), Math.ceil(max + padding)];
			}
		}
	};
</script>

<UPlotChart data={chartData} {series} {axes} {scales} {timeRange} />
