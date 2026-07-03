<script lang="ts">
	import { onMount, onDestroy, getContext } from 'svelte';
	import { page } from '$app/stores';
	import * as api from '$lib/api.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import type { Host, SSEEvent, Service, ServiceHealthUpdate } from '$lib/types';
	import ServiceStateBadge from '$lib/components/ServiceStateBadge.svelte';
	import EnabledStateBadge from '$lib/components/EnabledStateBadge.svelte';
	import { formatDateTime } from '$lib/utils';
	import { userStore } from '$lib/stores/user';
	import { Columns3, ChevronDown } from 'lucide-svelte';

	const timeFormat = $derived(($userStore.user?.time_format ?? '24h') as '12h' | '24h');

	const ALL_COLUMNS = [
		{ key: 'name', label: 'Name', defaultVisible: true },
		{ key: 'enabled', label: 'Enabled', defaultVisible: true },
		{ key: 'state', label: 'State', defaultVisible: true },
		{ key: 'substate', label: 'Substate', defaultVisible: true },
		{ key: 'description', label: 'Description', defaultVisible: false },
		{ key: 'updated', label: 'Updated', defaultVisible: true }
	] as const;

	type ColumnKey = (typeof ALL_COLUMNS)[number]['key'];

	const DEFAULT_VISIBLE_COLUMNS = ALL_COLUMNS.filter((c) => c.defaultVisible).map(
		(c) => c.key
	) as ColumnKey[];

	const COLUMNS_STORAGE_KEY = 'wf_svc_columns';

	function loadStoredColumns(): ColumnKey[] {
		try {
			const stored = localStorage.getItem(COLUMNS_STORAGE_KEY);
			if (stored) {
				const parsed = JSON.parse(stored) as string[];
				const valid = parsed.filter((k) => ALL_COLUMNS.some((c) => c.key === k));
				if (valid.length > 0) return valid as ColumnKey[];
			}
		} catch {}
		return DEFAULT_VISIBLE_COLUMNS;
	}

	const hostId = $derived($page.params.id);

	const ctx = getContext<{
		host: Host | null;
		subscribeToSSE: (cb: (e: SSEEvent) => void) => () => void;
	}>('hostDetail');

	let services = $state<Service[]>([]);
	let summary = $state({ total: 0, failed: 0 });
	let loading = $state(true);
	let error = $state('');
	let searchTerm = $state('');
	let visibleColumns = $state<Set<ColumnKey>>(new Set(loadStoredColumns()));

	const col = $derived((key: ColumnKey) => visibleColumns.has(key));
	const extraColumnsCount = $derived(
		[...visibleColumns].filter((k) => !DEFAULT_VISIBLE_COLUMNS.includes(k)).length
	);

	const STALE_MS = 90_000;

	function isStale(s: Service): boolean {
		return Date.now() - new Date(s.collected_at).getTime() > STALE_MS;
	}

	function toggleColumn(key: ColumnKey) {
		const next = new Set(visibleColumns);
		if (next.has(key)) next.delete(key);
		else next.add(key);
		visibleColumns = next;
		try {
			localStorage.setItem(COLUMNS_STORAGE_KEY, JSON.stringify([...next]));
		} catch {}
	}

	function applyHealth(u: ServiceHealthUpdate) {
		if (u.host_id !== hostId) return;
		const now = new Date().toISOString();
		const byName = new Map(u.services.map((s) => [s.name, s]));
		services = services
			.filter((s) => byName.has(s.name))
			.map((s) => {
				const h = byName.get(s.name)!;
				return { ...s, active_state: h.active_state, sub_state: h.sub_state, collected_at: now };
			});
		summary = {
			total: services.length,
			failed: services.filter((s) => s.active_state === 'failed').length
		};
	}

	type SortColumn = 'name' | 'enabled' | 'state' | 'substate' | 'updated';
	let sortColumn = $state<SortColumn>('name');
	let sortOrder = $state<'asc' | 'desc'>('asc');

	function stateRank(state: string): number {
		switch (state) {
			case 'failed':
				return 0;
			case 'activating':
			case 'deactivating':
			case 'reloading':
				return 1;
			case 'inactive':
				return 2;
			case 'active':
				return 3;
			default:
				return 4;
		}
	}

	function handleSort(column: SortColumn) {
		if (sortColumn === column) {
			sortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
		} else {
			sortColumn = column;
			sortOrder = column === 'updated' ? 'desc' : 'asc';
		}
	}

	const displayServices = $derived(
		services
			.filter((s) => {
				const q = searchTerm.trim().toLowerCase();
				if (!q) return true;
				return s.name.toLowerCase().includes(q) || s.description.toLowerCase().includes(q);
			})
			.sort((a, b) => {
				let r = 0;
				switch (sortColumn) {
					case 'name':
						r = a.name.localeCompare(b.name);
						break;
					case 'enabled':
						r = a.enabled_state.localeCompare(b.enabled_state);
						break;
					case 'state':
						r = stateRank(a.active_state) - stateRank(b.active_state);
						break;
					case 'substate':
						r = a.sub_state.localeCompare(b.sub_state);
						break;
					case 'updated':
						r = new Date(a.collected_at).getTime() - new Date(b.collected_at).getTime();
						break;
				}
				if (r !== 0) return sortOrder === 'asc' ? r : -r;
				return a.name.localeCompare(b.name);
			})
	);

	async function load() {
		loading = true;
		error = '';
		try {
			const res = await api.getHostServices(hostId);
			services = res.services;
			summary = res.summary;
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Failed to load services';
		} finally {
			loading = false;
		}
	}

	let unsub: (() => void) | undefined;

	onMount(() => {
		load();
		unsub = ctx.subscribeToSSE((event) => {
			if (event.type === 'service_health_update') {
				applyHealth(event.data as ServiceHealthUpdate);
			}
		});
	});

	onDestroy(() => unsub?.());
