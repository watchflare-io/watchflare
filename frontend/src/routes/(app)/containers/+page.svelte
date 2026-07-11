<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import * as api from '$lib/api.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import type { GlobalContainer, ContainerLiveness } from '$lib/types';
	import {
		formatBytes,
		parsePortBadges,
		cpuBarClass,
		memBarClass,
		healthBadgeClass,
		memoryPercent,
		containerIsLive,
		filterContainers
	} from '$lib/utils';
	import { formatRate } from '$lib/chart-utils';
	import { userStore } from '$lib/stores/user';
	import ContainerDetailDrawer from '$lib/components/host/ContainerDetailDrawer.svelte';
	import HostStatusDot from '$lib/components/HostStatusDot.svelte';
	import { Box, Activity, Clock, ChevronDown, Filter, X } from 'lucide-svelte';

	const REFRESH_MS = 60_000;

	const networkUnit = $derived($userStore.user?.network_unit ?? 'bytes');

	let containers: GlobalContainer[] = $state([]);
	let initialLoading = $state(true);
	let tableLoading = $state(false);
	let error = $state('');

	let searchQuery = $state('');
	let hostFilter = $state('');
	let runtimeFilter = $state('');
	let livenessFilter: ContainerLiveness = $state('all');

	let drawerOpen = $state(false);
	let selected: GlobalContainer | null = $state(null);

	let refreshTimer: ReturnType<typeof setInterval> | null = null;

	const hosts = $derived(
		[...new Map(containers.map((c) => [c.host_id, c.host_name])).entries()].sort((a, b) =>
			a[1].localeCompare(b[1])
		)
	);
	const runtimes = $derived([...new Set(containers.map((c) => c.runtime).filter(Boolean))].sort());

	const displayed = $derived(
		filterContainers(containers, {
			search: searchQuery,
			host: hostFilter,
			runtime: runtimeFilter,
			liveness: livenessFilter
		})
	);

	const total = $derived(containers.length);
	const liveCount = $derived(containers.filter((c) => containerIsLive(c.host_status)).length);
	const staleCount = $derived(total - liveCount);

	const hasActiveFilters = $derived(
		searchQuery !== '' || hostFilter !== '' || runtimeFilter !== '' || livenessFilter !== 'all'
	);

	async function loadData(isInitial = false) {
		if (isInitial) initialLoading = true;
		else tableLoading = true;
		error = '';
		try {
			const resp = await api.listContainers();
			containers = resp.containers ?? [];
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Failed to load containers';
		} finally {
			initialLoading = false;
			tableLoading = false;
		}
	}

	function clearFilters() {
		searchQuery = '';
		hostFilter = '';
		runtimeFilter = '';
		livenessFilter = 'all';
	}

	function openDrawer(c: GlobalContainer) {
		selected = c;
		drawerOpen = true;
	}

	function relativeAge(iso: string): string {
		const diff = Date.now() - new Date(iso).getTime();
		const s = Math.max(0, Math.floor(diff / 1000));
		if (s < 60) return `${s}s ago`;
		const m = Math.floor(s / 60);
		if (m < 60) return `${m}m ago`;
		const h = Math.floor(m / 60);
		if (h < 24) return `${h}h ago`;
		return `${Math.floor(h / 24)}d ago`;
	}

	function truncateImage(image: string): string {
		if (image.length <= 50) return image;
		return image.substring(0, 47) + '…';
	}

	onMount(() => {
		loadData(true);
		refreshTimer = setInterval(() => loadData(false), REFRESH_MS);
	});

	onDestroy(() => {
		if (refreshTimer) clearInterval(refreshTimer);
	});
</script>

<svelte:head>
	<title>Containers - Watchflare</title>
</svelte:head>

<div class="mb-6">
	<h1 class="text-xl font-semibold text-foreground">Containers</h1>
	<p class="text-sm text-muted-foreground mt-0.5">Running containers across all hosts</p>
</div>

