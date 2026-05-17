<script lang="ts">
    import { Server, Cpu, MemoryStick, HardDrive, Activity, Package, ShieldAlert, AlertTriangle, ArrowUp, ArrowDown } from "lucide-svelte";
    import { formatBytes } from "$lib/utils";
    import { ALERT_METRIC_LABELS, type TimeRange, type AggregatedMetric, type ActiveIncident } from "$lib/types";
    import Sparkline from "./Sparkline.svelte";

    interface TrendItem {
        direction: "up" | "down" | "stable";
        delta: number;
    }

    interface Trends {
        cpu: TrendItem;
        memory: TrendItem;
        disk: TrendItem;
        loadAvg: TrendItem;
    }

    interface Stats {
        totalHosts: number;
        onlineHosts: number;
        offlineHosts: number;
        avgCPU: number;
        avgMemory: number;
        avgDisk: number;
        usedMemory: number;
        totalMemory: number;
        loadAvg: number;
    }

    const {
        stats,
        trends,
        timeRange,
        hasSufficientTrendData,
        aggregatedMetrics,
        packagesStats,
        activeIncidents,
    }: {
        stats: Stats;
        trends: Trends;
        timeRange: TimeRange;
        hasSufficientTrendData: boolean;
        aggregatedMetrics: AggregatedMetric[];
        packagesStats: { outdatedCount: number; securityCount: number; outdatedHostsCount: number; securityHostsCount: number };
        activeIncidents: ActiveIncident[];
    } = $props();

    const tsSeries = $derived(aggregatedMetrics.map(m => new Date(m.timestamp).getTime() / 1000));
    const cpuSeries = $derived(aggregatedMetrics.map(m => m.cpu_usage_percent));
    const memorySeries = $derived(aggregatedMetrics.map(m =>
        m.memory_total_bytes > 0 ? (m.memory_used_bytes / m.memory_total_bytes) * 100 : 0
    ));
    const diskSeries = $derived(aggregatedMetrics.map(m =>
        m.disk_total_bytes > 0 ? (m.disk_used_bytes / m.disk_total_bytes) * 100 : 0
    ));
    const loadSeries = $derived(aggregatedMetrics.map(m => m.load_avg_1min));

    function valueColor(pct: number): string {
        if (pct >= 85) return "text-destructive";
        if (pct >= 70) return "text-warning";
        return "text-foreground";
    }

    const memoryPct = $derived(
        stats.totalMemory > 0 ? (stats.usedMemory / stats.totalMemory) * 100 : 0
    );

</script>

