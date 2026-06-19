<script lang="ts">
	import { X, XCircle, AlertTriangle, RefreshCw } from 'lucide-svelte';
	import { alertsStore } from '$lib/stores';
	import type { ActiveIncident, AlertMetricType } from '$lib/types';
	import { ALERT_METRIC_LABELS } from '$lib/types';
	import { formatRelativeTime } from '$lib/utils';
	import RightSidebar from '$lib/components/RightSidebar.svelte';

	const {
		open,
		onClose
	}: {
		open: boolean;
		onClose: () => void;
	} = $props();

	let refreshing = $state(false);

	$effect(() => {
		if (open) {
			alertsStore.loadIncidents();
		}
	});

	async function handleRefresh() {
		refreshing = true;
		await alertsStore.loadIncidents();
		refreshing = false;
	}

	const incidents = $derived($alertsStore.activeIncidents);

	function isCritical(incident: ActiveIncident): boolean {
		return incident.metric_type === 'host_down';
	}

	function formatMessage(incident: ActiveIncident): string {
		const { metric_type, current_value, threshold_value } = incident;
		switch (metric_type as AlertMetricType) {
			case 'host_down':
				return 'Host is offline';
			case 'cpu_usage':
				return `CPU: ${current_value.toFixed(1)}% (threshold: ${threshold_value.toFixed(0)}%)`;
			case 'memory_usage':
				return `Memory: ${current_value.toFixed(1)}% (threshold: ${threshold_value.toFixed(0)}%)`;
			case 'disk_usage':
				return `Disk: ${current_value.toFixed(1)}% (threshold: ${threshold_value.toFixed(0)}%)`;
			case 'load_avg':
				return `Load avg (1m): ${current_value.toFixed(2)} (threshold: ${threshold_value.toFixed(2)})`;
			case 'load_avg_5':
				return `Load avg (5m): ${current_value.toFixed(2)} (threshold: ${threshold_value.toFixed(2)})`;
			case 'load_avg_15':
				return `Load avg (15m): ${current_value.toFixed(2)} (threshold: ${threshold_value.toFixed(2)})`;
			case 'temperature':
				return `Temperature: ${current_value.toFixed(1)}°C (threshold: ${threshold_value.toFixed(0)}°C)`;
		}
	}
</script>

<RightSidebar {open} {onClose}>
	<!-- Header -->
	<div class="flex items-center justify-between border-b px-6 py-4 shrink-0">
		<h2 class="text-base font-semibold text-foreground">Active Alerts</h2>
		<div class="flex items-center gap-2">
			{#if incidents.length > 0}
				<span
					class="flex h-5 w-5 items-center justify-center rounded-full bg-destructive text-xs font-medium text-primary-foreground"
				>
					{incidents.length}
				</span>
			{/if}
			<button
				type="button"
				onclick={handleRefresh}
				disabled={refreshing}
				class="flex h-8 w-8 items-center justify-center rounded-lg text-muted-foreground hover:bg-muted hover:text-foreground transition-colors disabled:opacity-40"
				aria-label="Refresh alerts"
			>
				<RefreshCw class="h-3.5 w-3.5 {refreshing ? 'animate-spin' : ''}" />
			</button>
			<button
				type="button"
				onclick={onClose}
				class="flex h-8 w-8 items-center justify-center rounded-lg text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
				aria-label="Close alerts"
			>
				<X class="h-4 w-4" />
			</button>
		</div>
	</div>

	<!-- Content -->
	<div class="flex-1 overflow-y-auto p-6">
		{#if incidents.length === 0}
			<div class="rounded-lg border border-dashed bg-muted/20 p-6 text-center">
				<svg
					class="mx-auto h-8 w-8 text-muted-foreground/50 mb-2"
					fill="none"
					stroke="currentColor"
					viewBox="0 0 24 24"
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-width="1.5"
						d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
					/>
				</svg>
				<p class="text-xs text-muted-foreground">No active alerts</p>
			</div>
		{:else}
			<div class="space-y-2">
				{#each incidents as incident (incident.id)}
					<div
						class="rounded-lg border p-3 {isCritical(incident)
							? 'bg-destructive/10 text-destructive border-destructive/20'
							: 'bg-warning/10 text-warning border-warning/20'}"
					>
						<div class="flex items-start gap-2">
							<div class="mt-0.5 shrink-0">
								{#if isCritical(incident)}
									<XCircle class="h-4 w-4" />
								{:else}
									<AlertTriangle class="h-4 w-4" />
								{/if}
							</div>
							<div class="flex-1 min-w-0">
								<p class="text-xs font-medium mb-0.5">{incident.host_name}</p>
								<p class="text-xs opacity-90">{formatMessage(incident)}</p>
								<p class="text-xs opacity-60 mt-1">
									{formatRelativeTime(incident.started_at)}
								</p>
							</div>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</RightSidebar>
