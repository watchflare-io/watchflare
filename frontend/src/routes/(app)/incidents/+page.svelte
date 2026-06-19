<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import * as api from '$lib/api';
	import type { GlobalIncident, IncidentStatusFilter } from '$lib/types';
	import { ALERT_METRIC_LABELS } from '$lib/types';
	import {
		incidentState,
		BADGE_CLASSES,
		BADGE_LABELS,
		type IncidentState
	} from '$lib/incident-state';
	import { formatDateTime, formatRelativeTime } from '$lib/utils';
	import { userStore } from '$lib/stores/user';
	import Pagination from '$lib/components/Pagination.svelte';

	const PAGE_SIZE_OPTIONS = [20, 50, 100];
	const DEFAULT_PAGE_SIZE = 20;

	const timeFormat = $derived(($userStore.user?.time_format ?? '24h') as '12h' | '24h');

	let incidents: GlobalIncident[] = $state([]);
	let totalCount = $state(0);
	let initialLoading = $state(true);
	let tableLoading = $state(false);
	let statusFilter: IncidentStatusFilter = $state('all');
	let limit = $state(DEFAULT_PAGE_SIZE);
	let offset = $state(0);

	const currentPage = $derived(Math.floor(offset / limit) + 1);
	const totalPages = $derived(Math.ceil(totalCount / limit) || 1);

	async function loadData(isInitial = false) {
		if (isInitial) {
			initialLoading = true;
		} else {
			tableLoading = true;
		}
		try {
			const data = await api.getAllIncidents({
				status: statusFilter,
				limit,
				offset
			});
			incidents = data.incidents;
			totalCount = data.total_count;
		} catch (err) {
			console.warn('failed to load incidents:', err);
		} finally {
			initialLoading = false;
			tableLoading = false;
		}
	}

	function updateURL() {
		const params = new URLSearchParams();
		if (statusFilter !== 'all') params.set('status', statusFilter);
		if (limit !== DEFAULT_PAGE_SIZE) params.set('limit', String(limit));
		if (offset > 0) params.set('offset', String(offset));
		const qs = params.toString();
		history.replaceState(history.state, '', qs ? `?${qs}` : location.pathname);
	}

	function handleFilterChange(filter: IncidentStatusFilter) {
		statusFilter = filter;
		offset = 0;
		updateURL();
		loadData();
	}

	function handlePageChange(newPage: number) {
		offset = (newPage - 1) * limit;
		updateURL();
		loadData();
	}

	function handlePageSizeChange(size: number) {
		limit = size;
		offset = 0;
		updateURL();
		loadData();
	}

	onMount(() => {
		const params = $page.url.searchParams;
		const urlStatus = params.get('status');
		if (urlStatus === 'active' || urlStatus === 'paused' || urlStatus === 'resolved') {
			statusFilter = urlStatus;
		}
		const urlOffset = Number(params.get('offset'));
		offset = isNaN(urlOffset) ? 0 : urlOffset;
		const urlLimit = Number(params.get('limit'));
		if (!isNaN(urlLimit) && urlLimit > 0) limit = urlLimit;
		loadData(true);
	});

	function incidentDuration(incident: GlobalIncident): string {
		const start = new Date(incident.started_at).getTime();
		const end = incident.resolved_at ? new Date(incident.resolved_at).getTime() : Date.now();
		const secs = Math.floor((end - start) / 1000);
		if (secs < 60) return `${secs}s`;
		if (secs < 3600) return `${Math.floor(secs / 60)}m ${secs % 60}s`;
		const h = Math.floor(secs / 3600);
		const m = Math.floor((secs % 3600) / 60);
		return m > 0 ? `${h}h ${m}m` : `${h}h`;
	}

	function formatIncidentValue(incident: GlobalIncident): string {
		const { metric_type, current_value, threshold_value } = incident;
		if (metric_type === 'host_down') return '—';
		const isPercent = ['cpu_usage', 'memory_usage', 'disk_usage'].includes(metric_type);
		const isLoad = metric_type.startsWith('load_avg');
		const isTemp = metric_type === 'temperature';
		if (isPercent) return `${current_value.toFixed(1)}% / ${threshold_value.toFixed(0)}%`;
		if (isLoad) return `${current_value.toFixed(2)} / ${threshold_value.toFixed(2)}`;
		if (isTemp) return `${current_value.toFixed(1)}°C / ${threshold_value.toFixed(0)}°C`;
		return `${current_value.toFixed(2)} / ${threshold_value.toFixed(2)}`;
	}