<!-- display:contents lets each card become a direct grid item of the parent bento grid -->
<div class="contents">

    <!-- Hosts — 2×1 -->
    <div class="flex flex-col rounded-lg border bg-card">
        <div class="flex flex-col gap-2 p-4 flex-1">
            <div class="flex items-center justify-between">
                <h3 class="text-sm text-muted-foreground">Hosts</h3>
                <div class="flex items-center justify-center rounded-md bg-primary/10 text-primary h-8 w-8 shrink-0">
                    <Server class="h-4 w-4" />
                </div>
            </div>
            <p class="text-3xl font-bold text-foreground tabular-nums">
                {stats.onlineHosts}<span class="text-lg font-medium text-muted-foreground">/{stats.totalHosts}</span>
            </p>
            <p class="text-xs text-muted-foreground tabular-nums">
                {stats.totalHosts > 0 ? ((stats.onlineHosts / stats.totalHosts) * 100).toFixed(1) + '% uptime' : 'No hosts yet'}
            </p>
            <div class="flex items-center gap-1.5 mt-auto">
                {#if stats.offlineHosts > 0}
                    <span class="h-1.5 w-1.5 rounded-full bg-destructive shrink-0"></span>
                    <span class="text-xs text-destructive">{stats.offlineHosts} offline</span>
                {:else if stats.totalHosts > 0}
                    <span class="h-1.5 w-1.5 rounded-full bg-success shrink-0"></span>
                    <span class="text-xs text-success">All online</span>
                {:else}
                    <span class="text-xs text-muted-foreground">—</span>
                {/if}
                <a href="/hosts" class="text-xs text-muted-foreground hover:text-foreground transition-colors ml-auto">View all →</a>
            </div>
        </div>
    </div>

    <!-- CPU — 2×1 -->
    <div class="flex flex-col rounded-lg border bg-card">
        <div class="flex flex-col gap-2 p-4 flex-1">
            <div class="flex items-center justify-between">
                <h3 class="text-sm text-muted-foreground">CPU</h3>
                <div class="flex items-center justify-center rounded-md bg-primary/10 text-primary h-8 w-8 shrink-0">
                    <Cpu class="h-4 w-4" />
                </div>
            </div>
            <p class="text-3xl font-bold tabular-nums {valueColor(stats.avgCPU)}">
                {stats.avgCPU.toFixed(1)}<span class="text-lg font-medium text-muted-foreground">%</span>
            </p>
            <Sparkline values={cpuSeries} timestamps={tsSeries} {timeRange} yMin={0} yMax={100} class="text-(--chart-1)/60 mt-auto" />
            <div class="flex items-center gap-1.5">
                {#if hasSufficientTrendData && trends.cpu.direction === "up"}
                    <ArrowUp class="h-3 w-3 text-destructive ml-0.5" />
                    <span class="text-xs text-destructive">{Math.abs(trends.cpu.delta).toFixed(1)}%</span>
                    <span class="text-xs text-muted-foreground">vs {timeRange}<span class="hidden sm:inline">&nbsp;ago</span></span>
                {:else if hasSufficientTrendData && trends.cpu.direction === "down"}
                    <ArrowDown class="h-3 w-3 text-success ml-0.5" />
                    <span class="text-xs text-success">{Math.abs(trends.cpu.delta).toFixed(1)}%</span>
                    <span class="text-xs text-muted-foreground">vs {timeRange}<span class="hidden sm:inline">&nbsp;ago</span></span>
                {:else}
                    <span class="text-xs text-muted-foreground">—</span>
                {/if}
            </div>
        </div>
    </div>

    <!-- Memory — 2×1 -->
    <div class="flex flex-col rounded-lg border bg-card">
        <div class="flex flex-col gap-2 p-4 flex-1">
            <div class="flex items-center justify-between">
                <h3 class="text-sm text-muted-foreground">Memory</h3>
                <div class="flex items-center justify-center rounded-md bg-primary/10 text-primary h-8 w-8 shrink-0">
                    <MemoryStick class="h-4 w-4" />
                </div>
            </div>
            <p class="text-3xl font-bold tabular-nums {valueColor(memoryPct)}">
                {memoryPct.toFixed(1)}<span class="text-lg font-medium text-muted-foreground">%</span>
            </p>
            <p class="text-xs text-muted-foreground tabular-nums">
                {formatBytes(stats.usedMemory)} / {formatBytes(stats.totalMemory)}
            </p>
            <Sparkline values={memorySeries} timestamps={tsSeries} {timeRange} yMin={0} yMax={100} class="text-(--chart-2)/60 mt-auto" />
            <div class="flex items-center gap-1.5">
                {#if hasSufficientTrendData && trends.memory.direction === "up"}
                    <ArrowUp class="h-3 w-3 text-destructive ml-0.5" />
                    <span class="text-xs text-destructive">{Math.abs(trends.memory.delta).toFixed(1)}%</span>
                    <span class="text-xs text-muted-foreground">vs {timeRange}<span class="hidden sm:inline">&nbsp;ago</span></span>
                {:else if hasSufficientTrendData && trends.memory.direction === "down"}
                    <ArrowDown class="h-3 w-3 text-success ml-0.5" />
                    <span class="text-xs text-success">{Math.abs(trends.memory.delta).toFixed(1)}%</span>
                    <span class="text-xs text-muted-foreground">vs {timeRange}<span class="hidden sm:inline">&nbsp;ago</span></span>
                {:else}
                    <span class="text-xs text-muted-foreground">—</span>
                {/if}
            </div>
        </div>
    </div>

    <!-- Disk — 2×1 -->
    <div class="flex flex-col rounded-lg border bg-card">
        <div class="flex flex-col gap-2 p-4 flex-1">
            <div class="flex items-center justify-between">
                <h3 class="text-sm text-muted-foreground">Disk</h3>
                <div class="flex items-center justify-center rounded-md bg-primary/10 text-primary h-8 w-8 shrink-0">
                    <HardDrive class="h-4 w-4" />
                </div>
            </div>
            <p class="text-3xl font-bold tabular-nums {valueColor(stats.avgDisk)}">
                {stats.avgDisk.toFixed(1)}<span class="text-lg font-medium text-muted-foreground">%</span>
            </p>
            <Sparkline values={diskSeries} timestamps={tsSeries} {timeRange} yMin={0} yMax={100} class="text-(--chart-3)/60 mt-auto" />
            <div class="flex items-center gap-1.5">
                {#if hasSufficientTrendData && trends.disk.direction === "up"}
                    <ArrowUp class="h-3 w-3 text-destructive ml-0.5" />
                    <span class="text-xs text-destructive">{Math.abs(trends.disk.delta).toFixed(1)}%</span>
                    <span class="text-xs text-muted-foreground">vs {timeRange}<span class="hidden sm:inline">&nbsp;ago</span></span>
                {:else if hasSufficientTrendData && trends.disk.direction === "down"}
                    <ArrowDown class="h-3 w-3 text-success ml-0.5" />
                    <span class="text-xs text-success">{Math.abs(trends.disk.delta).toFixed(1)}%</span>
                    <span class="text-xs text-muted-foreground">vs {timeRange}<span class="hidden sm:inline">&nbsp;ago</span></span>
                {:else}
                    <span class="text-xs text-muted-foreground">—</span>
                {/if}
            </div>
        </div>
    </div>

    <!-- Load Average — 2×1 -->
    <div class="flex flex-col rounded-lg border bg-card">
        <div class="flex flex-col gap-2 p-4 flex-1">
            <div class="flex items-center justify-between">
                <h3 class="text-sm text-muted-foreground">Load Avg 1min</h3>
                <div class="flex items-center justify-center rounded-md bg-primary/10 text-primary h-8 w-8 shrink-0">
                    <Activity class="h-4 w-4" />
                </div>
            </div>
            <p class="text-3xl font-bold text-foreground tabular-nums">
                {stats.loadAvg.toFixed(2)}
            </p>
            <Sparkline values={loadSeries} timestamps={tsSeries} {timeRange} class="text-(--chart-4)/60 mt-auto" />
            <div class="flex items-center gap-1.5">
                {#if hasSufficientTrendData && trends.loadAvg.direction === "up"}
                    <ArrowUp class="h-3 w-3 text-destructive ml-0.5" />
                    <span class="text-xs text-destructive">{Math.abs(trends.loadAvg.delta).toFixed(2)}</span>
                    <span class="text-xs text-muted-foreground">vs {timeRange}<span class="hidden sm:inline">&nbsp;ago</span></span>
                {:else if hasSufficientTrendData && trends.loadAvg.direction === "down"}
                    <ArrowDown class="h-3 w-3 text-success ml-0.5" />
                    <span class="text-xs text-success">{Math.abs(trends.loadAvg.delta).toFixed(2)}</span>
                    <span class="text-xs text-muted-foreground">vs {timeRange}<span class="hidden sm:inline">&nbsp;ago</span></span>
                {:else}
                    <span class="text-xs text-muted-foreground">—</span>
                {/if}
            </div>
        </div>
    </div>

    <!-- Outdated Packages — 2×1 -->
    <div class="flex flex-col rounded-lg border bg-card">
        <div class="flex flex-col gap-2 p-4 flex-1">
            <div class="flex items-center justify-between">
                <h3 class="text-sm text-muted-foreground">Outdated packages</h3>
                <div class="flex items-center justify-center rounded-md bg-primary/10 text-primary h-8 w-8 shrink-0">
                    <Package class="h-4 w-4" />
                </div>
            </div>
            <p class="text-3xl font-bold tabular-nums {packagesStats.outdatedCount > 0 ? 'text-warning' : 'text-foreground'}">
                {packagesStats.outdatedCount}
            </p>
            <p class="text-xs text-muted-foreground">
                {#if packagesStats.outdatedCount > 0}
                    {packagesStats.outdatedHostsCount} host{packagesStats.outdatedHostsCount !== 1 ? "s" : ""} affected
                {:else}
                    all up to date
                {/if}
            </p>
            <div class="flex items-center gap-1.5 mt-auto">
                <a href="/packages?status=outdated" class="text-xs text-muted-foreground hover:text-foreground transition-colors">View all →</a>
            </div>
        </div>
    </div>

    <!-- Security Updates — 2×1 -->
    <div class="flex flex-col rounded-lg border bg-card">
        <div class="flex flex-col gap-2 p-4 flex-1">
            <div class="flex items-center justify-between">
                <h3 class="text-sm text-muted-foreground">Security updates</h3>
                <div class="flex items-center justify-center rounded-md bg-primary/10 text-primary h-8 w-8 shrink-0">
                    <ShieldAlert class="h-4 w-4" />
                </div>
            </div>
            <p class="text-3xl font-bold tabular-nums {packagesStats.securityCount > 0 ? 'text-destructive' : 'text-foreground'}">
                {packagesStats.securityCount}
            </p>
            <p class="text-xs text-muted-foreground">
                {#if packagesStats.securityCount > 0}
                    {packagesStats.securityHostsCount} host{packagesStats.securityHostsCount !== 1 ? "s" : ""} affected
                {:else}
                    no vulnerabilities
                {/if}
            </p>
            <div class="flex items-center gap-1.5 mt-auto">
                <a href="/packages?status=security" class="text-xs text-muted-foreground hover:text-foreground transition-colors">View all →</a>
            </div>
        </div>
    </div>

    <!-- Active Alerts — 2×1 -->
    <div class="flex flex-col rounded-lg border bg-card">
        <div class="flex flex-col gap-2 p-4 flex-1">
            <div class="flex items-center justify-between">
                <h3 class="text-sm text-muted-foreground">Active alerts</h3>
                <div class="flex items-center justify-center rounded-md bg-primary/10 text-primary h-8 w-8 shrink-0">
                    <AlertTriangle class="h-4 w-4" />
                </div>
            </div>
            <p class="text-3xl font-bold tabular-nums {activeIncidents.length > 0 ? 'text-destructive' : 'text-foreground'}">
                {activeIncidents.length}
            </p>
            {#if activeIncidents.length > 0}
                <div class="flex flex-col gap-1 mt-auto">
                    {#each activeIncidents.slice(0, 2) as incident}
                        <p class="text-xs text-muted-foreground truncate">
                            <span class="text-foreground font-medium">{incident.host_name}</span>
                            · {ALERT_METRIC_LABELS[incident.metric_type]}
                        </p>
                    {/each}
                    {#if activeIncidents.length > 2}
                        <p class="text-xs text-muted-foreground">+{activeIncidents.length - 2} more</p>
                    {/if}
                </div>
            {:else}
                <p class="text-xs text-muted-foreground mt-auto">All clear</p>
            {/if}
        </div>
    </div>

</div>
