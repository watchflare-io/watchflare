<script lang="ts">
	import { formatBytes, formatPercent } from '$lib/utils';

	const {
		stats
	}: {
		stats: {
			onlineHosts: number;
			totalHosts: number;
			avgCPU: number;
			usedMemory: number;
			usedDisk: number;
		};
	} = $props();
</script>

<div class="flex flex-wrap items-center gap-6 rounded-lg border bg-card px-6 py-4">
	<!-- Hosts -->
	<div class="flex items-center gap-3">
		<div class="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10">
			<svg class="h-5 w-5 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					stroke-width="2"
					d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"
				/>
			</svg>
		</div>
		<div>
			<p class="text-sm text-muted-foreground">Hosts</p>
			<p class="text-lg font-semibold">
				{stats.onlineHosts}/{stats.totalHosts}
			</p>
		</div>
	</div>

	<!-- Divider -->
	<div class="h-10 w-px bg-border"></div>

	<!-- CPU -->
	<div class="flex items-center gap-3">
		<div class="flex h-9 w-9 items-center justify-center rounded-lg bg-chart-1/10">
			<svg class="h-5 w-5 text-chart-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					stroke-width="2"
					d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z"
				/>
			</svg>
		</div>
		<div>
			<p class="text-sm text-muted-foreground">Avg CPU</p>
			<p class="text-lg font-semibold">
				{formatPercent(stats.avgCPU)}
			</p>
		</div>
	</div>

	<!-- Divider -->
	<div class="h-10 w-px bg-border"></div>

	<!-- Memory -->
	<div class="flex items-center gap-3">
		<div class="flex h-9 w-9 items-center justify-center rounded-lg bg-chart-2/10">
			<svg class="h-5 w-5 text-chart-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					stroke-width="2"
					d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z"
				/>
			</svg>
		</div>
		<div>
			<p class="text-sm text-muted-foreground">RAM</p>
			<p class="text-lg font-semibold">
				{formatBytes(stats.usedMemory)}
			</p>
		</div>
	</div>

	<!-- Divider -->
	<div class="h-10 w-px bg-border"></div>

	<!-- Disk -->
	<div class="flex items-center gap-3">
		<div class="flex h-9 w-9 items-center justify-center rounded-lg bg-chart-3/10">
			<svg class="h-5 w-5 text-chart-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					stroke-width="2"
					d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4"
				/>
			</svg>
		</div>
		<div>
			<p class="text-sm text-muted-foreground">Disk</p>
			<p class="text-lg font-semibold">
				{formatBytes(stats.usedDisk)}
			</p>
		</div>
	</div>
</div>