</script>

<svelte:head>
	<title>Incidents - Watchflare</title>
</svelte:head>

<!-- Page header -->
<div class="mb-6 flex items-center justify-between">
	<div>
		<h1 class="text-xl font-semibold text-foreground">Incidents</h1>
		<p class="text-sm text-muted-foreground mt-0.5">Alert history across all hosts</p>
	</div>
	<div class="flex items-center gap-1">
		{#each ['all', 'active', 'paused', 'resolved'] as IncidentStatusFilter[] as filter}
			<button
				type="button"
				onclick={() => handleFilterChange(filter)}
				class="rounded-full px-3.5 py-1.5 text-sm font-medium capitalize transition-colors {statusFilter ===
				filter
					? 'bg-primary text-primary-foreground'
					: 'bg-muted text-muted-foreground hover:text-foreground'}"
			>
				{filter}
			</button>
		{/each}
	</div>
</div>

<!-- Table -->
<div class="rounded-xl border bg-card overflow-hidden mb-2">
	{#if initialLoading}
		<div class="animate-pulse">
			<!-- Desktop skeleton -->
			<div class="hidden sm:block">
				<div class="border-b bg-table-header px-4 py-2.5 flex gap-4">
					{#each [8, 20, 18, 16, 14, 10, 14] as w}
						<div class="h-4 rounded bg-muted" style="width:{w}%"></div>
					{/each}
				</div>
				{#each Array(8) as _}
					<div class="border-b px-4 py-3 flex gap-4 items-center">
						<div class="h-5 w-16 rounded-full bg-muted"></div>
						<div class="h-4 w-28 rounded bg-muted"></div>
						<div class="h-4 w-24 rounded bg-muted"></div>
						<div class="h-4 w-20 rounded bg-muted"></div>
						<div class="h-4 w-28 rounded bg-muted"></div>
						<div class="h-4 w-12 rounded bg-muted"></div>
						<div class="h-4 w-28 rounded bg-muted"></div>
					</div>
				{/each}
			</div>
			<!-- Mobile skeleton -->
			<div class="sm:hidden p-3 flex flex-col gap-2">
				{#each Array(5) as _}
					<div class="rounded-lg border bg-card">
						<div
							class="rounded-t-lg bg-table-header px-4 py-2.5 border-b flex items-center justify-between"
						>
							<div class="h-4 w-28 rounded bg-muted"></div>
							<div class="h-5 w-16 rounded-full bg-muted"></div>
						</div>
						<div class="px-4 py-2.5 flex flex-col gap-1.5">
							<div class="h-3 w-32 rounded bg-muted"></div>
							<div class="h-3 w-20 rounded bg-muted"></div>
						</div>
					</div>
				{/each}
			</div>
		</div>
	{:else}
		<!-- Mobile: cards -->
		<div
			class="sm:hidden p-3 flex flex-col gap-2 transition-opacity {tableLoading
				? 'opacity-50 pointer-events-none'
				: ''}"
		>
			{#each incidents as incident (incident.id)}
				{@const state = incidentState(incident)}
				<div class="rounded-lg border bg-card">
					<div
						class="rounded-t-lg bg-table-header px-4 py-2.5 border-b border-border flex items-center justify-between gap-2"
					>
						<a
							href="/hosts/{incident.host_id}"
							class="text-sm font-medium text-foreground hover:text-primary transition-colors truncate"
						>
							{incident.host_name}
						</a>
						<span
							class="shrink-0 inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium {BADGE_CLASSES[
								state
							]}">{BADGE_LABELS[state]}</span
						>
					</div>
					<div class="px-4 py-2.5 flex flex-col gap-1">
						<p class="text-sm text-foreground font-medium">
							{ALERT_METRIC_LABELS[incident.metric_type] ?? incident.metric_type}
						</p>
						<p class="text-xs text-muted-foreground">
							<span title={formatDateTime(incident.started_at, timeFormat)}
								>{formatRelativeTime(incident.started_at)}</span
							>
							{#if incident.metric_type !== 'host_down'}
								· <span class="font-mono">{formatIncidentValue(incident)}</span>
							{/if}
						</p>
						<p class="text-xs text-muted-foreground">
							Duration: {incidentDuration(incident)}
						</p>
						{#if incident.resolved_at}
							<p class="text-xs text-muted-foreground">
								Resolved: {formatDateTime(incident.resolved_at, timeFormat)}
							</p>
						{/if}
					</div>
				</div>
			{:else}
				<div class="py-16 text-center">
					<p class="text-sm text-muted-foreground">No incidents found</p>
				</div>
			{/each}
		</div>

		<!-- Desktop: table -->
		<div class="hidden sm:block overflow-auto max-h-[65vh]">
			<table class="w-full min-w-200">
				<thead>
					<tr
						class="bg-table-header sticky top-0 z-10 [box-shadow:0_1px_0_var(--border)] whitespace-nowrap"
					>
						<th
							scope="col"
							class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">Status</th
						>
						<th
							scope="col"
							class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">Host</th
						>
						<th
							scope="col"
							class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">Metric</th
						>
						<th
							scope="col"
							class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
							>Value / Threshold</th
						>
						<th
							scope="col"
							class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">Started</th
						>
						<th
							scope="col"
							class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">Duration</th
						>
						<th
							scope="col"
							class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">Resolved</th
						>
					</tr>
				</thead>
				<tbody
					class="divide-y transition-opacity {tableLoading ? 'opacity-50 pointer-events-none' : ''}"
				>
					{#each incidents as incident (incident.id)}
						{@const state = incidentState(incident)}
						<tr class="hover:bg-muted/20 transition-colors whitespace-nowrap">
							<td class="px-4 py-3">
								<span
									class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium {BADGE_CLASSES[
										state
									]}">{BADGE_LABELS[state]}</span
								>
							</td>
							<td class="px-4 py-3">
								<a
									href="/hosts/{incident.host_id}"
									class="text-sm font-medium text-foreground hover:text-primary transition-colors"
								>
									{incident.host_name}
								</a>
							</td>
							<td class="px-4 py-3 text-sm text-foreground">
								{ALERT_METRIC_LABELS[incident.metric_type] ?? incident.metric_type}
							</td>
							<td class="px-4 py-3 text-sm font-mono text-muted-foreground">
								{formatIncidentValue(incident)}
							</td>
							<td
								class="px-4 py-3 text-sm text-muted-foreground"
								title={formatDateTime(incident.started_at, timeFormat)}
							>
								{formatRelativeTime(incident.started_at)}
							</td>
							<td class="px-4 py-3 text-sm text-muted-foreground">
								{incidentDuration(incident)}
							</td>
							<td class="px-4 py-3 text-sm text-muted-foreground">
								{incident.resolved_at ? formatDateTime(incident.resolved_at, timeFormat) : '—'}
							</td>
						</tr>
					{:else}
						<tr>
							<td colspan="7" class="py-16 text-center">
								<p class="text-sm text-muted-foreground">No incidents found</p>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}

	<Pagination
		{currentPage}
		{totalPages}
		totalItems={totalCount}
		pageSize={limit}
		itemLabel="incidents"
		onPageChange={handlePageChange}
		onPageSizeChange={handlePageSizeChange}
		pageSizeOptions={PAGE_SIZE_OPTIONS}
	/>
</div>
