<script lang="ts">
    import { goto } from "$app/navigation";
    import { formatBytes, formatPercent, isAgentOutdated } from "$lib/utils";
    import {
        EllipsisVertical,
        Pencil,
        Pause,
        Play,
        Trash2,
        BellRing,
        AlertTriangle,
    } from "lucide-svelte";
    import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
    import HostFilters from "$lib/components/host/HostFilters.svelte";
    import type { HostWithMetrics, Metric, Host, HostStatus } from "$lib/types";

    const {
        hosts,
        latestMetrics,
        activeIncidentHostIds = new Map(),
        showFilters = true,
        tableLoading = false,
        latestAgentVersion = null,
        onRename,
        onPause,
        onResume,
        onDelete,
    }: {
        hosts: HostWithMetrics[];
        latestMetrics: Record<string, Metric>;
        activeIncidentHostIds?: Map<string, number>;
        showFilters?: boolean;
        tableLoading?: boolean;
        latestAgentVersion?: string | null;
        onRename: (host: Host) => void;
        onPause: (hostId: string) => void;
        onResume: (hostId: string) => void;
        onDelete: (host: Host) => void;
    } = $props();

    function hasIPMismatch(host: Host): boolean {
        return !!(
            host.configured_ip &&
            host.ip_address_v4 &&
            host.configured_ip !== host.ip_address_v4 &&
            !host.ignore_ip_mismatch
        );
    }

    function statusDotColor(status: HostStatus): string {
        switch (status) {
            case "online":
            case "ip_mismatch":
                return "bg-success";
            case "offline":
                return "bg-danger";
            case "pending":
                return "bg-warning";
            case "paused":
                return "bg-muted-foreground";
            default:
                return "bg-muted-foreground";
        }
    }

    function statusLabel(host: Host): string {
        if (host.status === "ip_mismatch") {
            return host.configured_ip
                ? `Online — IP mismatch: expected ${host.configured_ip}`
                : "Online — IP mismatch";
        }
        return host.status.charAt(0).toUpperCase() + host.status.slice(1);
    }

    let sortColumn = $state("name");
    let sortOrder = $state<"asc" | "desc">("asc");
    let searchQuery = $state("");
    let statusFilter = $state("");

    function handleSearchInput(e: Event) {
        searchQuery = (e.target as HTMLInputElement).value;
    }

    function handleStatusChange(value: string) {
        statusFilter = value;
    }

    function handleSort(column: string) {
        if (sortColumn === column) {
            sortOrder = sortOrder === "asc" ? "desc" : "asc";
        } else {
            sortColumn = column;
            sortOrder = "desc";
        }
    }

    function getLastMetrics(hostId: string) {
        const latest = latestMetrics[hostId];
        if (!latest) {
            return {
                hasData: false,
                cpu: 0,
                memory: 0,
                disk: 0,
                load1: 0,
                load5: 0,
                load15: 0,
                netRx: 0,
                netTx: 0,
                temp: 0,
            };
        }

        return {
            hasData: true,
            cpu: latest.cpu_usage_percent || 0,
            memory:
                latest.memory_total_bytes > 0
                    ? (latest.memory_used_bytes / latest.memory_total_bytes) *
                      100
                    : 0,
            disk:
                latest.disk_total_bytes > 0
                    ? (latest.disk_used_bytes / latest.disk_total_bytes) * 100
                    : 0,
            load1: latest.load_avg_1min || 0,
            load5: latest.load_avg_5min || 0,
            load15: latest.load_avg_15min || 0,
            netRx: latest.network_rx_bytes_per_sec || 0,
            netTx: latest.network_tx_bytes_per_sec || 0,
            temp: latest.cpu_temperature_celsius || 0,
        };
    }

    function getBarColor(percent: number): string {
        if (percent >= 90) return "bg-danger";
        if (percent >= 70) return "bg-warning";
        return "bg-primary";
    }

    const sortedHosts = $derived(() => {
        const query = searchQuery.toLowerCase();
        const filtered = hosts.filter((s) => {
            if (statusFilter && s.host.status !== statusFilter) return false;
            if (query) {
                const name = (s.host.display_name || "").toLowerCase();
                const hostname = (s.host.hostname || "").toLowerCase();
                if (!name.includes(query) && !hostname.includes(query))
                    return false;
            }
            return true;
        });
        const metricsMap = new Map(
            filtered.map((h) => [h.host.id, getLastMetrics(h.host.id)]),
        );
        const sorted = [...filtered].sort((a, b) => {
            let valA, valB;
            switch (sortColumn) {
                case "name":
                    valA = (a.host.display_name || "").toLowerCase();
                    valB = (b.host.display_name || "").toLowerCase();
                    break;
                case "cpu": {
                    valA = metricsMap.get(a.host.id)?.cpu ?? 0;
                    valB = metricsMap.get(b.host.id)?.cpu ?? 0;
                    break;
                }
                case "memory": {
                    valA = metricsMap.get(a.host.id)?.memory ?? 0;
                    valB = metricsMap.get(b.host.id)?.memory ?? 0;
                    break;
                }
                case "disk": {
                    valA = metricsMap.get(a.host.id)?.disk ?? 0;
                    valB = metricsMap.get(b.host.id)?.disk ?? 0;
                    break;
                }
                case "load": {
                    valA = metricsMap.get(a.host.id)?.load1 ?? 0;
                    valB = metricsMap.get(b.host.id)?.load1 ?? 0;
                    break;
                }
                case "net": {
                    const mA = metricsMap.get(a.host.id);
                    const mB = metricsMap.get(b.host.id);
                    valA = (mA?.netRx ?? 0) + (mA?.netTx ?? 0);
                    valB = (mB?.netRx ?? 0) + (mB?.netTx ?? 0);
                    break;
                }
                case "temp": {
                    valA = metricsMap.get(a.host.id)?.temp ?? 0;
                    valB = metricsMap.get(b.host.id)?.temp ?? 0;
                    break;
                }
                default:
                    return 0;
            }
            if (valA < valB) return sortOrder === "asc" ? -1 : 1;
            if (valA > valB) return sortOrder === "asc" ? 1 : -1;
            return 0;
        });
        return sorted;
    });

    const displayedHosts = $derived(sortedHosts());
