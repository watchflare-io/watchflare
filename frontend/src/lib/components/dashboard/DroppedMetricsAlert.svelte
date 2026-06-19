<script lang="ts">
	import { AlertTriangle } from 'lucide-svelte';
	import type { DroppedMetric } from '$lib/types';

	const { alerts }: { alerts: DroppedMetric[] } = $props();

	function formatDuration(nanoseconds: number): string {
		const seconds = nanoseconds / 1_000_000_000;
		const hours = Math.floor(seconds / 3600);
		const minutes = Math.floor((seconds % 3600) / 60);

		if (hours > 0) {
			return `${hours}h${minutes > 0 ? ` ${minutes}min` : ''}`;
		} else if (minutes > 0) {
			return `${minutes}min`;
		} else {
			return `${Math.floor(seconds)}s`;
		}
	}
</script>

{#if alerts.length > 0}
	<div role="alert" class="mb-6 rounded-lg border border-warning bg-warning/5 p-4">
		<h3 class="mb-3 flex items-center gap-2 text-sm font-semibold text-warning">
			<AlertTriangle class="h-4 w-4 shrink-0" />
			Dropped Metrics
		</h3>
		{#each alerts as alert}
			<div class="mb-2 last:mb-0 rounded-md bg-background p-3">
				<p class="text-sm font-medium">
					<strong>{alert.hostname}</strong> dropped
					<strong>{alert.total_dropped} metrics</strong>
				</p>
				<p class="text-xs text-muted-foreground mt-1">
					Backend unavailable for {formatDuration(alert.downtime_duration)}
					({new Date(alert.first_dropped_at).toLocaleString()} → {new Date(
						alert.last_dropped_at
					).toLocaleString()})
				</p>
			</div>
		{/each}
	</div>
{/if}
