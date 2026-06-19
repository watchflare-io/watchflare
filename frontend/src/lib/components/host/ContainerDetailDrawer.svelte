<script lang="ts">
	import { X } from 'lucide-svelte';
	import { formatBytes } from '$lib/utils';
	import { formatRate } from '$lib/chart-utils';
	import { userStore } from '$lib/stores/user';
	import type { ContainerMetric } from '$lib/types';
	import RightSidebar from '$lib/components/RightSidebar.svelte';
	import RuntimeIcon from '$lib/components/icons/RuntimeIcon.svelte';

	const {
		container,
		open,
		onClose
	}: {
		container: ContainerMetric | null;
		open: boolean;
		onClose: () => void;
	} = $props();

	const networkUnit = $derived($userStore.user?.network_unit ?? 'bytes');

	const memPct = $derived(
		container && container.memory_limit_bytes > 0
			? Math.min(100, (container.memory_used_bytes / container.memory_limit_bytes) * 100)
			: 0
	);

	const portList = $derived(container?.ports ? container.ports.split(', ').filter(Boolean) : []);

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
</script>

<RightSidebar {open} {onClose} size="wide">
	<!-- Header -->
	<div class="flex items-center justify-between border-b px-6 py-4 shrink-0 gap-3 min-w-0">
		<div class="flex items-center gap-2 min-w-0">
			{#if container}
				<RuntimeIcon runtime={container.runtime} class="h-4 w-4 shrink-0 text-muted-foreground" />
				<h2 class="text-base font-semibold text-foreground truncate">
					{container.container_name}
				</h2>
			{/if}
		</div>
		<button
			type="button"
			onclick={onClose}
			class="shrink-0 flex h-8 w-8 items-center justify-center rounded-lg text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
			aria-label="Close"
		>
			<X class="h-4 w-4" />
		</button>
	</div>

	<!-- Content -->
	{#if container}
		<div class="flex-1 overflow-y-auto p-6 flex flex-col gap-5">
			<!-- Status & Health -->
			<div class="flex flex-col gap-1.5">
				<p class="text-xs font-medium text-muted-foreground uppercase tracking-wide">Status</p>
				<div class="flex items-center gap-2 flex-wrap">
					<span class="text-sm text-foreground">{container.status || '—'}</span>
					{#if container.health}
						<span
							class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium border {healthBadgeClass(
								container.health
							)}"
						>
							{container.health}
						</span>
					{/if}
				</div>
			</div>

			<!-- Image -->
			<div class="flex flex-col gap-1.5">
				<p class="text-xs font-medium text-muted-foreground uppercase tracking-wide">Image</p>
				<p class="text-sm font-mono text-foreground break-all">
					{container.image || '—'}
				</p>
			</div>

			<!-- Ports -->
			{#if portList.length > 0}
				<div class="flex flex-col gap-1.5">
					<p class="text-xs font-medium text-muted-foreground uppercase tracking-wide">Ports</p>
					<div class="flex flex-col gap-1">
						{#each portList as port}
							<span class="text-sm font-mono text-foreground">{port}</span>
						{/each}
					</div>
				</div>
			{/if}

			<!-- CPU -->
			<div class="flex flex-col gap-1.5">
				<p class="text-xs font-medium text-muted-foreground uppercase tracking-wide">CPU</p>
				<div class="flex items-center gap-3">
					<div class="flex-1 h-2 rounded-full bg-muted overflow-hidden">
						<div
							class="h-full rounded-full {cpuBarClass(container.cpu_percent)}"
							style="width: {Math.min(100, container.cpu_percent)}%"
						></div>
					</div>
					<span class="text-sm font-mono w-14 text-right">{container.cpu_percent.toFixed(1)}%</span>
				</div>
			</div>

			<!-- Memory -->
			<div class="flex flex-col gap-1.5">
				<p class="text-xs font-medium text-muted-foreground uppercase tracking-wide">Memory</p>
				<div class="flex items-center gap-3">
					<div class="flex-1 h-2 rounded-full bg-muted overflow-hidden">
						<div class="h-full rounded-full {memBarClass(memPct)}" style="width: {memPct}%"></div>
					</div>
					{#if container.memory_limit_bytes > 0}
						<span class="text-sm font-mono text-right whitespace-nowrap">
							{formatBytes(container.memory_used_bytes)}<span class="text-foreground/50"
								>/{formatBytes(container.memory_limit_bytes)}</span
							>
							<span class="text-muted-foreground">({memPct.toFixed(0)}%)</span>
						</span>
					{:else}
						<span class="text-sm font-mono">{formatBytes(container.memory_used_bytes)}</span>
					{/if}
				</div>
			</div>

			<!-- Network -->
			<div class="flex flex-col gap-1.5">
				<p class="text-xs font-medium text-muted-foreground uppercase tracking-wide">Network</p>
				<div class="flex gap-4 text-sm font-mono text-foreground">
					<span>↓ {formatRate(container.network_rx_bytes_per_sec, networkUnit)}</span>
					<span>↑ {formatRate(container.network_tx_bytes_per_sec, networkUnit)}</span>
				</div>
			</div>
		</div>
	{/if}
</RightSidebar>