</script>

{#snippet sortIcon(column)}
    {#if sortColumn === column}
        <svg class="h-3 w-3" viewBox="0 0 12 12" fill="currentColor">
            {#if sortOrder === "asc"}
                <path d="M6 2l4 5H2z" />
            {:else}
                <path d="M6 10l4-5H2z" />
            {/if}
        </svg>
    {:else}
        <svg
            class="h-3 w-3 opacity-40 group-hover:opacity-100 transition-opacity"
            viewBox="0 0 12 12"
            fill="currentColor"
        >
            <path d="M6 10l4-5H2z" />
        </svg>
    {/if}
{/snippet}

{#snippet metricBar(percent: number)}
    <div class="w-16 h-1.5 rounded-full bg-muted mt-1">
        <div
            class="h-full rounded-full {getBarColor(percent)}"
            style="width: {Math.min(percent, 100)}%"
        ></div>
    </div>
{/snippet}

{#snippet statusDot(host: Host, size: "sm" | "md" = "md")}
    {@const isOnline =
        host.status === "online" || host.status === "ip_mismatch"}
    {@const dotColor = statusDotColor(host.status)}
    {@const hw = size === "sm" ? "h-2 w-2" : "h-2.5 w-2.5"}
    <span class="relative flex shrink-0 {hw}" title={statusLabel(host)}>
        {#if isOnline}
            <span
                class="animate-ping absolute inline-flex h-full w-full rounded-full {dotColor} opacity-40"
                style="animation-duration: 2.5s"
            ></span>
        {/if}
        <span class="relative inline-flex rounded-full {hw} {dotColor}"></span>
    </span>
{/snippet}

{#if showFilters}
    <HostFilters
        {searchQuery}
        {statusFilter}
        onSearchInput={handleSearchInput}
        onStatusChange={handleStatusChange}
    />
{/if}

<div>
    <!-- Mobile: Cards layout -->
    <div
        class="md:hidden p-3 flex flex-col gap-2"
        class:opacity-50={tableLoading}
        class:pointer-events-none={tableLoading}
    >
        {#each displayedHosts as { host }}
            {@const metrics = getLastMetrics(host.id)}
            <a
                href="/hosts/{host.id}"
                class="block rounded-lg border bg-card hover:bg-muted/20 transition-colors"
            >
                <!-- Header: name + status dot + actions -->
                <div
                    class="rounded-t-lg bg-table-header px-4 py-3 border-b border-border"
                >
                    <div class="flex items-center justify-between gap-2">
                        <div class="min-w-0">
                            <div class="flex items-center gap-2">
                                <span class="mt-0.75"
                                    >{@render statusDot(host, "sm")}</span
                                >
                                <span
                                    class="font-medium text-foreground break-all"
                                    >{host.display_name}</span
                                >
                                {#if activeIncidentHostIds.has(host.id)}
                                    {@const count =
                                        activeIncidentHostIds.get(host.id) ?? 0}
                                    <span
                                        class="flex items-center gap-1 text-warning"
                                    >
                                        <BellRing
                                            class="shrink-0 h-3.5 w-3.5"
                                        />
                                        <span class="text-xs font-medium"
                                            >{count}</span
                                        >
                                    </span>
                                {/if}
                            </div>
                            {#if host.hostname || host.ip_address_v4 || host.configured_ip}
                                <p
                                    class="text-xs text-muted-foreground mt-0.5 ml-4 flex items-center gap-2"
                                >
                                    {#if host.hostname}<span
                                            >{host.hostname}</span
                                        >{/if}
                                    {#if host.ip_address_v4 || host.configured_ip}
                                        <span
                                            class="flex items-center gap-1 text-muted-foreground/70"
                                        >
                                            {host.ip_address_v4 ||
                                                host.configured_ip}
                                            {#if hasIPMismatch(host)}
                                                <AlertTriangle
                                                    class="h-3 w-3 text-warning"
                                                />
                                            {/if}
                                        </span>
                                    {/if}
                                </p>
                            {/if}
                        </div>
                        <div class="flex items-center shrink-0">
                            <!-- svelte-ignore a11y_click_events_have_key_events -->
                            <!-- svelte-ignore a11y_no_static_element_interactions -->
                            <div
                                class="flex items-center"
                                onclick={(e) => {
                                    e.preventDefault();
                                    e.stopPropagation();
                                }}
                            >
                                <DropdownMenu.Root>
                                    <DropdownMenu.Trigger>
                                        {#snippet child({ props })}
                                            <button
                                                type="button"
                                                {...props}
                                                class="rounded-lg p-1 text-muted-foreground transition-colors hover:bg-table-header-active hover:text-foreground"
                                                title="Host actions"
                                            >
                                                <EllipsisVertical
                                                    class="h-4 w-4"
                                                />
                                            </button>
                                        {/snippet}
                                    </DropdownMenu.Trigger>
                                    <DropdownMenu.Content
                                        side="bottom"
                                        align="end"
                                    >
                                        <DropdownMenu.Item
                                            onclick={() => onRename(host)}
                                        >
                                            <Pencil class="h-4 w-4" />
                                            Rename
                                        </DropdownMenu.Item>
                                        {#if host.status !== "pending"}
                                            {#if host.status === "paused"}
                                                <DropdownMenu.Item
                                                    onclick={() =>
                                                        onResume(host.id)}
                                                >
                                                    <Play class="h-4 w-4" />
                                                    Resume
                                                </DropdownMenu.Item>
                                            {:else}
                                                <DropdownMenu.Item
                                                    onclick={() =>
                                                        onPause(host.id)}
                                                >
                                                    <Pause class="h-4 w-4" />
                                                    Pause
                                                </DropdownMenu.Item>
                                            {/if}
                                        {/if}
                                        <DropdownMenu.Separator />
                                        <DropdownMenu.Item
                                            onclick={() => onDelete(host)}
                                            class="text-destructive data-highlighted:text-destructive"
                                        >
                                            <Trash2 class="h-4 w-4" />
                                            Delete
                                        </DropdownMenu.Item>
                                    </DropdownMenu.Content>
                                </DropdownMenu.Root>
                            </div>
                        </div>
                    </div>
                </div>

                <div class="px-4 py-3">
                    {#if metrics.hasData}
                        <div class="space-y-2 text-xs">
                            {#each [{ label: "CPU", value: metrics.cpu }, { label: "Mem", value: metrics.memory }, { label: "Disk", value: metrics.disk }] as { label, value }}
                                <div class="flex items-center gap-2">
                                    <span
                                        class="w-12 text-muted-foreground shrink-0"
                                        >{label}</span
                                    >
                                    <div
                                        class="flex-1 h-2.5 rounded-full bg-muted"
                                    >
                                        <div
                                            class="h-full rounded-full {getBarColor(
                                                value,
                                            )}"
                                            style="width: {Math.min(
                                                value,
                                                100,
                                            )}%"
                                        ></div>
                                    </div>
                                    <span
                                        class="w-12 text-foreground text-left shrink-0"
                                        >{formatPercent(value)}</span
                                    >
                                </div>
                            {/each}
                            <div class="flex items-center gap-2">
                                <span
                                    class="w-12 text-muted-foreground shrink-0"
                                    >Load</span
                                >
                                <span class="text-foreground"
                                    >{metrics.load1.toFixed(2)}
                                    {metrics.load5.toFixed(2)}
                                    {metrics.load15.toFixed(2)}</span
                                >
                            </div>
                            <div class="flex items-center gap-2">
                                <span
                                    class="w-12 text-muted-foreground shrink-0"
                                    >Net</span
                                >
                                <span class="text-foreground"
                                    >↓{formatBytes(metrics.netRx)}/s ↑{formatBytes(
                                        metrics.netTx,
                                    )}/s</span
                                >
                            </div>
                            {#if metrics.temp > 0}
                                <div class="flex items-center gap-2">
                                    <span
                                        class="w-12 text-muted-foreground shrink-0"
                                        >Temp</span
                                    >
                                    <span class="text-foreground"
                                        >{Math.round(metrics.temp)}°C</span
                                    >
                                </div>
                            {/if}
                            {#if host.agent_version}
                                <div class="flex items-center gap-2">
                                    <span
                                        class="w-12 text-muted-foreground shrink-0"
                                        >Agent</span
                                    >
                                    <span
                                        class="text-foreground flex items-center gap-1"
                                    >
                                        v{host.agent_version}
                                        {#if isAgentOutdated(host.agent_version, latestAgentVersion)}
                                            <span
                                                class="text-warning font-medium"
                                                >↑</span
                                            >
                                        {/if}
                                    </span>
                                </div>
                            {/if}
                            {#if (host.outdated_count ?? 0) > 0 || (host.security_count ?? 0) > 0}
                                <div class="flex items-center gap-2">
                                    <span
                                        class="w-12 text-muted-foreground shrink-0"
                                        >Pkgs</span
                                    >
                                    <span class="flex items-center gap-1">
                                        {#if (host.security_count ?? 0) > 0}
                                            <span
                                                class="inline-flex items-center justify-center rounded-full border border-danger/20 bg-danger/10 min-w-5 px-1 py-0.5 font-medium text-danger"
                                                title="{host.security_count} security update{(host.security_count ??
                                                    0) > 1
                                                    ? 's'
                                                    : ''}"
                                                >{host.security_count}</span
                                            >
                                        {/if}
                                        {#if (host.outdated_count ?? 0) > 0}
                                            <span
                                                class="inline-flex items-center justify-center rounded-full border border-warning/20 bg-warning/10 min-w-5 px-1 py-0.5 font-medium text-warning"
                                                title="{host.outdated_count} outdated package{(host.outdated_count ??
                                                    0) > 1
                                                    ? 's'
                                                    : ''}"
                                                >{host.outdated_count}</span
                                            >
                                        {/if}
                                    </span>
                                </div>
                            {/if}
                        </div>
                    {:else}
                        <p class="text-xs text-muted-foreground">
                            No metrics available
                        </p>
                    {/if}
                </div>
            </a>
        {/each}
    </div>

    <!-- Desktop: Table layout -->
    <div class="hidden md:block overflow-auto max-h-[65vh]">
        <table class="w-full min-w-260">
            <colgroup>
                <col class="min-w-50" />
                <col class="w-28" />
                <col class="w-28" />
                <col class="w-28" />
                <col class="w-33" />
                <col class="w-38" />
                <col class="w-16" />
                <col class="w-28" />
                <col class="w-24" />
                <col class="w-24" />
            </colgroup>
            <thead>
                <tr
                    class="bg-table-header sticky top-0 z-10 [box-shadow:0_1px_0_var(--border)] whitespace-nowrap"
                >
                    <th
                        scope="col"
                        class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
                    >
                        <button
                            type="button"
                            onclick={() => handleSort("name")}
                            class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
                            'name'
                                ? 'bg-table-header-active text-foreground'
                                : ''}"
                        >
                            Host {@render sortIcon("name")}
                        </button>
                    </th>
                    <th
                        scope="col"
                        class="px-4 py-2.5 text-center text-sm font-semibold text-muted-foreground"
                    >
                        <button
                            type="button"
                            onclick={() => handleSort("cpu")}
                            class="group inline-flex items-center gap-1 justify-center h-8 rounded-md px-2.5 mx-auto select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
                            'cpu'
                                ? 'bg-table-header-active text-foreground'
                                : ''}"
                        >
                            CPU {@render sortIcon("cpu")}
                        </button>
                    </th>
                    <th
                        scope="col"
                        class="px-4 py-2.5 text-center text-sm font-semibold text-muted-foreground"
                    >
                        <button
                            type="button"
                            onclick={() => handleSort("memory")}
                            class="group inline-flex items-center gap-1 justify-center h-8 rounded-md px-2.5 mx-auto select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
                            'memory'
                                ? 'bg-table-header-active text-foreground'
                                : ''}"
                        >
                            Memory {@render sortIcon("memory")}
                        </button>
                    </th>
                    <th
                        scope="col"
                        class="px-4 py-2.5 text-center text-sm font-semibold text-muted-foreground"
                    >
                        <button
                            type="button"
                            onclick={() => handleSort("disk")}
                            class="group inline-flex items-center gap-1 justify-center h-8 rounded-md px-2.5 mx-auto select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
                            'disk'
                                ? 'bg-table-header-active text-foreground'
                                : ''}"
                        >
                            Disk {@render sortIcon("disk")}
                        </button>
                    </th>
                    <th
                        scope="col"
                        class="px-4 py-2.5 text-center text-sm font-semibold text-muted-foreground"
                    >
                        <button
                            type="button"
                            onclick={() => handleSort("load")}
                            class="group inline-flex items-center gap-1 justify-center h-8 rounded-md px-2.5 mx-auto select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
                            'load'
                                ? 'bg-table-header-active text-foreground'
                                : ''}"
                        >
                            Load {@render sortIcon("load")}
                        </button>
                    </th>
                    <th
                        scope="col"
                        class="px-4 py-2.5 text-center text-sm font-semibold text-muted-foreground"
                    >
                        <button
                            type="button"
                            onclick={() => handleSort("net")}
                            class="group inline-flex items-center gap-1 justify-center h-8 rounded-md px-2.5 mx-auto select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
                            'net'
                                ? 'bg-table-header-active text-foreground'
                                : ''}"
                        >
                            Net {@render sortIcon("net")}
                        </button>
                    </th>
                    <th
                        scope="col"
                        class="px-4 py-2.5 text-center text-sm font-semibold text-muted-foreground"
                    >
                        <button
                            type="button"
                            onclick={() => handleSort("temp")}
                            class="group inline-flex items-center gap-1 justify-center h-8 rounded-md px-2.5 mx-auto select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
                            'temp'
                                ? 'bg-table-header-active text-foreground'
                                : ''}"
                        >
                            Temp {@render sortIcon("temp")}
                        </button>
                    </th>
                    <th
                        scope="col"
                        class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground whitespace-nowrap"
                    >
                        Agent
                    </th>
                    <th
                        scope="col"
                        class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground whitespace-nowrap"
                    >
                        Packages
                    </th>
                    <th
                        scope="col"
                        class="px-4 py-2.5 text-center text-sm font-semibold text-muted-foreground"
                    >
                    </th>
                </tr>
            </thead>
            <tbody
                class="divide-y divide-border"
                class:opacity-50={tableLoading}
                class:pointer-events-none={tableLoading}
            >
                {#each displayedHosts as { host }}
                    {@const metrics = getLastMetrics(host.id)}
                    <tr
                        onclick={() => goto(`/hosts/${host.id}`)}
                        class="hover:bg-muted/20 transition-colors cursor-pointer"
                    >
                        <!-- Host Name + status dot -->
                        <td class="px-4 py-3">
                            <div class="group flex flex-col">
                                <div class="flex items-center gap-2.5">
                                    <span class="mt-0.75"
                                        >{@render statusDot(host)}</span
                                    >
                                    <span
                                        class="font-medium text-foreground group-hover:text-primary transition-colors whitespace-nowrap"
                                    >
                                        {host.display_name}
                                    </span>
                                    {#if activeIncidentHostIds.has(host.id)}
                                        {@const count =
                                            activeIncidentHostIds.get(
                                                host.id,
                                            ) ?? 0}
                                        <span
                                            class="flex items-center gap-1 text-warning"
                                        >
                                            <BellRing
                                                class="shrink-0 h-3.5 w-3.5"
                                            />
                                            <span class="text-xs font-medium"
                                                >{count}</span
                                            >
                                        </span>
                                    {/if}
                                </div>
                                {#if host.hostname}
                                    <span
                                        class="text-xs text-muted-foreground whitespace-nowrap ml-5"
                                        >{host.hostname}</span
                                    >
                                {/if}
                                {#if host.ip_address_v4 || host.configured_ip}
                                    <span
                                        class="flex items-center gap-1 text-xs text-muted-foreground/70 whitespace-nowrap ml-5"
                                    >
                                        {host.ip_address_v4 ||
                                            host.configured_ip}
                                        {#if hasIPMismatch(host)}
                                            <span title="IP mismatch: expected {host.configured_ip}">
                                                <AlertTriangle class="h-3 w-3 text-warning" />
                                            </span>
                                        {/if}
                                    </span>
                                {/if}
                            </div>
                        </td>

                        <!-- CPU -->
                        <td class="px-4 py-3 text-center">
                            {#if metrics.hasData}
                                <div class="flex flex-col items-center">
                                    <span class="text-foreground">
                                        {formatPercent(metrics.cpu)}
                                    </span>
                                    {@render metricBar(metrics.cpu)}
                                </div>
                            {:else}
                                <span class="text-muted-foreground">-</span>
                            {/if}
                        </td>

                        <!-- Memory -->
                        <td class="px-4 py-3 text-center">
                            {#if metrics.hasData}
                                <div class="flex flex-col items-center">
                                    <span class="text-foreground">
                                        {formatPercent(metrics.memory)}
                                    </span>
                                    {@render metricBar(metrics.memory)}
                                </div>
                            {:else}
                                <span class="text-muted-foreground">-</span>
                            {/if}
                        </td>

                        <!-- Disk -->
                        <td class="px-4 py-3 text-center">
                            {#if metrics.hasData}
                                <div class="flex flex-col items-center">
                                    <span class="text-foreground">
                                        {formatPercent(metrics.disk)}
                                    </span>
                                    {@render metricBar(metrics.disk)}
                                </div>
                            {:else}
                                <span class="text-muted-foreground">-</span>
                            {/if}
                        </td>

                        <!-- Load Avg -->
                        <td class="px-4 py-3 text-center">
                            {#if metrics.hasData}
                                <span
                                    class="text-sm text-foreground whitespace-nowrap"
                                    >{metrics.load1.toFixed(2)}
                                    {metrics.load5.toFixed(2)}
                                    {metrics.load15.toFixed(2)}</span
                                >
                            {:else}
                                <span class="text-muted-foreground">-</span>
                            {/if}
                        </td>

                        <!-- Network -->
                        <td class="px-4 py-3 text-center">
                            {#if metrics.hasData}
                                <div
                                    class="flex flex-col items-center text-sm whitespace-nowrap"
                                >
                                    <span class="text-foreground"
                                        >{formatBytes(metrics.netRx)}/s ↓</span
                                    >
                                    <span class="text-foreground"
                                        >{formatBytes(metrics.netTx)}/s ↑</span
                                    >
                                </div>
                            {:else}
                                <span class="text-muted-foreground">-</span>
                            {/if}
                        </td>

                        <!-- Temperature -->
                        <td class="px-4 py-3 text-center">
                            {#if metrics.hasData && metrics.temp > 0}
                                <span class="text-foreground"
                                    >{Math.round(metrics.temp)}°C</span
                                >
                            {:else}
                                <span class="text-muted-foreground"
                                    >{metrics.hasData ? "" : "-"}</span
                                >
                            {/if}
                        </td>

                        <!-- Agent -->
                        <td class="px-4 py-3 text-sm whitespace-nowrap">
                            <div class="flex items-center gap-1.5">
                                <span class="text-foreground">
                                    {host.agent_version
                                        ? `v${host.agent_version}`
                                        : "—"}
                                </span>
                                {#if isAgentOutdated(host.agent_version, latestAgentVersion)}
                                    <span
                                        class="inline-flex items-center rounded-full border border-warning/20 bg-warning/10 px-1.5 py-0.5 text-xs font-medium text-warning"
                                        title="v{latestAgentVersion} available"
                                        >↑</span
                                    >
                                {/if}
                            </div>
                        </td>

                        <!-- Packages -->
                        <td class="px-4 py-3 text-sm whitespace-nowrap">
                            {#if (host.outdated_count ?? 0) > 0 || (host.security_count ?? 0) > 0}
                                <div class="flex items-center gap-1.5">
                                    {#if (host.security_count ?? 0) > 0}
                                        <span
                                            class="inline-flex items-center justify-center rounded-full border border-danger/20 bg-danger/10 min-w-5 px-1 py-0.5 text-xs font-medium text-danger"
                                            title="{host.security_count} security update{(host.security_count ??
                                                0) > 1
                                                ? 's'
                                                : ''}"
                                        >
                                            {host.security_count}
                                        </span>
                                    {/if}
                                    {#if (host.outdated_count ?? 0) > 0}
                                        <span
                                            class="inline-flex items-center justify-center rounded-full border border-warning/20 bg-warning/10 min-w-5 px-1 py-0.5 text-xs font-medium text-warning"
                                            title="{host.outdated_count} outdated package{(host.outdated_count ??
                                                0) > 1
                                                ? 's'
                                                : ''}"
                                        >
                                            {host.outdated_count}
                                        </span>
                                    {/if}
                                </div>
                            {:else}
                                <span class="text-muted-foreground">—</span>
                            {/if}
                        </td>

                        <!-- Actions menu -->
                        <!-- svelte-ignore a11y_click_events_have_key_events -->
                        <td
                            class="px-4 py-3 text-center"
                            onclick={(e) => e.stopPropagation()}
                        >
                            <DropdownMenu.Root>
                                <DropdownMenu.Trigger>
                                    {#snippet child({ props })}
                                        <button
                                            type="button"
                                            {...props}
                                            class="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-table-header-active hover:text-foreground"
                                            title="Host actions"
                                        >
                                            <EllipsisVertical class="h-5 w-5" />
                                        </button>
                                    {/snippet}
                                </DropdownMenu.Trigger>
                                <DropdownMenu.Content side="bottom" align="end">
                                    <DropdownMenu.Item
                                        onclick={(e) => {
                                            e.stopPropagation();
                                            onRename(host);
                                        }}
                                    >
                                        <Pencil class="h-4 w-4" />
                                        Rename
                                    </DropdownMenu.Item>
                                    {#if host.status !== "pending"}
                                        {#if host.status === "paused"}
                                            <DropdownMenu.Item
                                                onclick={(e) => {
                                                    e.stopPropagation();
                                                    onResume(host.id);
                                                }}
                                            >
                                                <Play class="h-4 w-4" />
                                                Resume
                                            </DropdownMenu.Item>
                                        {:else}
                                            <DropdownMenu.Item
                                                onclick={(e) => {
                                                    e.stopPropagation();
                                                    onPause(host.id);
                                                }}
                                            >
                                                <Pause class="h-4 w-4" />
                                                Pause
                                            </DropdownMenu.Item>
                                        {/if}
                                    {/if}
                                    <DropdownMenu.Separator />
                                    <DropdownMenu.Item
                                        onclick={(e) => {
                                            e.stopPropagation();
                                            onDelete(host);
                                        }}
                                        class="text-destructive data-highlighted:text-destructive"
                                    >
                                        <Trash2 class="h-4 w-4" />
                                        Delete
                                    </DropdownMenu.Item>
                                </DropdownMenu.Content>
                            </DropdownMenu.Root>
                        </td>
                    </tr>
                {/each}
            </tbody>
        </table>
    </div>

    {#if displayedHosts.length === 0}
        <div
            class="flex flex-col items-center justify-center py-12 text-center"
        >
            <svg
                class="h-12 w-12 text-muted-foreground/50 mb-3"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
            >
                <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="1.5"
                    d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"
                />
            </svg>
            {#if hosts.length === 0}
                <p class="text-sm text-muted-foreground">No hosts found</p>
                <p class="text-xs text-muted-foreground mt-1">
                    Add your first host to start monitoring
                </p>
            {:else}
                <p class="text-sm text-muted-foreground">No matching hosts</p>
            {/if}
        </div>
    {/if}
</div>
