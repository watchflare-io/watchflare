<script lang="ts">
	import CPUChart from '$lib/components/CPUChart.svelte';
	import MemoryChart from '$lib/components/MemoryChart.svelte';
	import DashboardDiskChart from '$lib/components/DashboardDiskChart.svelte';
	import LoadChart from '$lib/components/LoadChart.svelte';
	import { formatBytes, formatPercent } from '$lib/utils';
	import type { AggregatedMetric, TimeRange } from '$lib/types';

	interface Stats {
		avgCPU: number;
		avgDisk: number;
		usedMemory: number;
		totalMemory: number;
		loadAvg: number;
		loadAvg5: number;
		loadAvg15: number;
	}

	const {
		aggregatedMetrics,
		stats,
		timeRange
	}: {
		aggregatedMetrics: AggregatedMetric[];
		stats: Stats;
		timeRange: TimeRange;
	} = $props();

	const memoryPct = $derived(
		stats.totalMemory > 0 ? (stats.usedMemory / stats.totalMemory) * 100 : 0
	);
</script>

<!-- display:contents lets each card become a direct grid item of the parent bento grid -->
<div class="contents">
	<!-- CPU Chart -->
	<div class="rounded-lg border bg-card p-4">
		<div class="mb-3 flex items-center justify-between">
			<h3 class="text-sm font-medium">CPU Usage</h3>
			<span class="text-xs text-muted-foreground tabular-nums">{formatPercent(stats.avgCPU)}</span>
		</div>
		<CPUChart data={aggregatedMetrics} {timeRange} />
	</div>

	<!-- Memory Chart -->
	<div class="rounded-lg border bg-card p-4">
		<div class="mb-3 flex items-center justify-between">
			<h3 class="text-sm font-medium">Memory Usage</h3>
			<span class="text-xs text-muted-foreground tabular-nums">
				<span class="sm:hidden">{formatPercent(memoryPct)}</span>
				<span class="hidden sm:inline"
					>{formatBytes(stats.usedMemory)} / {formatBytes(stats.totalMemory)}</span
				>
			</span>
		</div>
		<MemoryChart data={aggregatedMetrics} {timeRange} />
	</div>

	<!-- Disk Chart -->
	<div class="rounded-lg border bg-card p-4">
		<div class="mb-3 flex items-center justify-between">
			<h3 class="text-sm font-medium">Disk Usage</h3>
			<span class="text-xs text-muted-foreground tabular-nums">{formatPercent(stats.avgDisk)}</span>
		</div>
		<DashboardDiskChart data={aggregatedMetrics} {timeRange} />
	</div>

	<!-- Load Average Chart -->
	<div class="rounded-lg border bg-card p-4">
		<div class="mb-3 flex items-center justify-between">
			<h3 class="text-sm font-medium">Load Average</h3>
			<span class="text-xs text-muted-foreground tabular-nums"
				>{stats.loadAvg.toFixed(2)} · {stats.loadAvg5.toFixed(2)} · {stats.loadAvg15.toFixed(
					2
				)}</span
			>
		</div>
		<LoadChart data={aggregatedMetrics} {timeRange} />
	</div>
</div>
