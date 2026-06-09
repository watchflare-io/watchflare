<script lang="ts">
    import { formatBytes, parsePortBadges } from '$lib/utils';
    import { formatRate } from '$lib/chart-utils';
    import { userStore } from '$lib/stores/user';
    import type { ContainerMetric } from '$lib/types';
    import RuntimeIcon from '$lib/components/icons/RuntimeIcon.svelte';
    import ContainerDetailDrawer from './ContainerDetailDrawer.svelte';

    const { containerMetrics }: { containerMetrics: ContainerMetric[] } = $props();

    const networkUnit = $derived($userStore.user?.network_unit ?? 'bytes');

    let searchQuery = $state('');
    let sortColumn = $state<'name' | 'status' | 'health' | 'image' | 'cpu' | 'memory' | 'network' | 'ports'>('name');
    let sortOrder = $state<'asc' | 'desc'>('asc');
    let drawerOpen = $state(false);
    let selectedContainer = $state<ContainerMetric | null>(null);

    function handleSort(col: typeof sortColumn) {
        if (sortColumn === col) {
            sortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
        } else {
            sortColumn = col;
            sortOrder = 'asc';
        }
    }

    function handleRowClick(container: ContainerMetric) {
        selectedContainer = container;
        drawerOpen = true;
    }

    function handleDrawerClose() {
        drawerOpen = false;
    }

    // Derive the latest metric per container, preserving static fields
    // (image, runtime, ports) from any non-empty record — these are absent
    // from SSE updates which only carry live metrics.
    const latestContainers = $derived((() => {
        type Entry = {
            metric: ContainerMetric;
            image: string;
            runtime: string;
            ports: string;
        };
        const byId = new Map<string, Entry>();

        for (const m of containerMetrics) {
            const entry = byId.get(m.container_id);
            if (!entry) {
                byId.set(m.container_id, {
                    metric: m,
                    image: m.image ?? '',
                    runtime: m.runtime ?? '',
                    ports: m.ports ?? '',
                });
            } else if (m.timestamp > entry.metric.timestamp) {
                byId.set(m.container_id, {
                    metric: m,
                    image: m.image || entry.image,
                    runtime: m.runtime || entry.runtime,
                    ports: m.ports || entry.ports,
                });
            } else {
                if (!entry.image && m.image) entry.image = m.image;
                if (!entry.runtime && m.runtime) entry.runtime = m.runtime;
                if (!entry.ports && m.ports) entry.ports = m.ports;
            }
        }

        return [...byId.values()]
            .map(({ metric, image, runtime, ports }) => ({ ...metric, image, runtime, ports }));
    })());

    // Filtered and sorted containers
    const displayedContainers = $derived((() => {
        let result = latestContainers;

        if (searchQuery.trim()) {
            const q = searchQuery.toLowerCase();
            result = result.filter(c => c.container_name.toLowerCase().includes(q));
        }

        const healthOrder: Record<string, number> = { healthy: 0, starting: 1, unhealthy: 2, '': 3 };

        return [...result].sort((a, b) => {
            let cmp = 0;
            switch (sortColumn) {
                case 'name':
                    cmp = a.container_name.localeCompare(b.container_name);
                    break;
                case 'status':
                    cmp = (a.status ?? '').localeCompare(b.status ?? '');
                    break;
                case 'health':
                    cmp = (healthOrder[a.health ?? ''] ?? 3) - (healthOrder[b.health ?? ''] ?? 3);
                    break;
                case 'image':
                    cmp = (a.image ?? '').localeCompare(b.image ?? '');
                    break;
                case 'cpu':
                    cmp = a.cpu_percent - b.cpu_percent;
                    break;
                case 'memory':
                    cmp = a.memory_used_bytes - b.memory_used_bytes;
                    break;
                case 'network':
                    cmp = (a.network_rx_bytes_per_sec + a.network_tx_bytes_per_sec) -
                          (b.network_rx_bytes_per_sec + b.network_tx_bytes_per_sec);
                    break;
                case 'ports':
                    cmp = (a.ports ?? '').localeCompare(b.ports ?? '');
                    break;
            }
            return sortOrder === 'asc' ? cmp : -cmp;
        });
    })());

    function memoryPercent(m: ContainerMetric): number {
        if (!m.memory_limit_bytes || m.memory_limit_bytes === 0) return 0;
        return Math.min(100, (m.memory_used_bytes / m.memory_limit_bytes) * 100);
    }

    function cpuBarClass(cpu: number): string {
        if (cpu >= 80) return 'bg-danger';
        if (cpu >= 50) return 'bg-warning';
        return 'bg-success';
    }

    function memBarClass(pct: number): string {
        if (pct >= 90) return 'bg-danger';
        if (pct >= 70) return 'bg-warning';
        return 'bg-primary';
    }

    function healthBadgeClass(health: string): string {
        if (health === 'healthy') return 'bg-success/10 text-success border-success/20';
        if (health === 'unhealthy') return 'bg-destructive/10 text-destructive border-destructive/20';
        if (health === 'starting') return 'bg-warning/10 text-warning border-warning/20';
        return 'bg-muted text-muted-foreground border-border';
    }

    function healthLabel(health: string): string {
        return health || 'None';
    }

    function truncateImage(image: string): string {
        if (image.length <= 50) return image;
        return image.substring(0, 47) + '…';
    }
