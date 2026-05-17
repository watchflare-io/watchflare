<script lang="ts">
    import { onMount, getContext } from "svelte";
    import * as api from "$lib/api";
    import type { HostIncident, IncidentStatusFilter } from "$lib/types";
    import { ALERT_METRIC_LABELS } from "$lib/types";
    import { formatDateTime, formatRelativeTime } from "$lib/utils";
    import { userStore } from "$lib/stores/user";

    const { hostId }: { hostId: string } = $props();

    const timeFormat = $derived(
        ($userStore.user?.time_format ?? "24h") as "12h" | "24h",
    );

    const LIMIT = 20;

    type IncidentsCache = {
        incidents: HostIncident[];
        totalCount: number;
        offset: number;
        statusFilter: IncidentStatusFilter;
    };
    const ctx = getContext<{
        incidentsCache: IncidentsCache | null;
        setIncidentsCache: (data: IncidentsCache) => void;
    }>("hostDetail");

    const cached = ctx?.incidentsCache;
    let incidents: HostIncident[] = $state(cached?.incidents ?? []);
    let totalCount = $state(cached?.totalCount ?? 0);
    let offset = $state(cached?.offset ?? 0);
    let loading = $state(!cached);
    let tableLoading = $state(false);
    let loadingMore = $state(false);
    let statusFilter: IncidentStatusFilter = $state(
        cached?.statusFilter ?? "all",
    );

    onMount(() => {
        loadIncidents(true, !!cached);
    });

    function saveToCache() {
        ctx?.setIncidentsCache({ incidents, totalCount, offset, statusFilter });
    }

    async function loadIncidents(reset = false, silent = false) {
        if (reset) {
            offset = 0;
            if (loading) {
                // initial load — keep full loading state
            } else if (!silent) {
                tableLoading = true;
            }
        } else {
            loadingMore = true;
        }
        try {
            const data = await api.getHostIncidents(hostId, {
                status: statusFilter,
                limit: LIMIT,
                offset: reset ? 0 : offset,
            });
            if (reset) {
                incidents = data.incidents;
            } else {
                incidents = [...incidents, ...data.incidents];
            }
            totalCount = data.total_count;
            offset = incidents.length;
        } catch (err) {
            console.warn("failed to load incidents:", err);
        } finally {
            loading = false;
            tableLoading = false;
            loadingMore = false;
            saveToCache();
        }
    }

    function handleFilterChange(filter: IncidentStatusFilter) {
        statusFilter = filter;
        loadIncidents(true);
    }

    function incidentDuration(incident: HostIncident): string {
        const start = new Date(incident.started_at).getTime();
        const end = incident.resolved_at
            ? new Date(incident.resolved_at).getTime()
            : Date.now();
        const secs = Math.floor((end - start) / 1000);
        if (secs < 60) return `${secs}s`;
        if (secs < 3600) return `${Math.floor(secs / 60)}m ${secs % 60}s`;
        const h = Math.floor(secs / 3600);
        const m = Math.floor((secs % 3600) / 60);
        return m > 0 ? `${h}h ${m}m` : `${h}h`;
    }

    function formatIncidentValue(incident: HostIncident): string {
        const { metric_type, current_value, threshold_value } = incident;
        if (metric_type === "host_down") return "—";
        const isPercent = ["cpu_usage", "memory_usage", "disk_usage"].includes(
            metric_type,
        );
        const isLoad = metric_type.startsWith("load_avg");
        const isTemp = metric_type === "temperature";
        if (isPercent)
            return `${current_value.toFixed(1)}% / ${threshold_value.toFixed(0)}%`;
        if (isLoad)
            return `${current_value.toFixed(2)} / ${threshold_value.toFixed(2)}`;
        if (isTemp)
            return `${current_value.toFixed(1)}°C / ${threshold_value.toFixed(0)}°C`;
        return `${current_value.toFixed(2)} / ${threshold_value.toFixed(2)}`;
    }
</script>

