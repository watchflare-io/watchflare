<script lang="ts">
	import { formatPercent, formatUptime } from '$lib/utils';
	import { formatRate } from '$lib/chart-utils';
	import { userStore } from '$lib/stores/user';
	import { Cpu, MemoryStick, HardDrive, Network, Clock } from 'lucide-svelte';
	import type { Metric } from '$lib/types';

	const { metric }: { metric: Metric | null } = $props();

	const memoryPercent = $derived(
		metric && metric.memory_total_bytes > 0
			? (metric.memory_used_bytes / metric.memory_total_bytes) * 100
			: 0
	);

	const diskPercent = $derived(
		metric && metric.disk_total_bytes > 0
			? (metric.disk_used_bytes / metric.disk_total_bytes) * 100
			: 0
	);

	const networkUnit = $derived($userStore.user?.network_unit ?? 'bytes');
</script>

{#if metric}
	<div class="no-scrollbar mb-6 flex gap-3 overflow-x-auto">
		<div class="flex shrink-0 items-center gap-2 rounded-lg border bg-card px-3 py-2">
			<Cpu class="h-3.5 w-3.5 text-muted-foreground" />
			<span class="text-xs text-muted-foreground">CPU</span>
			<span class="text-sm font-medium text-foreground">{metric.cpu_usage_percent.toFixed(1)}%</span
			>
		</div>
		<div class="flex shrink-0 items-center gap-2 rounded-lg border bg-card px-3 py-2">
			<MemoryStick class="h-3.5 w-3.5 text-muted-foreground" />
			<span class="text-xs text-muted-foreground">Memory</span>
			<span class="text-sm font-medium text-foreground">{formatPercent(memoryPercent)}</span>
		</div>
		<div class="flex shrink-0 items-center gap-2 rounded-lg border bg-card px-3 py-2">
			<HardDrive class="h-3.5 w-3.5 text-muted-foreground" />
			<span class="text-xs text-muted-foreground">Disk</span>
			<span class="text-sm font-medium text-foreground">{formatPercent(diskPercent)}</span>
		</div>
		<div class="flex shrink-0 items-center gap-2 rounded-lg border bg-card px-3 py-2">
			<Network class="h-3.5 w-3.5 text-muted-foreground" />
			<span class="text-xs text-muted-foreground">Net</span>
			<span class="text-sm font-medium text-foreground"
				>↓ {formatRate(metric.network_rx_bytes_per_sec, networkUnit)} ↑ {formatRate(
					metric.network_tx_bytes_per_sec,
					networkUnit
				)}</span
			>
		</div>
		<div class="flex shrink-0 items-center gap-2 rounded-lg border bg-card px-3 py-2">
			<Clock class="h-3.5 w-3.5 text-muted-foreground" />
			<span class="text-xs text-muted-foreground">Uptime</span>
			<span class="text-sm font-medium text-foreground">{formatUptime(metric.uptime_seconds)}</span>
		</div>
	</div>
{/if}
