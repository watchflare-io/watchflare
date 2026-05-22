<script lang="ts">
    import { formatBytes } from '$lib/utils';
    import { formatRate } from '$lib/chart-utils';
    import { userStore } from '$lib/stores/user';
    import type { ContainerMetric } from '$lib/types';

    const { containerMetrics }: { containerMetrics: ContainerMetric[] } = $props();

    const networkUnit = $derived($userStore.user?.network_unit ?? 'bytes');

    let searchQuery = $state('');
    let sortColumn = $state<'runtime' | 'name' | 'status' | 'health' | 'image' | 'cpu' | 'memory' | 'network' | 'ports'>('name');
    let sortOrder = $state<'asc' | 'desc'>('asc');

    function handleSort(col: typeof sortColumn) {
        if (sortColumn === col) {
            sortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
        } else {
            sortColumn = col;
            sortOrder = 'asc';
        }
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
                case 'runtime':
                    cmp = (a.runtime ?? '').localeCompare(b.runtime ?? '');
                    break;
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

    function runtimeBadgeClass(runtime: string): string {
        if (runtime === 'docker') return 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20';
        if (runtime === 'podman') return 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20';
        return 'bg-muted text-muted-foreground border-border';
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

    // Split comma-separated ports into an array for vertical rendering
    function parsePorts(ports: string): string[] {
        if (!ports) return [];
        return ports.split(', ');
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
            {@const portList = parsePorts(container.ports ?? '')}
            <div class="rounded-lg border bg-card">
                <div class="rounded-t-lg bg-table-header px-4 py-2.5 border-b border-border flex items-center justify-between gap-2">
                    <span class="text-sm font-medium text-foreground truncate">{container.container_name}</span>
                    {#if container.runtime}
                        <span class="shrink-0 inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium border {runtimeBadgeClass(container.runtime)}">
                            {container.runtime}
                        </span>
                    {/if}
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
                        {#if container.memory_limit_bytes > 0}
                            <div class="w-16 h-1.5 rounded-full bg-muted overflow-hidden">
                                <div class="h-full rounded-full {memBarClass(pct)}" style="width: {pct}%"></div>
                            </div>
                            <span class="text-sm font-mono">{formatBytes(container.memory_used_bytes)} / {formatBytes(container.memory_limit_bytes)} <span class="text-xs text-muted-foreground">({pct.toFixed(0)}%)</span></span>
                        {:else}
                            <span class="text-sm font-mono">{formatBytes(container.memory_used_bytes)}</span>
                        {/if}
                    </div>
                    <div class="flex items-baseline gap-2">
                        <span class="w-16 shrink-0 text-xs text-muted-foreground">Network</span>
                        <span class="text-sm font-mono">↓ {formatRate(container.network_rx_bytes_per_sec, networkUnit)} ↑ {formatRate(container.network_tx_bytes_per_sec, networkUnit)}</span>
                    </div>
                    {#if portList.length > 0}
                        <div class="flex items-start gap-2">
                            <span class="w-16 shrink-0 text-xs text-muted-foreground">Ports</span>
                            <div class="flex flex-col gap-0.5">
                                {#each portList as port}
                                    <span class="text-sm font-mono">{port}</span>
                                {/each}
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
                    {#each ([['runtime', 'Runtime'], ['name', 'Name'], ['status', 'Status'], ['health', 'Health'], ['image', 'Image'], ['cpu', 'CPU'], ['memory', 'Memory'], ['network', 'Network'], ['ports', 'Ports']] as const) as [col, label]}
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
                    {@const portList = parsePorts(container.ports ?? '')}
                    <tr class="hover:bg-muted/20 transition-colors">
                        <td class="px-4 py-3 whitespace-nowrap">
                            {#if container.runtime}
                                <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium border {runtimeBadgeClass(container.runtime)}">
                                    {container.runtime}
                                </span>
                            {:else}
                                <span class="text-sm text-muted-foreground">—</span>
                            {/if}
                        </td>
                        <td class="px-4 py-3 whitespace-nowrap text-sm font-medium text-foreground">
                            {container.container_name}
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
                            {#if container.memory_limit_bytes > 0}
                                <div class="flex items-center gap-2">
                                    <div class="w-16 h-1.5 rounded-full bg-muted overflow-hidden">
                                        <div
                                            class="h-full rounded-full {memBarClass(pct)}"
                                            style="width: {pct}%"
                                        ></div>
                                    </div>
                                    <span class="text-sm font-mono text-muted-foreground">
                                        {formatBytes(container.memory_used_bytes)}<span class="text-foreground/50">/{formatBytes(container.memory_limit_bytes)}</span>
                                    </span>
                                </div>
                            {:else}
                                <span class="text-sm font-mono text-muted-foreground">{formatBytes(container.memory_used_bytes)}</span>
                            {/if}
                        </td>
                        <td class="px-4 py-3 whitespace-nowrap text-sm font-mono text-muted-foreground">
                            ↓ {formatRate(container.network_rx_bytes_per_sec, networkUnit)}
                            ↑ {formatRate(container.network_tx_bytes_per_sec, networkUnit)}
                        </td>
                        <td class="px-4 py-3">
                            {#if portList.length > 0}
                                <div class="flex flex-col gap-0.5">
                                    {#each portList as port}
                                        <span class="text-sm font-mono text-muted-foreground">{port}</span>
                                    {/each}
                                </div>
                            {:else}
                                <span class="text-sm text-muted-foreground">—</span>
                            {/if}
                        </td>
                    </tr>
                {:else}
                    <tr>
                        <td colspan="9" class="px-4 py-12 text-center text-sm text-muted-foreground">
                            No matching containers
                        </td>
                    </tr>
                {/each}
            </tbody>
        </table>
    </div>
</div>