<div class="mb-6">
    <div class="mb-4 flex items-center justify-end">
        <div class="flex rounded-lg border bg-card p-0.5">
            {#each ["all", "active", "resolved"] as IncidentStatusFilter[] as filter}
                <button
                    type="button"
                    onclick={() => handleFilterChange(filter)}
                    class="rounded-md px-3 py-1.5 text-sm font-medium capitalize transition-colors {statusFilter ===
                    filter
                        ? 'bg-background text-foreground shadow-sm'
                        : 'text-muted-foreground hover:text-foreground'}"
                >
                    {filter}
                </button>
            {/each}
        </div>
    </div>

    <div class="rounded-xl border bg-card overflow-hidden">
        {#if loading}
            <!-- Mobile skeleton -->
            <div class="sm:hidden p-3 flex flex-col gap-2 animate-pulse">
                {#each Array(3) as _}
                    <div class="rounded-lg border bg-card">
                        <div class="rounded-t-lg bg-table-header px-4 py-2.5 border-b flex items-center justify-between">
                            <div class="h-4 w-28 rounded bg-muted"></div>
                            <div class="h-5 w-16 rounded-full bg-muted"></div>
                        </div>
                        <div class="px-4 py-2.5 flex flex-col gap-2">
                            <div class="h-3 w-40 rounded bg-muted"></div>
                            <div class="h-3 w-24 rounded bg-muted"></div>
                        </div>
                    </div>
                {/each}
            </div>
            <!-- Desktop skeleton -->
            <div class="hidden sm:block animate-pulse">
                <div class="border-b bg-table-header px-4 py-2.5 flex gap-8">
                    <div class="h-4 w-16 rounded bg-muted"></div>
                    <div class="h-4 w-20 rounded bg-muted"></div>
                    <div class="h-4 w-24 rounded bg-muted"></div>
                    <div class="h-4 w-20 rounded bg-muted"></div>
                    <div class="h-4 w-16 rounded bg-muted"></div>
                    <div class="h-4 w-20 rounded bg-muted"></div>
                </div>
                {#each Array(3) as _}
                    <div class="border-b px-4 py-3 flex gap-8">
                        <div class="h-4 w-16 rounded bg-muted"></div>
                        <div class="h-4 w-32 rounded bg-muted"></div>
                        <div class="h-4 w-28 rounded bg-muted"></div>
                        <div class="h-4 w-24 rounded bg-muted"></div>
                        <div class="h-4 w-20 rounded bg-muted"></div>
                        <div class="h-4 w-24 rounded bg-muted"></div>
                    </div>
                {/each}
            </div>
        {:else}
            <!-- Mobile: cards -->
            <div
                class="sm:hidden p-3 flex flex-col gap-2 transition-opacity {tableLoading
                    ? 'opacity-50 pointer-events-none'
                    : ''}"
            >
                {#each incidents as incident (incident.id)}
                    <div class="rounded-lg border bg-card">
                        <div
                            class="rounded-t-lg bg-table-header px-4 py-2.5 border-b border-border flex items-center justify-between gap-2"
                        >
                            <span class="text-sm font-medium text-foreground">
                                {ALERT_METRIC_LABELS[incident.metric_type] ??
                                    incident.metric_type}
                            </span>
                            {#if incident.resolved_at}
                                <span
                                    class="shrink-0 inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-success/10 text-success border border-success/20"
                                    >Resolved</span
                                >
                            {:else}
                                <span
                                    class="shrink-0 inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-destructive/10 text-destructive border border-destructive/20"
                                    >Active</span
                                >
                            {/if}
                        </div>
                        <div class="px-4 py-2.5 flex flex-col gap-1">
                            <p class="text-xs text-muted-foreground">
                                <span
                                    title={formatDateTime(
                                        incident.started_at,
                                        timeFormat,
                                    )}
                                    >{formatRelativeTime(
                                        incident.started_at,
                                    )}</span
                                >
                                {#if incident.metric_type !== "host_down"}
                                    · <span class="font-mono"
                                        >{formatIncidentValue(incident)}</span
                                    >
                                {/if}
                            </p>
                            <p class="text-xs text-muted-foreground">
                                Duration: {incidentDuration(incident)}
                            </p>
                            {#if incident.resolved_at}
                                <p class="text-xs text-muted-foreground">
                                    Resolved: {formatDateTime(
                                        incident.resolved_at,
                                        timeFormat,
                                    )}
                                </p>
                            {/if}
                        </div>
                    </div>
                {:else}
                    <div class="rounded-lg border bg-card py-16 text-center">
                        <p class="text-sm text-muted-foreground">
                            No incidents found
                        </p>
                    </div>
                {/each}
                {#if incidents.length < totalCount}
                    <button
                        type="button"
                        onclick={() => loadIncidents(false)}
                        disabled={loadingMore}
                        class="rounded-lg border bg-card w-full py-3 text-xs font-medium text-primary hover:text-primary/80 hover:bg-muted/20 transition-colors disabled:opacity-40"
                    >
                        {loadingMore
                            ? "Loading..."
                            : `Load more (${totalCount - incidents.length} remaining)`}
                    </button>
                {/if}
            </div>

            <!-- Desktop: table -->
            <div class="hidden sm:block overflow-x-auto">
                <table class="w-full min-w-160">
                    <thead>
                        <tr class="border-b bg-table-header whitespace-nowrap">
                            <th
                                class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
                                >Status</th
                            >
                            <th
                                class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
                                >Metric</th
                            >
                            <th
                                class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
                                >Value / Threshold</th
                            >
                            <th
                                class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
                                >Started</th
                            >
                            <th
                                class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
                                >Duration</th
                            >
                            <th
                                class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
                                >Resolved</th
                            >
                        </tr>
                    </thead>
                    <tbody
                        class="divide-y transition-opacity {tableLoading
                            ? 'opacity-50 pointer-events-none'
                            : ''}"
                    >
                        {#each incidents as incident (incident.id)}
                            <tr
                                class="hover:bg-muted/20 transition-colors whitespace-nowrap"
                            >
                                <td class="px-4 py-3">
                                    {#if incident.resolved_at}
                                        <span
                                            class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-success/10 text-success border border-success/20"
                                            >Resolved</span
                                        >
                                    {:else}
                                        <span
                                            class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-destructive/10 text-destructive border border-destructive/20"
                                            >Active</span
                                        >
                                    {/if}
                                </td>
                                <td
                                    class="px-4 py-3 text-sm font-medium text-foreground"
                                >
                                    {ALERT_METRIC_LABELS[
                                        incident.metric_type
                                    ] ?? incident.metric_type}
                                </td>
                                <td
                                    class="px-4 py-3 text-sm font-mono text-muted-foreground"
                                >
                                    {formatIncidentValue(incident)}
                                </td>
                                <td
                                    class="px-4 py-3 text-sm text-muted-foreground"
                                    title={formatDateTime(
                                        incident.started_at,
                                        timeFormat,
                                    )}
                                >
                                    {formatRelativeTime(incident.started_at)}
                                </td>
                                <td
                                    class="px-4 py-3 text-sm text-muted-foreground"
                                >
                                    {incidentDuration(incident)}
                                </td>
                                <td
                                    class="px-4 py-3 text-sm text-muted-foreground"
                                >
                                    {incident.resolved_at
                                        ? formatDateTime(
                                              incident.resolved_at,
                                              timeFormat,
                                          )
                                        : "—"}
                                </td>
                            </tr>
                        {:else}
                            <tr>
                                <td colspan="6" class="py-16 text-center">
                                    <p class="text-sm text-muted-foreground">
                                        No incidents found
                                    </p>
                                </td>
                            </tr>
                        {/each}
                    </tbody>
                </table>
            </div>

            {#if incidents.length < totalCount}
                <button
                    type="button"
                    onclick={() => loadIncidents(false)}
                    disabled={loadingMore}
                    class="hidden sm:block w-full border-t px-4 py-3 text-xs font-medium text-primary hover:text-primary/80 hover:bg-muted/20 transition-colors disabled:opacity-40"
                >
                    {loadingMore
                        ? "Loading..."
                        : `Load more (${totalCount - incidents.length} remaining)`}
                </button>
            {/if}
        {/if}
    </div>
</div>
