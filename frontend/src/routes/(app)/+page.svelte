<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import { formatPercent, handleSSEReactivation, logger } from "$lib/utils";
    import { DROPPED_METRICS_POLL_INTERVAL } from "$lib/constants";
    import { listAllPackages } from "$lib/api";
    import {
        userStore,
        currentUser,
        hostStatsStore,
        aggregatedStore,
        aggregatedMetrics,
        currentTimeRange,
        dashboardStats,
        alertsStore,
        sseStore,
    } from "$lib/stores";
    import DashboardStats from "$lib/components/dashboard/DashboardStats.svelte";
    import DashboardCharts from "$lib/components/dashboard/DashboardCharts.svelte";
    import DroppedMetricsAlert from "$lib/components/dashboard/DroppedMetricsAlert.svelte";
    import TimeRangeSelector from "$lib/components/TimeRangeSelector.svelte";
    import type { SSEEvent, TimeRange, HostUpdateEvent, AggregatedMetricsUpdateEvent } from "$lib/types";

    let sseUnsubscribe: (() => void) | null = null;

    const TREND_COVERAGE_THRESHOLD = 0.8;
    const TIME_RANGE_MS: Record<TimeRange, number> = {
        "1h":  1 * 60 * 60 * 1000,
        "12h": 12 * 60 * 60 * 1000,
        "24h": 24 * 60 * 60 * 1000,
        "7d":  7 * 24 * 60 * 60 * 1000,
        "30d": 30 * 24 * 60 * 60 * 1000,
    };

    function getTrend(current: number | null, previous: number | null): { direction: "up" | "down" | "stable"; delta: number } {
        if (current === null || previous === null) return { direction: "stable", delta: 0 };
        const delta = current - previous;
        if (delta > 0) return { direction: "up", delta };
        if (delta < 0) return { direction: "down", delta };
        return { direction: "stable", delta };
    }

    let loading = $state(true);
    let packagesStats = $state({ outdatedCount: 0, securityCount: 0, outdatedHostsCount: 0, securityHostsCount: 0 });
    let user = $derived($currentUser);
    let stats = $derived($dashboardStats);
    let droppedAlerts = $derived($alertsStore.droppedMetrics);
    let activeIncidents = $derived($alertsStore.activeIncidents);
    let selectedTimeRange = $derived($currentTimeRange);

    let firstMetric = $derived(
        $aggregatedMetrics.length >= 2 ? $aggregatedMetrics[0] : null
    );
    let lastMetric = $derived(
        $aggregatedMetrics.length >= 2 ? $aggregatedMetrics[$aggregatedMetrics.length - 1] : null
    );

    let hasSufficientTrendData = $derived((() => {
        if (!firstMetric || !lastMetric) return false;
        const actualMs = new Date(lastMetric.timestamp).getTime() - new Date(firstMetric.timestamp).getTime();
        return actualMs >= TIME_RANGE_MS[selectedTimeRange] * TREND_COVERAGE_THRESHOLD;
    })());

    let trends = $derived({
        cpu: getTrend(
            lastMetric?.cpu_usage_percent ?? null,
            firstMetric?.cpu_usage_percent ?? null
        ),
        memory: getTrend(
            lastMetric && lastMetric.memory_total_bytes > 0
                ? (lastMetric.memory_used_bytes / lastMetric.memory_total_bytes) * 100
                : null,
            firstMetric && firstMetric.memory_total_bytes > 0
                ? (firstMetric.memory_used_bytes / firstMetric.memory_total_bytes) * 100
                : null
        ),
        disk: getTrend(
            lastMetric && lastMetric.disk_total_bytes > 0
                ? (lastMetric.disk_used_bytes / lastMetric.disk_total_bytes) * 100
                : null,
            firstMetric && firstMetric.disk_total_bytes > 0
                ? (firstMetric.disk_used_bytes / firstMetric.disk_total_bytes) * 100
                : null
        ),
        loadAvg: getTrend(
            lastMetric?.load_avg_1min ?? null,
            firstMetric?.load_avg_1min ?? null
        ),
    });

    async function loadData() {
        try {
            loading = true;

            if (!$currentUser) {
                await userStore.load();
            }

            const userTimeRange = $currentUser?.default_time_range || "24h";
            const migratedTimeRange: TimeRange =
                (userTimeRange as string) === "6h" ? "12h" : userTimeRange;
            aggregatedStore.setTimeRange(migratedTimeRange);

            const [,,,pkgData] = await Promise.all([
                hostStatsStore.load(),
                alertsStore.load(),
                aggregatedStore.load(migratedTimeRange),
                listAllPackages({ limit: 1 }),
            ]);
            packagesStats = {
                outdatedCount: pkgData.outdated_count,
                securityCount: pkgData.security_count,
                outdatedHostsCount: pkgData.outdated_hosts_count,
                securityHostsCount: pkgData.security_hosts_count,
            };
        } catch (err) {
            logger.error("Failed to load data:", err);
        } finally {
            loading = false;
        }
    }

    async function handleTimeRangeChange(newTimeRange: TimeRange) {
        aggregatedStore.setTimeRange(newTimeRange);
        await aggregatedStore.load(newTimeRange);
    }

    function handleSSEMessage(event: SSEEvent) {
        handleSSEReactivation(event);

        if (event.type === "host_update") {
            const update = event.data as HostUpdateEvent;
            hostStatsStore.applyUpdate(update.id, update.status);
        } else if (event.type === "aggregated_metrics_update") {
            aggregatedStore.addMetricPoint(event.data as AggregatedMetricsUpdateEvent);
        }
    }

    onMount(() => {
        loadData();

        sseUnsubscribe = sseStore.connect(handleSSEMessage);

        const droppedMetricsInterval = setInterval(
            () => alertsStore.load(),
            DROPPED_METRICS_POLL_INTERVAL,
        );

        return () => {
            clearInterval(droppedMetricsInterval);
        };
    });

    onDestroy(() => {
        if (sseUnsubscribe) {
            sseUnsubscribe();
        }
    });
