<script lang="ts">
	import { formatBytes, formatPercent } from '$lib/utils';
	import { formatRate, formatTemperature } from '$lib/chart-utils';
	import { userStore } from '$lib/stores/user';
	import CPUChart from '$lib/components/CPUChart.svelte';
	import MemoryChart from '$lib/components/MemoryChart.svelte';
	import DiskChart from '$lib/components/DiskChart.svelte';
	import LoadAvgChart from '$lib/components/LoadAvgChart.svelte';
	import DiskIOChart from '$lib/components/DiskIOChart.svelte';
	import NetworkChart from '$lib/components/NetworkChart.svelte';
	import TemperatureChart from '$lib/components/TemperatureChart.svelte';
	import SwapChart from '$lib/components/SwapChart.svelte';
	import ContainerCPUChart from '$lib/components/ContainerCPUChart.svelte';
	import ContainerMemoryChart from '$lib/components/ContainerMemoryChart.svelte';
	import ContainerNetworkChart from '$lib/components/ContainerNetworkChart.svelte';
	import ContainersTable from '$lib/components/host/ContainersTable.svelte';
	import TimeRangeSelector from '$lib/components/TimeRangeSelector.svelte';
	import type { Metric, ContainerMetric, TimeRange } from '$lib/types';

	let {
		hostId,
		metrics,
		containerMetrics = [],
		timeRange = $bindable()
	}: {
		hostId: string;
		metrics: Metric[];
		containerMetrics?: ContainerMetric[];
		timeRange: TimeRange;
	} = $props();

	const latestMetric = $derived(metrics.length > 0 ? metrics[metrics.length - 1] : null);
	const diskUnit = $derived($userStore.user?.disk_unit ?? 'bytes');
	const networkUnit = $derived($userStore.user?.network_unit ?? 'bytes');
	const tempUnit = $derived($userStore.user?.temperature_unit ?? 'celsius');
	const hasContainerData = $derived(containerMetrics.length > 0);

	// Compute container names once
	const containerNames = $derived([...new Set(containerMetrics.map((d) => d.container_name))]);

	// Pivot container data once, reused by all 3 charts
	const containerPivots = $derived(
		(() => {
			if (containerMetrics.length === 0)
				return { cpu: [], memory: [], network: [], networkKeys: [] };

			const cpuByTs = new Map<string, Record<string, unknown>>();
			const memByTs = new Map<string, Record<string, unknown>>();
			const netByTs = new Map<string, Record<string, unknown>>();

			for (const d of containerMetrics) {
				const ts = d.timestamp;

				if (!cpuByTs.has(ts)) cpuByTs.set(ts, { date: new Date(ts) });
				cpuByTs.get(ts)![d.container_name] = d.cpu_percent;

				if (!memByTs.has(ts)) memByTs.set(ts, { date: new Date(ts) });
				memByTs.get(ts)![d.container_name] = d.memory_used_bytes;

				if (!netByTs.has(ts)) netByTs.set(ts, { date: new Date(ts) });
				netByTs.get(ts)![`${d.container_name} (RX)`] = d.network_rx_bytes_per_sec;
				netByTs.get(ts)![`${d.container_name} (TX)`] = d.network_tx_bytes_per_sec;
			}

			const sortFn = (a: Record<string, unknown>, b: Record<string, unknown>) =>
				(a.date as Date).getTime() - (b.date as Date).getTime();

			return {
				cpu: [...cpuByTs.values()].sort(sortFn),
				memory: [...memByTs.values()].sort(sortFn),
				network: [...netByTs.values()].sort(sortFn),
				networkKeys: containerNames.flatMap((name) => [`${name} (RX)`, `${name} (TX)`])
			};
		})()
	);
</script>