</script>

<svelte:head>
	<title>Services{ctx.host ? ` - ${ctx.host.display_name}` : ''} - Watchflare</title>
</svelte:head>

{#if error}
	<div role="alert" class="mb-6 rounded-lg border border-destructive bg-destructive/10 p-4">
		<p class="text-sm text-destructive">{error}</p>
	</div>
{/if}

<!-- Summary -->
{#if loading}
	<div class="mb-4 flex animate-pulse items-center gap-4">
		<div class="h-4 w-40 rounded bg-muted"></div>
		<div class="h-4 w-20 rounded bg-muted"></div>
	</div>
{:else}
	<div class="mb-4 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
		<span class="text-muted-foreground">
			<span class="font-medium text-foreground">{summary.total}</span> systemd services
		</span>
		{#if summary.failed > 0}
			<span class="text-destructive"><span class="font-medium">{summary.failed}</span> failed</span>
		{/if}
	</div>
{/if}

<!-- Search & column config -->
<div class="mb-4 flex flex-wrap items-center gap-2">
	<input
		type="text"
		bind:value={searchTerm}
		placeholder="Search services..."
		class="h-9 min-w-0 flex-1 rounded-lg border bg-card px-3 text-sm text-foreground placeholder:text-sm placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
	/>

	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<button
					type="button"
					{...props}
					class="hidden h-9 items-center gap-1.5 whitespace-nowrap rounded-lg border px-3 text-sm font-medium transition-colors md:inline-flex
						{extraColumnsCount > 0
						? 'border-primary/40 bg-primary/5 text-primary hover:bg-primary/10'
						: 'bg-card text-muted-foreground hover:bg-muted hover:text-foreground'}"
				>
					<Columns3 class="h-3.5 w-3.5" />
					<span class="hidden sm:inline">Columns</span>
					{#if extraColumnsCount > 0}
						<span
							class="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-primary/15 px-1 text-xs font-medium text-primary"
						>
							+{extraColumnsCount}
						</span>
					{/if}
					<ChevronDown class="hidden h-3 w-3 opacity-40 sm:inline-block" />
				</button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start">
			{#each ALL_COLUMNS as column}
				<DropdownMenu.Item closeOnSelect={false} onclick={() => toggleColumn(column.key)}>
					<div
						class="flex h-4 w-4 shrink-0 items-center justify-center rounded border
							{visibleColumns.has(column.key) ? 'border-primary bg-primary' : 'border-muted-foreground/40'}"
					>
						{#if visibleColumns.has(column.key)}
							<svg
								class="h-3 w-3 text-primary-foreground"
								fill="none"
								stroke="currentColor"
								viewBox="0 0 24 24"
							>
								<path
									stroke-linecap="round"
									stroke-linejoin="round"
									stroke-width="3"
									d="M5 13l4 4L19 7"
								/>
							</svg>
						{/if}
					</div>
					<span>{column.label}</span>
				</DropdownMenu.Item>
			{/each}
		</DropdownMenu.Content>
	</DropdownMenu.Root>
</div>

{#snippet sortIcon(column: SortColumn)}
	{#if sortColumn === column}
		<svg class="h-3 w-3 shrink-0" viewBox="0 0 12 12" fill="currentColor">
			{#if sortOrder === 'asc'}
				<path d="M6 2l4 5H2z" />
			{:else}
				<path d="M6 10l4-5H2z" />
			{/if}
		</svg>
	{:else}
		<svg
			class="h-3 w-3 shrink-0 opacity-40 transition-opacity group-hover:opacity-100"
			viewBox="0 0 12 12"
			fill="currentColor"
		>
			<path d="M6 10l4-5H2z" />
		</svg>
	{/if}
{/snippet}

<!-- Table / Cards -->
<div class="mb-2 overflow-hidden rounded-xl border bg-card">
	{#if loading}
		<div class="hidden animate-pulse md:block">
			<div class="flex gap-6 bg-table-header px-4 py-2.5 [box-shadow:0_1px_0_var(--border)]">
				<div class="h-4 w-32 rounded bg-muted"></div>
				<div class="h-4 w-20 rounded bg-muted"></div>
				<div class="h-4 w-20 rounded bg-muted"></div>
			</div>
			{#each Array(8) as _}
				<div class="flex items-center gap-6 border-b px-4 py-3">
					<div class="h-4 w-40 rounded bg-muted"></div>
					<div class="h-5 w-16 rounded-full bg-muted"></div>
					<div class="h-5 w-14 rounded-full bg-muted"></div>
					<div class="h-4 w-24 rounded bg-muted"></div>
				</div>
			{/each}
		</div>
		<div class="flex flex-col gap-2 p-3 md:hidden">
			{#each Array(5) as _}
				<div class="h-16 animate-pulse rounded-lg border bg-muted/40"></div>
			{/each}
		</div>
	{:else}
		<!-- Desktop table -->
		<div class="hidden max-h-[65vh] overflow-auto md:block">
			<table class="w-full">
				<thead>
					<tr
						class="sticky top-0 z-10 whitespace-nowrap bg-table-header [box-shadow:0_1px_0_var(--border)]"
					>
						{#if col('name')}
							<th
								scope="col"
								class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
							>
								<button
									type="button"
									onclick={() => handleSort('name')}
									class="group inline-flex h-8 cursor-pointer select-none items-center gap-1 rounded-md px-2.5 transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
									'name'
										? 'bg-table-header-active text-foreground'
										: ''}">Name {@render sortIcon('name')}</button
								>
							</th>
						{/if}
						{#if col('enabled')}
							<th
								scope="col"
								class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
							>
								<button
									type="button"
									onclick={() => handleSort('enabled')}
									class="group inline-flex h-8 cursor-pointer select-none items-center gap-1 rounded-md px-2.5 transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
									'enabled'
										? 'bg-table-header-active text-foreground'
										: ''}">Enabled {@render sortIcon('enabled')}</button
								>
							</th>
						{/if}
						{#if col('state')}
							<th
								scope="col"
								class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
							>
								<button
									type="button"
									onclick={() => handleSort('state')}
									class="group inline-flex h-8 cursor-pointer select-none items-center gap-1 rounded-md px-2.5 transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
									'state'
										? 'bg-table-header-active text-foreground'
										: ''}">State {@render sortIcon('state')}</button
								>
							</th>
						{/if}
						{#if col('substate')}
							<th
								scope="col"
								class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
							>
								<button
									type="button"
									onclick={() => handleSort('substate')}
									class="group inline-flex h-8 cursor-pointer select-none items-center gap-1 rounded-md px-2.5 transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
									'substate'
										? 'bg-table-header-active text-foreground'
										: ''}">Substate {@render sortIcon('substate')}</button
								>
							</th>
						{/if}
						{#if col('description')}
							<th
								scope="col"
								class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
								>Description</th
							>
						{/if}
						{#if col('updated')}
							<th
								scope="col"
								class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
							>
								<button
									type="button"
									onclick={() => handleSort('updated')}
									class="group inline-flex h-8 cursor-pointer select-none items-center gap-1 rounded-md px-2.5 transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
									'updated'
										? 'bg-table-header-active text-foreground'
										: ''}">Updated {@render sortIcon('updated')}</button
								>
							</th>
						{/if}
					</tr>
				</thead>
				<tbody class="divide-y divide-border">
					{#each displayServices as s (s.name)}
						<tr class="transition-colors hover:bg-muted/20">
							{#if col('name')}
								<td class="px-4 py-3">
									<span class="font-mono text-sm text-foreground" title={s.description || undefined}
										>{s.name}</span
									>
								</td>
							{/if}
							{#if col('enabled')}
								<td class="w-px whitespace-nowrap px-4 py-3"
									><EnabledStateBadge state={s.enabled_state} /></td
								>
							{/if}
							{#if col('state')}
								<td class="w-px whitespace-nowrap px-4 py-3"
									><ServiceStateBadge state={s.active_state} /></td
								>
							{/if}
							{#if col('substate')}
								<td class="whitespace-nowrap px-4 py-3 text-sm text-muted-foreground"
									>{s.sub_state}</td
								>
							{/if}
							{#if col('description')}
								<td
									class="max-w-xs truncate px-4 py-3 text-sm text-muted-foreground"
									title={s.description}>{s.description}</td
								>
							{/if}
							{#if col('updated')}
								<td
									class="w-px whitespace-nowrap px-4 py-3 text-left text-sm text-muted-foreground"
								>
									{formatDateTime(s.collected_at, timeFormat)}
									{#if isStale(s)}<span class="ml-1 text-warning">(stale)</span>{/if}
								</td>
							{/if}
						</tr>
					{:else}
						<tr>
							<td colspan={visibleColumns.size} class="py-16 text-center">
								<p class="text-sm text-muted-foreground">
									{searchTerm.trim() ? 'No services match your search' : 'No services found'}
								</p>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<!-- Mobile cards -->
		<div class="flex flex-col gap-2 p-3 md:hidden">
			{#each displayServices as s (s.name)}
				<div class="rounded-lg border bg-card">
					<div
						class="flex items-center justify-between gap-2 rounded-t-lg border-b border-border bg-table-header px-4 py-2.5"
					>
						<span class="break-all font-mono text-sm font-medium text-foreground">{s.name}</span>
						<ServiceStateBadge state={s.active_state} />
					</div>
					<div class="flex flex-wrap items-center gap-3 px-4 py-2.5">
						<EnabledStateBadge state={s.enabled_state} />
						{#if s.sub_state}
							<span class="text-xs text-muted-foreground">{s.sub_state}</span>
						{/if}
						<span class="text-xs text-muted-foreground">
							{formatDateTime(s.collected_at, timeFormat)}
							{#if isStale(s)}<span class="text-warning">(stale)</span>{/if}
						</span>
					</div>
				</div>
			{:else}
				<div class="py-16 text-center">
					<p class="text-sm text-muted-foreground">
						{searchTerm.trim() ? 'No services match your search' : 'No services found'}
					</p>
				</div>
			{/each}
		</div>
	{/if}
</div>