</script>

<svelte:head>
    <title>Dashboard - Watchflare</title>
</svelte:head>

{#if loading}
    <!-- Skeleton: header -->
    <div class="mb-6 animate-pulse">
        <div class="h-7 w-48 rounded bg-muted mb-2"></div>
        <div class="h-4 w-64 rounded bg-muted"></div>
    </div>
    <!-- Skeleton: stat cards -->
    <div class="grid grid-stats gap-5 mb-5 animate-pulse">
        {#each Array(8) as _}
            <div class="flex flex-col gap-2 rounded-lg border bg-card p-4">
                <div class="flex items-center justify-between">
                    <div class="h-4 w-16 rounded bg-muted"></div>
                    <div class="h-8 w-8 rounded-md bg-muted"></div>
                </div>
                <div class="h-8 w-20 rounded bg-muted"></div>
                <div class="h-3 w-10 rounded bg-muted mt-auto"></div>
            </div>
        {/each}
    </div>
    <!-- Skeleton: chart cards -->
    <div class="grid gap-5 mb-8 2xl:grid-cols-2 animate-pulse">
        {#each Array(2) as _}
            <div class="rounded-lg border bg-card p-4">
                <div class="mb-3 flex items-center justify-between">
                    <div class="h-4 w-20 rounded bg-muted"></div>
                    <div class="h-4 w-12 rounded bg-muted"></div>
                </div>
                <div class="h-48 sm:h-64 rounded bg-muted"></div>
            </div>
        {/each}
    </div>
{:else}
    <!-- Header -->
    <div class="mb-6 flex items-start justify-between gap-4">
        <div>
            <h1 class="text-xl sm:text-2xl font-semibold text-foreground">
                Welcome back, <span class="text-primary"
                    >{user?.username || user?.email?.split("@")[0] || "User"}</span
                >
            </h1>
            <p class="text-sm text-muted-foreground mt-1">
                {#if stats.totalHosts === 0}
                    No hosts monitored yet
                {:else}
                    Global uptime at <span class="font-medium text-foreground"
                        >{formatPercent(
                            (stats.onlineHosts / stats.totalHosts) * 100,
                        )}</span
                    > in the last 24h
                {/if}
            </p>
        </div>
        <TimeRangeSelector
            bind:value={selectedTimeRange}
            onValueChange={handleTimeRangeChange}
            class="shrink-0"
        />
    </div>

    <!-- Dropped Metrics Alerts -->
    <DroppedMetricsAlert alerts={droppedAlerts} />

    <!-- Stat cards: auto-fill grid -->
    <div class="grid grid-stats gap-5 mb-5">
        <DashboardStats {stats} {trends} timeRange={selectedTimeRange} hasSufficientTrendData={hasSufficientTrendData} aggregatedMetrics={$aggregatedMetrics} {packagesStats} {activeIncidents} />
    </div>

    <!-- Chart cards -->
    <div class="grid gap-5 mb-8 2xl:grid-cols-2">
        <DashboardCharts
            aggregatedMetrics={$aggregatedMetrics}
            {stats}
            timeRange={selectedTimeRange}
        />
    </div>
{/if}