{#if error}
	<div role="alert" class="mb-6 rounded-lg border border-destructive bg-destructive/10 p-4">
		<p class="text-sm text-destructive">{error}</p>
	</div>
{/if}

<!-- Stat cards -->
<div class="mb-6 grid grid-cols-3 gap-3 lg:gap-4">
	<div class="flex items-center gap-2.5 rounded-lg border bg-card px-3 py-2.5">
		<div
			class="flex items-center justify-center rounded-md bg-primary/10 text-primary h-8 w-8 shrink-0"
		>
			<Box class="h-4 w-4" />
		</div>
		<div class="min-w-0">
			<p class="text-xs text-muted-foreground truncate">Containers</p>
			<p class="text-sm font-semibold text-foreground">{total.toLocaleString()}</p>
		</div>
	</div>
	<div class="flex items-center gap-2.5 rounded-lg border bg-card px-3 py-2.5">
		<div
			class="flex items-center justify-center rounded-md bg-success/10 text-success h-8 w-8 shrink-0"
		>
			<Activity class="h-4 w-4" />
		</div>
		<div class="min-w-0">
			<p class="text-xs text-muted-foreground truncate">Live</p>
			<p class="text-sm font-semibold text-foreground">{liveCount.toLocaleString()}</p>
		</div>
	</div>
	<div class="flex items-center gap-2.5 rounded-lg border bg-card px-3 py-2.5">
		<div
			class="flex items-center justify-center rounded-md h-8 w-8 shrink-0 {staleCount > 0
				? 'bg-warning/10 text-warning'
				: 'bg-muted text-muted-foreground'}"
		>
			<Clock class="h-4 w-4" />
		</div>
		<div class="min-w-0">
			<p class="text-xs text-muted-foreground truncate">Stale</p>
			<p class="text-sm font-semibold {staleCount > 0 ? 'text-warning' : 'text-foreground'}">
				{staleCount.toLocaleString()}
			</p>
		</div>
	</div>
</div>

