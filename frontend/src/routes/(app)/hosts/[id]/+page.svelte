<script lang="ts">
	import { onMount, onDestroy, getContext } from 'svelte';
	import { page } from '$app/stores';
	import * as api from '$lib/api.js';
	import { logger, TIME_RANGES, isSystemContainer } from '$lib/utils';
	import type { Host, Metric, ContainerMetric, SSEEvent, TimeRange } from '$lib/types';
	import HostMetricsCharts from '$lib/components/host/HostMetricsCharts.svelte';

	function rangeSeconds(range: TimeRange): number {
		return TIME_RANGES.find((r) => r.value === range)?.seconds ?? 3600;
	}

	function pruneMetricsByTime(arr: Metric[], range: TimeRange): Metric[] {
		const cutoff = Date.now() / 1000 - rangeSeconds(range) - 60;
		return arr.filter((m) => new Date(m.timestamp).getTime() / 1000 >= cutoff);
	}

	const hostId = $derived($page.params.id);

	type OverviewCache = {
		metrics: Metric[];
		containerMetrics: ContainerMetric[];
		timeRange: TimeRange;
	};
	const ctx = getContext<{
		host: Host | null;
		overviewCache: OverviewCache | null;
		setOverviewCache: (data: OverviewCache) => void;
		setLatestMetric: (m: Metric | null) => void;
		subscribeToSSE: (cb: (event: SSEEvent) => void) => () => void;
	}>('hostDetail');

	const systemContainer = $derived(ctx.host ? isSystemContainer(ctx.host) : false);

	const cached = ctx.overviewCache;
	let metrics: Metric[] = $state(cached?.metrics ?? []);
	let containerMetrics: ContainerMetric[] = $state(cached?.containerMetrics ?? []);
	let timeRange: TimeRange = $state(cached?.timeRange ?? '1h');
	let loading = $state(!cached);
	let metricsError = $state('');
	let metricsLoadId = 0;
	let lastCacheUpdate = 0;
	let sseUnsubscribe: (() => void) | null = null;

	onMount(() => {
		sseUnsubscribe = ctx.subscribeToSSE(handleSSEMessage);
	});

	onDestroy(() => {
		if (sseUnsubscribe) sseUnsubscribe();
	});

	$effect(() => {
		const range = timeRange;
		const id = hostId;
		loadMetrics(range, id);
	});

	function handleSSEMessage(event: SSEEvent) {
		if (event.type === 'metrics_update') {
			const metric = event.data;
			metrics = pruneMetricsByTime([...metrics, metric], timeRange);
			ctx.setLatestMetric(metric);
			const now = Date.now();
			if (now - lastCacheUpdate >= 5000) {
				lastCacheUpdate = now;
				ctx.setOverviewCache({
					metrics,
					containerMetrics,
					timeRange
				});
			}
		}
		if (event.type === 'container_metrics_update') {
			const update = event.data as {
				host_id: string;
				metrics: ContainerMetric[];
			};
			const cutoff = Date.now() / 1000 - rangeSeconds(timeRange) - 60;
			containerMetrics = [...containerMetrics, ...update.metrics].filter(
				(m) => new Date(m.timestamp).getTime() / 1000 >= cutoff
			);
			const now = Date.now();
			if (now - lastCacheUpdate >= 5000) {
				lastCacheUpdate = now;
				ctx.setOverviewCache({
					metrics,
					containerMetrics,
					timeRange
				});
			}
		}
	}

	async function loadMetrics(range: TimeRange, id: string) {
		const thisLoadId = ++metricsLoadId;
		loading = true;
		metricsError = '';
		lastCacheUpdate = 0;
		try {
			const data = await api.getHostMetrics(id, { time_range: range });
			if (thisLoadId !== metricsLoadId) return;
			metrics = data.metrics || [];
			if (metrics.length > 0) ctx.setLatestMetric(metrics[metrics.length - 1]);
		} catch (err) {
			logger.error('Failed to load metrics:', err);
			if (thisLoadId === metricsLoadId) metricsError = 'Failed to load metrics';
		}
		try {
			const containerData = await api.getContainerMetrics(id, range);
			if (thisLoadId !== metricsLoadId) return;
			containerMetrics = containerData.metrics || [];
		} catch (err) {
			logger.error('Failed to load container metrics:', err);
			containerMetrics = [];
		}
		loading = false;
		ctx.setOverviewCache({ metrics, containerMetrics, timeRange: range });
	}
</script>

{#if loading}
	<div class="animate-pulse">
		<!-- TimeRangeSelector placeholder -->
		<div class="mb-4 flex justify-end">
			<div class="h-9 w-full sm:w-48 rounded-lg bg-muted"></div>
		</div>
		<!-- Charts grid 1 -->
		<div class="grid gap-4 2xl:grid-cols-2 mb-4">
			{#each Array(2) as _}
				<div class="rounded-lg border bg-card p-4">
					<div class="mb-3 flex items-center justify-between">
						<div class="h-4 w-24 rounded bg-muted"></div>
						<div class="h-4 w-12 rounded bg-muted"></div>
					</div>
					<div class="h-40 rounded bg-muted"></div>
				</div>
			{/each}
		</div>
		<!-- Charts grid 2 -->
		<div class="grid gap-4 2xl:grid-cols-2">
			{#each Array(2) as _}
				<div class="rounded-lg border bg-card p-4">
					<div class="mb-3 flex items-center justify-between">
						<div class="h-4 w-24 rounded bg-muted"></div>
						<div class="h-4 w-12 rounded bg-muted"></div>
					</div>
					<div class="h-40 rounded bg-muted"></div>
				</div>
			{/each}
		</div>
	</div>
{:else if metricsError}
	<div role="alert" class="flex items-center justify-center py-20">
		<p class="text-sm text-destructive">{metricsError}</p>
	</div>
{:else}
	<HostMetricsCharts
		{hostId}
		{metrics}
		{containerMetrics}
		bind:timeRange
		isSystemContainer={systemContainer}
	/>
{/if}