<div class="mb-6">
	<div class="mb-3 flex flex-col sm:flex-row sm:justify-end">
		<TimeRangeSelector bind:value={timeRange} class="w-full sm:w-auto" />
	</div>

	<div class="grid gap-4 2xl:grid-cols-2">
		<div class="rounded-lg border bg-card p-4">
			<div class="mb-3 flex items-center justify-between">
				<h3 class="text-sm font-medium">CPU Usage</h3>
				{#if latestMetric}
					<span class="text-xs text-muted-foreground"
						>{latestMetric.cpu_usage_percent.toFixed(1)}%</span
					>
				{/if}
			</div>
			<CPUChart data={metrics} {timeRange} />
		</div>
		<div class="rounded-lg border bg-card p-4">
			<div class="mb-3 flex items-center justify-between">
				<h3 class="text-sm font-medium">Memory Usage</h3>
				{#if latestMetric}
					<span class="text-xs text-muted-foreground">
						<span class="sm:hidden"
							>{formatPercent(
								latestMetric.memory_total_bytes > 0
									? (latestMetric.memory_used_bytes / latestMetric.memory_total_bytes) * 100
									: 0
							)}</span
						>
						<span class="hidden sm:inline"
							>{formatBytes(latestMetric.memory_used_bytes)} / {formatBytes(
								latestMetric.memory_total_bytes
							)} ({formatPercent(
								latestMetric.memory_total_bytes > 0
									? (latestMetric.memory_used_bytes / latestMetric.memory_total_bytes) * 100
									: 0
							)})</span
						>
					</span>
				{/if}
			</div>
			<MemoryChart data={metrics} {timeRange} />
		</div>
		<div class="rounded-lg border bg-card p-4">
			<div class="mb-3 flex items-center justify-between">
				<h3 class="text-sm font-medium">Disk Usage</h3>
				{#if latestMetric}
					<span class="text-xs text-muted-foreground">
						<span class="sm:hidden"
							>{formatPercent(
								latestMetric.disk_total_bytes > 0
									? (latestMetric.disk_used_bytes / latestMetric.disk_total_bytes) * 100
									: 0
							)}</span
						>
						<span class="hidden sm:inline"
							>{formatBytes(latestMetric.disk_used_bytes)} / {formatBytes(
								latestMetric.disk_total_bytes
							)} ({formatPercent(
								latestMetric.disk_total_bytes > 0
									? (latestMetric.disk_used_bytes / latestMetric.disk_total_bytes) * 100
									: 0
							)})</span
						>
					</span>
				{/if}
			</div>
			<DiskChart data={metrics} {timeRange} />
		</div>
		<div class="rounded-lg border bg-card p-4">
			<div class="mb-3 flex items-center justify-between">
				<h3 class="text-sm font-medium">Load Average</h3>
				{#if latestMetric}
					<span class="text-xs text-muted-foreground">
						{latestMetric.load_avg_1min.toFixed(2)} / {latestMetric.load_avg_5min.toFixed(2)} / {latestMetric.load_avg_15min.toFixed(
							2
						)}
					</span>
				{/if}
			</div>
			<LoadAvgChart data={metrics} {timeRange} />
		</div>
		<div class="rounded-lg border bg-card p-4">
			<div class="mb-3 flex items-center justify-between">
				<h3 class="text-sm font-medium">Disk I/O</h3>
				{#if latestMetric}
					<span class="text-xs text-muted-foreground">
						<span class="sm:hidden"
							>{formatRate(latestMetric.disk_read_bytes_per_sec, diskUnit)}</span
						>
						<span class="hidden sm:inline"
							>R: {formatRate(latestMetric.disk_read_bytes_per_sec, diskUnit)} / W:
							{formatRate(latestMetric.disk_write_bytes_per_sec, diskUnit)}</span
						>
					</span>
				{/if}
			</div>
			<DiskIOChart data={metrics} {timeRange} />
		</div>
		<div class="rounded-lg border bg-card p-4">
			<div class="mb-3 flex items-center justify-between">
				<h3 class="text-sm font-medium">Network</h3>
				{#if latestMetric}
					<span class="text-xs text-muted-foreground">
						<span class="sm:hidden"
							>{formatRate(latestMetric.network_rx_bytes_per_sec, networkUnit)}</span
						>
						<span class="hidden sm:inline"
							>↓ {formatRate(latestMetric.network_rx_bytes_per_sec, networkUnit)} / ↑ {formatRate(
								latestMetric.network_tx_bytes_per_sec,
								networkUnit
							)}</span
						>
					</span>
				{/if}
			</div>
			<NetworkChart data={metrics} {timeRange} />
		</div>
		{#if latestMetric && latestMetric.swap_total_bytes > 0}
			<div class="rounded-lg border bg-card p-4">
				<div class="mb-3 flex items-center justify-between">
					<h3 class="text-sm font-medium">Swap Usage</h3>
					<span class="text-xs text-muted-foreground">
						<span class="sm:hidden"
							>{formatPercent(
								latestMetric.swap_total_bytes > 0
									? (latestMetric.swap_used_bytes / latestMetric.swap_total_bytes) * 100
									: 0
							)}</span
						>
						<span class="hidden sm:inline"
							>{formatBytes(latestMetric.swap_used_bytes)} / {formatBytes(
								latestMetric.swap_total_bytes
							)} ({formatPercent(
								latestMetric.swap_total_bytes > 0
									? (latestMetric.swap_used_bytes / latestMetric.swap_total_bytes) * 100
									: 0
							)})</span
						>
					</span>
				</div>
				<SwapChart data={metrics} {timeRange} />
			</div>
		{/if}
		{#if latestMetric && (latestMetric.cpu_temperature_celsius > 0 || (latestMetric.sensor_readings && latestMetric.sensor_readings.length > 0))}
			<div class="rounded-lg border bg-card p-4">
				<div class="mb-3 flex items-center justify-between">
					<h3 class="text-sm font-medium">CPU Temperature</h3>
					<span class="text-xs text-muted-foreground">
						{formatTemperature(latestMetric.cpu_temperature_celsius, tempUnit)}
					</span>
				</div>
				<TemperatureChart data={metrics} {hostId} {timeRange} />
			</div>
		{/if}
	</div>
</div>

{#if hasContainerData}
	<div class="mb-6">
		<div class="grid gap-4 2xl:grid-cols-2">
			<div class="rounded-lg border bg-card p-4">
				<div class="mb-3">
					<h3 class="text-sm font-medium">Container CPU</h3>
				</div>
				<ContainerCPUChart pivotedData={containerPivots.cpu} {containerNames} {timeRange} />
			</div>
			<div class="rounded-lg border bg-card p-4">
				<div class="mb-3">
					<h3 class="text-sm font-medium">Container Memory</h3>
				</div>
				<ContainerMemoryChart pivotedData={containerPivots.memory} {containerNames} {timeRange} />
			</div>
			<div class="rounded-lg border bg-card p-4 2xl:col-span-2">
				<div class="mb-3">
					<h3 class="text-sm font-medium">Container Network</h3>
				</div>
				<ContainerNetworkChart
					pivotedData={containerPivots.network}
					seriesKeys={containerPivots.networkKeys}
					{timeRange}
				/>
			</div>
		</div>
		<div class="mt-4">
			<ContainersTable {containerMetrics} />
		</div>
	</div>
{/if}