</script>

{#snippet sortIcon(column: string)}
    {#if sortColumn === column}
        <svg class="h-3 w-3" viewBox="0 0 12 12" fill="currentColor">
            {#if sortOrder === 'asc'}
                <path d="M6 2l4 5H2z" />
            {:else}
                <path d="M6 10l4-5H2z" />
            {/if}
        </svg>
    {:else}
        <svg class="h-3 w-3 opacity-40 group-hover:opacity-100 transition-opacity" viewBox="0 0 12 12" fill="currentColor">
            <path d="M6 10l4-5H2z" />
        </svg>
    {/if}
{/snippet}

<ContainerDetailDrawer
    container={selectedContainer}
    open={drawerOpen}
    onClose={handleDrawerClose}
/>

<div class="rounded-xl border bg-card overflow-hidden">
    <!-- Header -->
    <div class="bg-table-header px-4 py-2.5 border-b flex items-center justify-between gap-4">
        <div class="flex items-center gap-2 shrink-0">
            <h3 class="text-sm font-semibold">Containers</h3>
            <span class="inline-flex items-center justify-center rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
                {latestContainers.length}
            </span>
        </div>
        <input
            type="text"
            placeholder="Search..."
            bind:value={searchQuery}
            class="h-8 w-44 rounded-lg border bg-background px-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
        />
    </div>

    <!-- Mobile cards -->
    <div class="sm:hidden p-3 flex flex-col gap-2">
        {#each displayedContainers as container (container.container_id)}
            {@const pct = memoryPercent(container)}
            {@const badges = parsePortBadges(container.ports ?? '')}
            {@const extraPorts = badges.length > 2 ? badges.length - 2 : 0}
            <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
            <div
                class="rounded-lg border bg-card cursor-pointer"
                onclick={() => handleRowClick(container)}
                onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), handleRowClick(container))}
                role="button"
                tabindex="0"
            >
                <div class="rounded-t-lg bg-table-header px-4 py-2.5 border-b border-border flex items-center justify-between gap-2">
                    <div class="flex items-center gap-2 min-w-0">
                        <RuntimeIcon runtime={container.runtime} class="h-4 w-4 shrink-0 text-muted-foreground" />
                        <span class="text-sm font-medium text-foreground truncate">{container.container_name}</span>
                    </div>
                </div>
                <div class="px-4 py-2.5 flex flex-col gap-1">
                    {#if container.image}
                        <p class="text-xs text-muted-foreground truncate mb-1" title={container.image}>
                            {truncateImage(container.image)}
                        </p>
                    {/if}
                    <div class="flex items-baseline gap-2">
                        <span class="w-16 shrink-0 text-xs text-muted-foreground">Status</span>
                        <span class="text-sm">{container.status || '—'}</span>
                    </div>
                    <div class="flex items-center gap-2">
                        <span class="w-16 shrink-0 text-xs text-muted-foreground">Health</span>
                        <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium border {healthBadgeClass(container.health ?? '')}">
                            {healthLabel(container.health ?? '')}
                        </span>
                    </div>
                    <div class="flex items-center gap-2">
                        <span class="w-16 shrink-0 text-xs text-muted-foreground">CPU</span>
                        <div class="w-16 h-1.5 rounded-full bg-muted overflow-hidden">
                            <div class="h-full rounded-full {cpuBarClass(container.cpu_percent)}" style="width: {Math.min(100, container.cpu_percent)}%"></div>
                        </div>
                        <span class="text-sm font-mono">{container.cpu_percent.toFixed(1)}%</span>
                    </div>
                    <div class="flex items-center gap-2">
                        <span class="w-16 shrink-0 text-xs text-muted-foreground">Memory</span>
                        <div class="w-16 h-1.5 rounded-full bg-muted overflow-hidden">
                            <div class="h-full rounded-full {memBarClass(pct)}" style="width: {pct}%"></div>
                        </div>
                        <span class="text-sm font-mono">{formatBytes(container.memory_used_bytes)}</span>
                    </div>
                    <div class="flex items-baseline gap-2">
                        <span class="w-16 shrink-0 text-xs text-muted-foreground">Network</span>
                        <span class="text-sm font-mono">↓ {formatRate(container.network_rx_bytes_per_sec, networkUnit)} ↑ {formatRate(container.network_tx_bytes_per_sec, networkUnit)}</span>
                    </div>
                    {#if badges.length > 0}
                        <div class="flex items-center gap-2">
                            <span class="w-16 shrink-0 text-xs text-muted-foreground">Ports</span>
                            <div class="flex items-center gap-1">
                                {#each badges.slice(0, 2) as badge}
                                    <span class="inline-flex items-center rounded px-1.5 py-0.5 text-xs font-mono bg-muted text-muted-foreground">{badge}</span>
                                {/each}
                                {#if extraPorts > 0}
                                    <span class="inline-flex items-center rounded px-1.5 py-0.5 text-xs font-mono bg-muted text-muted-foreground">+{extraPorts}</span>
                                {/if}
                            </div>
                        </div>
                    {/if}
                </div>
            </div>
        {:else}
            <div class="py-8 text-center text-sm text-muted-foreground">No matching containers</div>
        {/each}
    </div>

    <!-- Desktop table -->
    <div class="hidden sm:block overflow-x-auto">
        <table class="w-full min-w-[900px]">
            <thead>
                <tr class="border-b bg-table-header whitespace-nowrap">
                    {#each ([['name', 'Name'], ['status', 'Status'], ['health', 'Health'], ['image', 'Image'], ['cpu', 'CPU'], ['memory', 'Memory'], ['network', 'Network'], ['ports', 'Ports']] as const) as [col, label]}
                        <th scope="col" class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">
                            <button
                                type="button"
                                onclick={() => handleSort(col)}
                                class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn === col ? 'bg-table-header-active text-foreground' : ''}"
                            >
                                {label} {@render sortIcon(col)}
                            </button>
                        </th>
                    {/each}
                </tr>
            </thead>
            <tbody class="divide-y">
                {#each displayedContainers as container (container.container_id)}
                    {@const pct = memoryPercent(container)}
                    {@const badges = parsePortBadges(container.ports ?? '')}
                    {@const extraPorts = badges.length > 2 ? badges.length - 2 : 0}
                    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
                    <tr
                        class="hover:bg-muted/20 transition-colors cursor-pointer"
                        onclick={() => handleRowClick(container)}
                        onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), handleRowClick(container))}
                        tabindex="0"
                        role="button"
                    >
                        <td class="px-4 py-3 whitespace-nowrap">
                            <div class="flex items-center gap-2">
                                <RuntimeIcon runtime={container.runtime} class="h-4 w-4 shrink-0 text-muted-foreground" />
                                <span class="text-sm font-medium text-foreground">{container.container_name}</span>
                            </div>
                        </td>
                        <td class="px-4 py-3 whitespace-nowrap text-sm text-muted-foreground">
                            {container.status || '—'}
                        </td>
                        <td class="px-4 py-3 whitespace-nowrap">
                            <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium border {healthBadgeClass(container.health ?? '')}">
                                {healthLabel(container.health ?? '')}
                            </span>
                        </td>
                        <td class="px-4 py-3 max-w-xs">
                            <span class="text-sm text-muted-foreground truncate block" title={container.image}>
                                {container.image ? truncateImage(container.image) : '—'}
                            </span>
                        </td>
                        <td class="px-4 py-3 whitespace-nowrap">
                            <div class="flex items-center gap-2">
                                <div class="w-16 h-1.5 rounded-full bg-muted overflow-hidden">
                                    <div
                                        class="h-full rounded-full {cpuBarClass(container.cpu_percent)}"
                                        style="width: {Math.min(100, container.cpu_percent)}%"
                                    ></div>
                                </div>
                                <span class="text-sm font-mono text-muted-foreground w-12">{container.cpu_percent.toFixed(1)}%</span>
                            </div>
                        </td>
                        <td class="px-4 py-3 whitespace-nowrap">
                            <div class="flex items-center gap-2">
                                <div class="w-16 h-1.5 rounded-full bg-muted overflow-hidden">
                                    <div
                                        class="h-full rounded-full {memBarClass(pct)}"
                                        style="width: {pct}%"
                                    ></div>
                                </div>
                                <span class="text-sm font-mono text-muted-foreground">
                                    {formatBytes(container.memory_used_bytes)}
                                </span>
                            </div>
                        </td>
                        <td class="px-4 py-3 whitespace-nowrap text-sm font-mono text-muted-foreground">
                            ↓ {formatRate(container.network_rx_bytes_per_sec, networkUnit)}
                            ↑ {formatRate(container.network_tx_bytes_per_sec, networkUnit)}
                        </td>
                        <td class="px-4 py-3 whitespace-nowrap">
                            {#if badges.length > 0}
                                <div class="flex items-center gap-1">
                                    {#each badges.slice(0, 2) as badge}
                                        <span class="inline-flex items-center rounded px-1.5 py-0.5 text-xs font-mono bg-muted text-muted-foreground">{badge}</span>
                                    {/each}
                                    {#if extraPorts > 0}
                                        <span class="inline-flex items-center rounded px-1.5 py-0.5 text-xs font-mono bg-muted text-muted-foreground">+{extraPorts}</span>
                                    {/if}
                                </div>
                            {:else}
                                <span class="text-sm text-muted-foreground">—</span>
                            {/if}
                        </td>
                    </tr>
                {:else}
                    <tr>
                        <td colspan="8" class="px-4 py-12 text-center text-sm text-muted-foreground">
                            No matching containers
                        </td>
                    </tr>
                {/each}
            </tbody>
        </table>
    </div>
</div>
