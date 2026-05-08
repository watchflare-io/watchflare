<script lang="ts">
	import CPUChart from '$lib/components/CPUChart.svelte';
	import MemoryChart from '$lib/components/MemoryChart.svelte';
	import { formatBytes, formatPercent } from '$lib/utils';
	import type { AggregatedMetric, TimeRange } from '$lib/types';

	interface Stats {
		avgCPU: number;
		usedMemory: number;
		totalMemory: number;
	}

	const {
		aggregatedMetrics,
		stats,
		timeRange,
	}: {
		aggregatedMetrics: AggregatedMetric[];
		stats: Stats;
		timeRange: TimeRange;
	} = $props();
</script>

<!-- display:contents lets each card become a direct grid item of the parent bento grid -->
<div class="contents">
	<!-- CPU Chart — 4×2 -->
	<div class="rounded-lg border bg-card p-4">
		<div class="mb-3 flex items-center justify-between">
			<h3 class="text-sm font-medium">CPU Usage</h3>
			<span class="text-xs text-muted-foreground tabular-nums">{formatPercent(stats.avgCPU)}</span>
		</div>
		<CPUChart data={aggregatedMetrics} {timeRange} />
	</div>

	<!-- Memory Chart — 4×2 -->
	<div class="rounded-lg border bg-card p-4">
		<div class="mb-3 flex items-center justify-between">
			<h3 class="text-sm font-medium">Memory Usage</h3>
			<span class="text-xs text-muted-foreground tabular-nums">
				<span class="sm:hidden">{formatPercent(stats.totalMemory > 0 ? (stats.usedMemory / stats.totalMemory) * 100 : 0)}</span>
				<span class="hidden sm:inline">{formatBytes(stats.usedMemory)} / {formatBytes(stats.totalMemory)}</span>
			</span>
		</div>
		<MemoryChart data={aggregatedMetrics} {timeRange} />
	</div>
</div>