<!-- Search & filters -->
<div class="mb-4 flex items-center gap-2 flex-wrap">
	<input
		type="text"
		bind:value={searchQuery}
		placeholder="Search name or image..."
		class="flex-1 min-w-48 h-9 rounded-lg border bg-card px-3 text-sm text-foreground placeholder:text-sm placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
	/>

	<!-- Host filter -->
	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<button
					type="button"
					{...props}
					class="inline-flex items-center gap-1.5 h-9 rounded-lg border px-3 text-sm font-medium transition-colors whitespace-nowrap {hostFilter
						? 'border-primary/40 bg-primary/5 text-primary hover:bg-primary/10'
						: 'bg-card text-muted-foreground hover:bg-muted hover:text-foreground'}"
				>
					<Filter class="h-3.5 w-3.5 shrink-0" />
					<span class="hidden sm:inline"
						>{hostFilter
							? (hosts.find((h) => h[0] === hostFilter)?.[1] ?? 'Host')
							: 'All hosts'}</span
					>
					<ChevronDown class="hidden sm:inline-block h-3 w-3 opacity-40" />
				</button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start">
			<div class="max-h-48 overflow-y-auto">
				<DropdownMenu.Item onclick={() => (hostFilter = '')}>All hosts</DropdownMenu.Item>
				{#each hosts as [id, name] (id)}
					<DropdownMenu.Item onclick={() => (hostFilter = id)}>{name}</DropdownMenu.Item>
				{/each}
			</div>
		</DropdownMenu.Content>
	</DropdownMenu.Root>

	<!-- Runtime filter -->
	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<button
					type="button"
					{...props}
					class="inline-flex items-center gap-1.5 h-9 rounded-lg border px-3 text-sm font-medium transition-colors whitespace-nowrap {runtimeFilter
						? 'border-primary/40 bg-primary/5 text-primary hover:bg-primary/10'
						: 'bg-card text-muted-foreground hover:bg-muted hover:text-foreground'}"
				>
					<Filter class="h-3.5 w-3.5 shrink-0" />
					<span class="hidden sm:inline">{runtimeFilter || 'All runtimes'}</span>
					<ChevronDown class="hidden sm:inline-block h-3 w-3 opacity-40" />
				</button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start">
			<DropdownMenu.Item onclick={() => (runtimeFilter = '')}>All runtimes</DropdownMenu.Item>
			{#each runtimes as rt (rt)}
				<DropdownMenu.Item onclick={() => (runtimeFilter = rt)}>{rt}</DropdownMenu.Item>
			{/each}
		</DropdownMenu.Content>
	</DropdownMenu.Root>

	<!-- Liveness filter -->
	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<button
					type="button"
					{...props}
					class="inline-flex items-center gap-1.5 h-9 rounded-lg border px-3 text-sm font-medium transition-colors whitespace-nowrap {livenessFilter !==
					'all'
						? 'border-primary/40 bg-primary/5 text-primary hover:bg-primary/10'
						: 'bg-card text-muted-foreground hover:bg-muted hover:text-foreground'}"
				>
					<Activity class="h-3.5 w-3.5 shrink-0" />
					<span class="hidden sm:inline"
						>{livenessFilter === 'all'
							? 'All states'
							: livenessFilter === 'live'
								? 'Live'
								: 'Stale'}</span
					>
					<ChevronDown class="hidden sm:inline-block h-3 w-3 opacity-40" />
				</button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start">
			<DropdownMenu.Item onclick={() => (livenessFilter = 'all')}>All states</DropdownMenu.Item>
			<DropdownMenu.Item onclick={() => (livenessFilter = 'live')}>Live</DropdownMenu.Item>
			<DropdownMenu.Item onclick={() => (livenessFilter = 'stale')}>Stale</DropdownMenu.Item>
		</DropdownMenu.Content>
	</DropdownMenu.Root>

	{#if hasActiveFilters}
		<button
			type="button"
			onclick={clearFilters}
			class="inline-flex items-center gap-1.5 h-9 rounded-lg border px-3 text-sm font-medium transition-colors whitespace-nowrap bg-card text-muted-foreground hover:bg-muted hover:text-foreground"
			aria-label="Clear all filters"
		>
			<X class="h-3.5 w-3.5 shrink-0" />
			<span class="hidden sm:inline">Clear filters</span>
		</button>
	{/if}
</div>

<ContainerDetailDrawer
	container={selected}
	open={drawerOpen}
	onClose={() => (drawerOpen = false)}
	hostHref={selected ? `/hosts/${selected.host_id}` : undefined}
	hostName={selected?.host_name}
	hostStatus={selected?.host_status}
/>

<div class="rounded-xl border bg-card overflow-hidden mb-2">
	{#if initialLoading}
		<div class="flex items-center justify-center py-20">
			<p class="text-muted-foreground">Loading containers...</p>
		</div>
	{:else}
		<!-- Mobile cards -->
		<div
			class="md:hidden p-3 flex flex-col gap-2 {tableLoading
				? 'opacity-50 pointer-events-none'
				: ''}"
		>
			{#each displayed as c (c.host_id + c.container_id)}
				{@const pct = memoryPercent(c.memory_used_bytes, c.memory_limit_bytes)}
				<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
				<div
					class="rounded-lg border bg-card cursor-pointer"
					onclick={() => openDrawer(c)}
					onkeydown={(e) =>
						(e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), openDrawer(c))}
					role="button"
					tabindex="0"
				>
					<div
						class="rounded-t-lg bg-table-header px-4 py-2.5 border-b border-border flex items-center justify-between gap-2"
					>
						<div class="flex items-center gap-2 min-w-0">
							<span class="text-sm font-medium text-foreground truncate">{c.container_name}</span>
						</div>
					</div>
					<div class="px-4 py-2.5 flex flex-col gap-1">
						<div class="flex items-center gap-2">
							<HostStatusDot
								status={c.host_status}
								title={`${c.host_status} · ${relativeAge(c.updated_at)}`}
							/>
							<a
								href={`/hosts/${c.host_id}`}
								onclick={(e) => e.stopPropagation()}
								class="text-xs text-primary hover:underline w-fit focus-visible:ring-2 focus-visible:ring-primary/50 rounded"
								>{c.host_name}</a
							>
						</div>
						{#if c.image}
							<p class="text-xs text-muted-foreground truncate" title={c.image}>
								{truncateImage(c.image)}
							</p>
						{/if}
						<div class="flex items-center gap-2">
							<span class="w-16 shrink-0 text-xs text-muted-foreground">CPU</span>
							<div class="flex-1 h-2.5 rounded-full bg-muted">
								<div
									class="h-full rounded-full {cpuBarClass(c.cpu_percent)}"
									style="width: {Math.min(100, c.cpu_percent)}%"
								></div>
							</div>
							<span class="w-12 text-sm text-muted-foreground text-left shrink-0"
								>{c.cpu_percent.toFixed(1)}%</span
							>
						</div>
						<div class="flex items-center gap-2">
							<span class="w-16 shrink-0 text-xs text-muted-foreground">Memory</span>
							{#if c.memory_limit_bytes > 0}
								<div class="flex-1 h-2.5 rounded-full bg-muted">
									<div class="h-full rounded-full {memBarClass(pct)}" style="width: {pct}%"></div>
								</div>
							{/if}
							<span class="w-12 text-sm text-muted-foreground text-left shrink-0"
								>{formatBytes(c.memory_used_bytes)}</span
							>
						</div>
					</div>
				</div>
			{:else}
				<div class="py-16 text-center text-sm text-muted-foreground">No matching containers</div>
			{/each}
		</div>

		<!-- Desktop table -->
		<div class="hidden md:block overflow-auto max-h-[65vh]">
			<table class="w-full min-w-[1000px]">
				<thead>
					<tr
						class="bg-table-header sticky top-0 z-10 [box-shadow:0_1px_0_var(--border)] whitespace-nowrap"
					>
						{#each ['Name', 'Host', 'Status', 'Health', 'Image', 'CPU', 'Memory', 'Network', 'Ports'] as label}
							<th
								scope="col"
								class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
							>
								{label}
							</th>
						{/each}
					</tr>
				</thead>
				<tbody class="divide-y {tableLoading ? 'opacity-50 pointer-events-none' : ''}">
					{#each displayed as c (c.host_id + c.container_id)}
						{@const pct = memoryPercent(c.memory_used_bytes, c.memory_limit_bytes)}
						{@const badges = parsePortBadges(c.ports ?? '')}
						{@const extraPorts = badges.length > 2 ? badges.length - 2 : 0}
						<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
						<tr
							class="hover:bg-muted/20 transition-colors cursor-pointer"
							onclick={() => openDrawer(c)}
							onkeydown={(e) =>
								(e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), openDrawer(c))}
							tabindex="0"
							role="button"
						>
							<td class="px-4 py-3 whitespace-nowrap">
								<span class="text-sm font-medium text-foreground">{c.container_name}</span>
							</td>
							<td class="px-4 py-3 whitespace-nowrap">
								<div class="flex items-center gap-2">
									<HostStatusDot
										status={c.host_status}
										title={`${c.host_status} · ${relativeAge(c.updated_at)}`}
									/>
									<a
										href={`/hosts/${c.host_id}`}
										onclick={(e) => e.stopPropagation()}
										class="text-sm text-primary hover:underline focus-visible:ring-2 focus-visible:ring-primary/50 rounded"
										>{c.host_name}</a
									>
								</div>
							</td>
							<td class="px-4 py-3 whitespace-nowrap text-sm text-muted-foreground">
								{c.status || '-'}
							</td>
							<td class="px-4 py-3 whitespace-nowrap">
								<span
									class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium border {healthBadgeClass(
										c.health ?? ''
									)}"
								>
									{c.health || 'None'}
								</span>
							</td>
							<td class="px-4 py-3 max-w-xs">
								<span class="text-sm text-muted-foreground truncate block" title={c.image}>
									{c.image ? truncateImage(c.image) : '-'}
								</span>
							</td>
							<td class="px-4 py-3 text-center">
								<div class="flex flex-col items-center">
									<span class="text-sm text-muted-foreground">{c.cpu_percent.toFixed(1)}%</span>
									<div class="w-16 h-1.5 rounded-full bg-muted mt-1">
										<div
											class="h-full rounded-full {cpuBarClass(c.cpu_percent)}"
											style="width: {Math.min(100, c.cpu_percent)}%"
										></div>
									</div>
								</div>
							</td>
							<td class="px-4 py-3 text-center whitespace-nowrap">
								<div class="flex flex-col items-center">
									<span class="text-sm text-muted-foreground"
										>{formatBytes(c.memory_used_bytes)}</span
									>
									{#if c.memory_limit_bytes > 0}
										<div class="w-20 h-1.5 rounded-full bg-muted mt-1">
											<div
												class="h-full rounded-full {memBarClass(pct)}"
												style="width: {pct}%"
											></div>
										</div>
									{/if}
								</div>
							</td>
							<td class="px-4 py-3 whitespace-nowrap text-sm font-mono text-muted-foreground">
								&#8595; {formatRate(c.network_rx_bytes_per_sec, networkUnit)}
								&#8593; {formatRate(c.network_tx_bytes_per_sec, networkUnit)}
							</td>
							<td class="px-4 py-3 whitespace-nowrap">
								{#if badges.length > 0}
									<div class="flex items-center gap-1">
										{#each badges.slice(0, 2) as badge (badge)}
											<span
												class="inline-flex items-center rounded px-1.5 py-0.5 text-xs font-mono bg-muted text-muted-foreground"
												>{badge}</span
											>
										{/each}
										{#if extraPorts > 0}
											<span
												class="inline-flex items-center rounded px-1.5 py-0.5 text-xs font-mono bg-muted text-muted-foreground"
												>+{extraPorts}</span
											>
										{/if}
									</div>
								{:else}
									<span class="text-sm text-muted-foreground">-</span>
								{/if}
							</td>
						</tr>
					{:else}
						<tr>
							<td colspan="9" class="px-4 py-16 text-center text-sm text-muted-foreground">
								No matching containers
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>
