<script lang="ts">
    import { formatBytes } from '$lib/utils';
    import { formatRate } from '$lib/chart-utils';
    import { userStore } from '$lib/stores/user';
    import type { ContainerMetric } from '$lib/types';

    const { containerMetrics }: { containerMetrics: ContainerMetric[] } = $props();

    const networkUnit = $derived($userStore.user?.network_unit ?? 'bytes');

    // Derive the latest metric per container, preserving image and runtime
    // from any non-empty record (image/runtime are absent from SSE updates).
    const latestContainers = $derived((() => {
        type Entry = { metric: ContainerMetric; image: string; runtime: string };
        const byId = new Map<string, Entry>();

        for (const m of containerMetrics) {
            const entry = byId.get(m.container_id);
            if (!entry) {
                byId.set(m.container_id, {
                    metric: m,
                    image: m.image ?? '',
                    runtime: m.runtime ?? '',
                });
            } else if (m.timestamp > entry.metric.timestamp) {
                byId.set(m.container_id, {
                    metric: m,
                    image: m.image || entry.image,
                    runtime: m.runtime || entry.runtime,
                });
            } else {
                if (!entry.image && m.image) entry.image = m.image;
                if (!entry.runtime && m.runtime) entry.runtime = m.runtime;
            }
        }

        return [...byId.values()]
            .map(({ metric, image, runtime }) => ({ ...metric, image, runtime }))
            .sort((a, b) => a.container_name.localeCompare(b.container_name));
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

    function truncateImage(image: string): string {
        if (image.length <= 50) return image;
        return image.substring(0, 47) + '…';
    }
</script>

<div class="rounded-xl border bg-card overflow-hidden">
    <!-- Header -->
    <div class="bg-table-header px-4 py-2.5 border-b flex items-center gap-2">
        <h3 class="text-sm font-semibold">Containers</h3>
        <span class="inline-flex items-center justify-center rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
            {latestContainers.length}
        </span>
    </div>

    <!-- Mobile cards -->
    <div class="sm:hidden p-3 flex flex-col gap-2">
        {#each latestContainers as container (container.container_id)}
            {@const pct = memoryPercent(container)}
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
                </div>
            </div>
        {/each}
    </div>

    <!-- Desktop table -->
    <div class="hidden sm:block overflow-x-auto">
        <table class="w-full min-w-[640px]">
            <thead>
                <tr class="border-b bg-table-header whitespace-nowrap">
                    <th scope="col" class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">Runtime</th>
                    <th scope="col" class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">Name</th>
                    <th scope="col" class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">Image</th>
                    <th scope="col" class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">CPU</th>
                    <th scope="col" class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">Memory</th>
                    <th scope="col" class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">Network</th>
                </tr>
            </thead>
            <tbody class="divide-y">
                {#each latestContainers as container (container.container_id)}
                    {@const pct = memoryPercent(container)}
                    <tr class="hover:bg-muted/20 transition-colors whitespace-nowrap">
                        <td class="px-4 py-3">
                            {#if container.runtime}
                                <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium border {runtimeBadgeClass(container.runtime)}">
                                    {container.runtime}
                                </span>
                            {:else}
                                <span class="text-sm text-muted-foreground">—</span>
                            {/if}
                        </td>
                        <td class="px-4 py-3 text-sm font-medium text-foreground">
                            {container.container_name}
                        </td>
                        <td class="px-4 py-3 max-w-xs">
                            <span class="text-sm text-muted-foreground truncate block" title={container.image}>
                                {container.image ? truncateImage(container.image) : '—'}
                            </span>
                        </td>
                        <td class="px-4 py-3">
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
                        <td class="px-4 py-3">
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
                        <td class="px-4 py-3 text-sm font-mono text-muted-foreground">
                            ↓ {formatRate(container.network_rx_bytes_per_sec, networkUnit)}
                            ↑ {formatRate(container.network_tx_bytes_per_sec, networkUnit)}
                        </td>
                    </tr>
                {/each}
            </tbody>
        </table>
    </div>
</div>
