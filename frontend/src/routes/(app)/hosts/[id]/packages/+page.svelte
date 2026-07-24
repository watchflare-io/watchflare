<script lang="ts">
	import { onMount, onDestroy, getContext } from 'svelte';
	import { page } from '$app/stores';
	import * as api from '$lib/api.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { PACKAGES_PER_PAGE } from '$lib/constants';
	import type { Host, Package, PackageStats } from '$lib/types';
	import Pagination from '$lib/components/Pagination.svelte';
	import PackageStatusBadge from '$lib/components/PackageStatusBadge.svelte';
	import { getManagerLabel, getManagerColor, formatDateTime } from '$lib/utils';
	import { userStore } from '$lib/stores/user';
	import {
		Filter,
		Columns3,
		ChevronDown,
		RefreshCw,
		ShieldAlert,
		ArrowUp,
		Package as PackageIcon,
		Tag,
		X
	} from 'lucide-svelte';

	const timeFormat = $derived(($userStore.user?.time_format ?? '24h') as '12h' | '24h');

	const ALL_COLUMNS = [
		{ key: 'name', label: 'Name', defaultVisible: true },
		{ key: 'version', label: 'Version', defaultVisible: true },
		{ key: 'status', label: 'Status', defaultVisible: true },
		{ key: 'manager', label: 'Manager', defaultVisible: true },
		{
			key: 'latest_version',
			label: 'Latest Version',
			defaultVisible: true
		},
		{ key: 'arch', label: 'Architecture', defaultVisible: false },
		{ key: 'description', label: 'Description', defaultVisible: false },
		{ key: 'first_seen', label: 'First Seen', defaultVisible: false },
		{ key: 'last_seen', label: 'Last Seen', defaultVisible: true }
	] as const;

	type ColumnKey = (typeof ALL_COLUMNS)[number]['key'];

	const DEFAULT_VISIBLE_COLUMNS = ALL_COLUMNS.filter((c) => c.defaultVisible).map(
		(c) => c.key
	) as ColumnKey[];

	type StatusFilter = 'outdated' | 'security' | 'up_to_date' | 'not_checked';

	type PackagesCache = {
		packages: Package[];
		totalCount: number;
		totalPages: number;
		stats: PackageStats | null;
		searchTerm: string;
		allManagerKeys: string[];
		selectedManagers: string[];
		selectedStatuses: string[];
		sortColumn: string;
		sortOrder: 'asc' | 'desc';
		offset: number;
		limit: number;
		visibleColumns: string[];
	};
	const ctx = getContext<{
		host: Host | null;
		packagesCache: PackagesCache | null;
		setPackagesCache: (data: PackagesCache) => void;
		packageInventorySignal: number;
	}>('hostDetail');

	const cached = ctx.packagesCache;
	let packages: Package[] = $state(cached?.packages ?? []);
	let totalCount = $state(cached?.totalCount ?? 0);
	let totalPages = $state(cached?.totalPages ?? 1);
	let stats: PackageStats | null = $state(cached?.stats ?? null);
	let loading = $state(!cached);
	let tableLoading = $state(false);
	let error = $state('');
	let searchTerm = $state(cached?.searchTerm ?? '');
	let selectedManagers: Set<string> = $state(new Set(cached?.selectedManagers ?? []));
	let allManagerKeys: string[] = $state(cached?.allManagerKeys ?? []);
	const COLUMNS_STORAGE_KEY = 'wf_pkg_columns';
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
	let visibleColumns: Set<ColumnKey> = $state(
		new Set((cached?.visibleColumns ?? loadStoredColumns()) as ColumnKey[])
	);
	let offset = $state(cached?.offset ?? 0);
	const STATUS_LABELS: Record<StatusFilter, string> = {
		outdated: 'Outdated',
		security: 'Security update',
		up_to_date: 'Up to date',
		not_checked: 'Not checked'
	};
	let selectedStatuses: Set<StatusFilter> = $state(
		new Set((cached?.selectedStatuses ?? []) as StatusFilter[])
	);

	const PAGE_SIZE_OPTIONS = [25, 50, 100, 200];
	let limit = $state(cached?.limit ?? PACKAGES_PER_PAGE);
	let sortColumn = $state(cached?.sortColumn ?? 'name');
	let sortOrder = $state<'asc' | 'desc'>(cached?.sortOrder ?? 'asc');
	const hostId = $derived($page.params.id);

	const col = $derived((key: ColumnKey) => visibleColumns.has(key));

	const currentPage = $derived(limit > 0 ? Math.floor(offset / limit) + 1 : 1);
	const isStatusFiltered = $derived(selectedStatuses.size > 0);
	const statusFilterLabel = $derived(
		selectedStatuses.size === 0
			? 'All statuses'
			: selectedStatuses.size === 1
				? STATUS_LABELS[[...selectedStatuses][0]]
				: `${selectedStatuses.size} statuses`
	);
	const isFiltered = $derived(selectedManagers.size > 0);
	const filterLabel = $derived(
		!isFiltered
			? 'All packages'
			: selectedManagers.size === 1
				? getManagerLabel([...selectedManagers][0])
				: `${selectedManagers.size} managers`
	);
	// Badge on Columns button: count of extra columns beyond default
	const extraColumnsCount = $derived(
		[...visibleColumns].filter((k) => !DEFAULT_VISIBLE_COLUMNS.includes(k)).length
	);

	onMount(async () => {
		await loadData(!!cached);
	});

	onDestroy(() => {
		if (searchDebounce) clearTimeout(searchDebounce);
		if (collectErrorTimeout) clearTimeout(collectErrorTimeout);
	});

	// Reload silently when the Hub pushes a new package inventory.
	// seenSignal captures the value at mount time so we only react to signals
	// that arrive AFTER this component is rendered (not pre-existing ones).
	let seenSignal = ctx.packageInventorySignal;
	$effect(() => {
		const sig = ctx.packageInventorySignal;
		if (sig > seenSignal) {
			seenSignal = sig;
			awaitingInventory = false;
			try {
				sessionStorage.removeItem(COLLECT_SESSION_KEY);
			} catch {
				/* unavailable */
			}
			loadData(true);
		}
	});

	function saveToCache() {
		ctx.setPackagesCache({
			packages,
			totalCount,
			totalPages,
			stats,
			searchTerm,
			allManagerKeys,
			selectedManagers: [...selectedManagers],
			selectedStatuses: [...selectedStatuses],
			sortColumn,
			sortOrder,
			offset,
			limit,
			visibleColumns: [...visibleColumns]
		});
	}

	async function loadData(silent = false) {
		if (!silent) {
			if (cached && packages.length > 0) {
				tableLoading = true;
			} else {
				loading = true;
			}
		}
		error = '';
		try {
			const [packagesData, statsData] = await Promise.all([
				api.getHostPackages(hostId, {
					limit,
					offset,
					q: searchTerm || undefined,
					manager: selectedManagers.size > 0 ? [...selectedManagers] : undefined,
					status: selectedStatuses.size > 0 ? [...selectedStatuses] : undefined,
					sort_by: sortColumn,
					sort_order: sortOrder
				}),
				api.getPackageStats(hostId)
			]);

			packages = packagesData.packages || [];
			totalCount = packagesData.pagination?.total ?? 0;
			totalPages = packagesData.pagination?.pages ?? 1;
			stats = statsData;

			if (allManagerKeys.length === 0 && statsData.by_package_manager?.length > 0) {
				allManagerKeys = statsData.by_package_manager.map(
					(pm: { package_manager: string }) => pm.package_manager
				);
			}
		} catch (err: unknown) {
			if (!silent) error = err instanceof Error ? err.message : 'Failed to load packages';
		} finally {
			loading = false;
			tableLoading = false;
			saveToCache();
		}
	}

	let searchDebounce: ReturnType<typeof setTimeout> | null = null;

	function handleSearchInput() {
		offset = 0;
		if (searchDebounce) clearTimeout(searchDebounce);
		searchDebounce = setTimeout(() => loadData(), 300);
	}

	function toggleManager(manager: string) {
		const next = new Set(selectedManagers);
		if (next.has(manager)) next.delete(manager);
		else next.add(manager);
		selectedManagers = next;
		offset = 0;
		loadData();
	}

	function clearFilter() {
		selectedManagers = new Set();
		offset = 0;
		loadData();
	}

	const hasActiveFilters = $derived(
		searchTerm !== '' || selectedStatuses.size > 0 || selectedManagers.size > 0
	);

	function clearAllFilters() {
		searchTerm = '';
		selectedStatuses = new Set();
		selectedManagers = new Set();
		offset = 0;
		loadData();
	}

	function toggleColumn(key: ColumnKey) {
		const next = new Set(visibleColumns);
		if (next.has(key)) next.delete(key);
		else next.add(key);
		visibleColumns = next;
		try {
			localStorage.setItem(COLUMNS_STORAGE_KEY, JSON.stringify([...next]));
		} catch {}
		saveToCache();
	}

	function toggleStatusFilter(status: StatusFilter) {
		const next = new Set(selectedStatuses);
		if (next.has(status)) next.delete(status);
		else next.add(status);
		selectedStatuses = next;
		offset = 0;
		loadData();
	}

	function handlePageChange(newPage: number) {
		offset = (newPage - 1) * limit;
		loadData();
	}

	function handlePageSizeChange(size: number) {
		limit = size;
		offset = 0;
		loadData();
	}

	function handleSort(column: string) {
		if (sortColumn === column) {
			sortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
		} else {
			sortColumn = column;
			sortOrder = column === 'latest_version' ? 'desc' : 'asc';
		}
		offset = 0;
		loadData();
	}

	let collecting = $state(false);
	let collectError = $state('');
	let collectErrorTimeout: ReturnType<typeof setTimeout> | null = null;

	const COLLECT_SESSION_KEY = $derived(`wf_awaiting_collect_${hostId}`);
	const COLLECT_TIMEOUT_MS = 5 * 60 * 1000; // 5 minutes

	function getStoredAwaitingInventory(): boolean {
		try {
			const raw = sessionStorage.getItem(COLLECT_SESSION_KEY);
			if (!raw) return false;
			const ts = Number(raw);
			if (isNaN(ts) || Date.now() - ts > COLLECT_TIMEOUT_MS) {
				sessionStorage.removeItem(COLLECT_SESSION_KEY);
				return false;
			}
			return true;
		} catch {
			return false;
		}
	}

	let awaitingInventory = $state(getStoredAwaitingInventory());

	async function handleForceCollect() {
		if (collecting) return;
		collecting = true;
		collectError = '';
		try {
			await api.triggerPackageCollect(hostId);
			awaitingInventory = true;
			try {
				sessionStorage.setItem(COLLECT_SESSION_KEY, String(Date.now()));
			} catch {
				/* unavailable */
			}
		} catch (err: unknown) {
			collectError = err instanceof Error ? err.message : 'Failed to trigger collection';
			if (collectErrorTimeout) clearTimeout(collectErrorTimeout);
			collectErrorTimeout = setTimeout(() => {
				collectError = '';
			}, 4000);
		} finally {
			collecting = false;
		}
	}
</script>

<svelte:head>
	<title>Packages{ctx.host ? ` - ${ctx.host.display_name}` : ''} - Watchflare</title>
</svelte:head>

<!-- Error -->
{#if error}
	<div role="alert" class="mb-6 rounded-lg border border-destructive bg-destructive/10 p-4">
		<p class="text-sm text-destructive">{error}</p>
	</div>
{/if}

<!-- Stats -->
{#if loading}
	<div class="flex items-center gap-4 mb-4 animate-pulse">
		<div class="h-4 w-24 rounded bg-muted"></div>
		<div class="h-4 w-20 rounded bg-muted"></div>
		<div class="h-4 w-28 rounded bg-muted"></div>
	</div>
{:else if stats}
	<div class="flex items-center gap-x-4 gap-y-1 flex-wrap text-sm mb-4">
		<span class="text-muted-foreground"
			><span class="font-medium text-foreground">{stats.total_packages || 0}</span> packages</span
		>
		{#if (stats.outdated_count || 0) > 0}
			<span class="text-warning"
				><span class="font-medium">{stats.outdated_count}</span> outdated</span
			>
		{/if}
		{#if (stats.security_updates_count || 0) > 0}
			<span class="text-destructive"
				><span class="font-medium">{stats.security_updates_count}</span> security updates</span
			>
		{/if}
	</div>
{/if}

<!-- Search & Filters -->
<div class="mb-4 flex items-center gap-2 flex-wrap">
	<input
		type="text"
		bind:value={searchTerm}
		oninput={handleSearchInput}
		onkeydown={(e) => {
			if (e.key === 'Enter') {
				if (searchDebounce) clearTimeout(searchDebounce);
				offset = 0;
				loadData();
			}
		}}
		placeholder="Search packages..."
		class="flex-1 min-w-0 h-9 rounded-lg border bg-card px-3 text-sm text-foreground placeholder:text-sm placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
	/>
	<button
		type="button"
		onclick={handleForceCollect}
		disabled={collecting || awaitingInventory || ctx.host?.status !== 'online'}
		title={ctx.host?.status !== 'online'
			? 'Host must be online to collect packages'
			: 'Force package collection now'}
		class="shrink-0 inline-flex items-center gap-1.5 h-9 rounded-lg border px-3 text-sm font-medium transition-colors whitespace-nowrap
                bg-card text-muted-foreground hover:bg-muted hover:text-foreground
                disabled:opacity-40 disabled:cursor-not-allowed"
	>
		<RefreshCw class="h-3.5 w-3.5 {collecting || awaitingInventory ? 'animate-spin' : ''}" />
		<span class="hidden sm:inline">Collect Now</span>
	</button>

	{#if hasActiveFilters}
		<button
			type="button"
			onclick={clearAllFilters}
			class="inline-flex items-center gap-1.5 h-9 rounded-lg border px-3 text-sm font-medium transition-colors whitespace-nowrap bg-card text-muted-foreground hover:bg-muted hover:text-foreground"
			title="Clear all filters"
		>
			<X class="h-3.5 w-3.5 shrink-0" />
			<span class="hidden sm:inline">Clear filters</span>
		</button>
	{/if}

	<!-- Status filter -->
	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<button
					type="button"
					{...props}
					class="inline-flex items-center gap-1.5 h-9 rounded-lg border px-3 text-sm font-medium transition-colors whitespace-nowrap
                        {isStatusFiltered
						? 'border-primary/40 bg-primary/5 text-primary hover:bg-primary/10'
						: 'bg-card text-muted-foreground hover:bg-muted hover:text-foreground'}"
				>
					<Tag class="h-3.5 w-3.5 shrink-0" />
					<span class="hidden sm:inline">{statusFilterLabel}</span>
					{#if isStatusFiltered}
						<span
							class="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-primary/15 px-1 text-xs font-medium text-primary"
						>
							{selectedStatuses.size}
						</span>
					{/if}
					<ChevronDown class="hidden sm:inline-block h-3 w-3 opacity-40" />
				</button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start">
			{#each [{ value: 'outdated' as StatusFilter, label: 'Outdated' }, { value: 'security' as StatusFilter, label: 'Security update' }, { value: 'up_to_date' as StatusFilter, label: 'Up to date' }, { value: 'not_checked' as StatusFilter, label: 'Not checked' }] as status}
				<DropdownMenu.Item closeOnSelect={false} onclick={() => toggleStatusFilter(status.value)}>
					<div
						class="flex h-4 w-4 shrink-0 items-center justify-center rounded border
                            {selectedStatuses.has(status.value)
							? 'border-primary bg-primary'
							: 'border-muted-foreground/40'}"
					>
						{#if selectedStatuses.has(status.value)}
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
					<span class="flex-1">{status.label}</span>
				</DropdownMenu.Item>
			{/each}
			{#if isStatusFiltered}
				<DropdownMenu.Separator />
				<DropdownMenu.Item
					onclick={() => {
						selectedStatuses = new Set();
						offset = 0;
						loadData();
					}}
					class="text-muted-foreground"
				>
					Clear filter
				</DropdownMenu.Item>
			{/if}
		</DropdownMenu.Content>
	</DropdownMenu.Root>

	<!-- Package manager filter -->
	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<button
					type="button"
					{...props}
					class="inline-flex items-center gap-1.5 h-9 rounded-lg border px-3 text-sm font-medium transition-colors whitespace-nowrap
                        {isFiltered
						? 'border-primary/40 bg-primary/5 text-primary hover:bg-primary/10'
						: 'bg-card text-muted-foreground hover:bg-muted hover:text-foreground'}"
				>
					<Filter class="h-3.5 w-3.5" />
					<span class="hidden sm:inline">{filterLabel}</span>
					{#if isFiltered}
						<span
							class="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-primary/15 px-1 text-xs font-medium text-primary"
						>
							{selectedManagers.size}
						</span>
					{/if}
					<ChevronDown class="hidden sm:inline-block h-3 w-3 opacity-40" />
				</button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start">
			{#each [...(stats?.by_package_manager || [])].sort((a, b) => b.count - a.count) as pm}
				<DropdownMenu.Item closeOnSelect={false} onclick={() => toggleManager(pm.package_manager)}>
					<div
						class="flex h-4 w-4 shrink-0 items-center justify-center rounded border
                        {selectedManagers.has(pm.package_manager)
							? 'border-primary bg-primary'
							: 'border-muted-foreground/40'}"
					>
						{#if selectedManagers.has(pm.package_manager)}
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
					<span class="flex-1">{getManagerLabel(pm.package_manager)}</span>
					<span class="ml-4 tabular-nums text-xs text-muted-foreground">{pm.count}</span>
				</DropdownMenu.Item>
			{/each}
			{#if isFiltered}
				<DropdownMenu.Separator />
				<DropdownMenu.Item onclick={clearFilter} class="text-muted-foreground">
					Clear filter
				</DropdownMenu.Item>
			{/if}
		</DropdownMenu.Content>
	</DropdownMenu.Root>

	<!-- Column visibility (desktop only) -->
	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<button
					type="button"
					{...props}
					class="hidden md:inline-flex items-center gap-1.5 h-9 rounded-lg border px-3 text-sm font-medium transition-colors whitespace-nowrap
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
					<ChevronDown class="hidden sm:inline-block h-3 w-3 opacity-40" />
				</button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start">
			{#each ALL_COLUMNS as column}
				<DropdownMenu.Item closeOnSelect={false} onclick={() => toggleColumn(column.key)}>
					<div
						class="flex h-4 w-4 shrink-0 items-center justify-center rounded border
                        {visibleColumns.has(column.key)
							? 'border-primary bg-primary'
							: 'border-muted-foreground/40'}"
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
					<span class="flex-1">{column.label}</span>
				</DropdownMenu.Item>
			{/each}
		</DropdownMenu.Content>
	</DropdownMenu.Root>
</div>
{#if collectError}
	<p class="mb-2 text-xs text-destructive">{collectError}</p>
{/if}

{#snippet sortIcon(column: string)}
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
			class="h-3 w-3 shrink-0 opacity-40 group-hover:opacity-100 transition-opacity"
			viewBox="0 0 12 12"
			fill="currentColor"
		>
			<path d="M6 10l4-5H2z" />
		</svg>
	{/if}
{/snippet}

<!-- Packages Table/Cards -->
<div
	class="rounded-xl border bg-card overflow-hidden mb-2 {tableLoading
		? 'opacity-60 pointer-events-none'
		: ''}"
>
	{#if loading}
		<!-- Mobile: skeleton cards -->
		<div class="md:hidden p-3 flex flex-col gap-2">
			{#each Array(6) as _}
				<div class="rounded-lg border bg-card animate-pulse">
					<div
						class="rounded-t-lg bg-table-header px-4 py-2.5 border-b border-border flex items-center justify-between gap-2"
					>
						<div class="h-4 w-32 rounded bg-muted"></div>
						<div class="h-5 w-16 rounded-full bg-muted"></div>
					</div>
					<div class="px-4 py-2.5 flex items-center gap-2">
						<div class="h-3 w-12 rounded bg-muted"></div>
						<div class="h-5 w-14 rounded-full bg-muted"></div>
					</div>
				</div>
			{/each}
		</div>
		<!-- Desktop: skeleton -->
		<div class="hidden md:block animate-pulse">
			<div
				class="bg-table-header sticky top-0 [box-shadow:0_1px_0_var(--border)] px-4 py-2.5 flex gap-6"
			>
				<div class="h-4 w-24 rounded bg-muted"></div>
				<div class="h-4 w-20 rounded bg-muted"></div>
				<div class="h-4 w-20 rounded bg-muted"></div>
				<div class="h-4 w-16 rounded bg-muted"></div>
				<div class="h-4 w-20 rounded bg-muted"></div>
			</div>
			{#each Array(8) as _}
				<div class="border-b px-4 py-3 flex items-center gap-6">
					<div class="h-4 w-32 rounded bg-muted"></div>
					<div class="h-4 w-20 rounded bg-muted font-mono"></div>
					<div class="h-4 w-20 rounded bg-muted"></div>
					<div class="h-5 w-16 rounded-full bg-muted"></div>
					<div class="h-4 w-24 rounded bg-muted"></div>
				</div>
			{/each}
		</div>
	{:else}
		<!-- Mobile: cards -->
		<div class="md:hidden p-3 flex flex-col gap-2">
			{#each packages as pkg}
				<div class="rounded-lg border bg-card">
					<!-- Header: name + status badge -->
					<div
						class="rounded-t-lg bg-table-header px-4 py-2.5 border-b border-border flex items-center justify-between gap-2"
					>
						<span class="flex items-center gap-2 min-w-0">
							<PackageIcon class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
							<span class="text-sm font-medium text-foreground break-all">{pkg.name}</span>
						</span>
						<PackageStatusBadge
							hasSecurityUpdate={pkg.has_security_update}
							availableVersion={pkg.available_version}
							updateChecked={pkg.update_checked}
						/>
					</div>
					<!-- Body: version + latest + manager -->
					<div class="px-4 py-2.5 flex items-center gap-2 flex-wrap">
						<span class="text-xs font-mono text-muted-foreground">{pkg.version || '—'}</span>
						{#if pkg.available_version}
							<span
								class="inline-flex items-center gap-1 text-xs font-mono font-medium {pkg.has_security_update
									? 'text-destructive'
									: 'text-warning'}"
							>
								{#if pkg.has_security_update}
									<ShieldAlert class="h-3 w-3 shrink-0" />
								{:else}
									<ArrowUp class="h-3 w-3 shrink-0" />
								{/if}
								{pkg.available_version}
							</span>
						{:else if pkg.update_checked}
							<span class="text-xs font-mono text-muted-foreground">{pkg.version || '—'}</span>
						{/if}
						<span
							class="inline-flex rounded-full border px-2 py-0.5 text-xs font-medium {getManagerColor(
								pkg.package_manager
							)}">{getManagerLabel(pkg.package_manager)}</span
						>
					</div>
				</div>
			{:else}
				<div class="py-16 text-center">
					<svg
						class="mx-auto h-10 w-10 text-muted-foreground/40 mb-3"
						fill="none"
						stroke="currentColor"
						viewBox="0 0 24 24"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="1.5"
							d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"
						/>
					</svg>
					<p class="text-sm text-muted-foreground">No packages found</p>
				</div>
			{/each}
		</div>

		<!-- Desktop: table -->
		<div class="hidden md:block overflow-auto max-h-[65vh]">
			<table class="w-full min-w-120">
				<thead>
					<tr
						class="bg-table-header sticky top-0 z-10 [box-shadow:0_1px_0_var(--border)] whitespace-nowrap"
					>
						{#if col('name')}
							<th
								scope="col"
								class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
							>
								<button
									type="button"
									onclick={() => handleSort('name')}
									class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 cursor-pointer select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
									'name'
										? 'bg-table-header-active text-foreground'
										: ''}">Name {@render sortIcon('name')}</button
								>
							</th>
						{/if}
						{#if col('version')}
							<th
								scope="col"
								class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
							>
								<button
									type="button"
									onclick={() => handleSort('version')}
									class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 cursor-pointer select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
									'version'
										? 'bg-table-header-active text-foreground'
										: ''}">Version {@render sortIcon('version')}</button
								>
							</th>
						{/if}
						{#if col('status')}
							<th
								scope="col"
								class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground w-px whitespace-nowrap"
							>
								<button
									type="button"
									onclick={() => handleSort('status')}
									class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 cursor-pointer select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
									'status'
										? 'bg-table-header-active text-foreground'
										: ''}">Status {@render sortIcon('status')}</button
								>
							</th>
						{/if}
						{#if col('manager')}
							<th
								scope="col"
								class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground w-px whitespace-nowrap"
							>
								<button
									type="button"
									onclick={() => handleSort('manager')}
									class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 cursor-pointer select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
									'manager'
										? 'bg-table-header-active text-foreground'
										: ''}">Manager {@render sortIcon('manager')}</button
								>
							</th>
						{/if}
						{#if col('latest_version')}
							<th
								scope="col"
								class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground whitespace-nowrap"
							>
								<button
									type="button"
									onclick={() => handleSort('latest_version')}
									class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 cursor-pointer select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
									'latest_version'
										? 'bg-table-header-active text-foreground'
										: ''}">Latest Version {@render sortIcon('latest_version')}</button
								>
							</th>
						{/if}
						{#if col('arch')}
							<th
								scope="col"
								class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground w-28"
							>
								<button
									type="button"
									onclick={() => handleSort('arch')}
									class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 cursor-pointer select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn ===
									'arch'
										? 'bg-table-header-active text-foreground'
										: ''}">Architecture {@render sortIcon('arch')}</button
								>
							</th>
						{/if}
						{#if col('description')}
							<th
								scope="col"
								class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground w-64"
								>Description</th
							>
						{/if}
						{#if col('first_seen')}
							<th
								scope="col"
								class="px-4 py-2.5 text-right text-sm font-semibold text-muted-foreground w-36"
							>
								<button
									type="button"
									onclick={() => handleSort('first_seen')}
									class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 cursor-pointer select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary ml-auto {sortColumn ===
									'first_seen'
										? 'bg-table-header-active text-foreground'
										: ''}">First Seen {@render sortIcon('first_seen')}</button
								>
							</th>
						{/if}
						{#if col('last_seen')}
							<th
								scope="col"
								class="px-4 py-2.5 text-right text-sm font-semibold text-muted-foreground w-36"
							>
								<button
									type="button"
									onclick={() => handleSort('last_seen')}
									class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 cursor-pointer select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary ml-auto {sortColumn ===
									'last_seen'
										? 'bg-table-header-active text-foreground'
										: ''}">Last Seen {@render sortIcon('last_seen')}</button
								>
							</th>
						{/if}
					</tr>
				</thead>
				<tbody class="divide-y divide-border">
					{#each packages as pkg}
						<tr class="hover:bg-muted/20 transition-colors">
							{#if col('name')}
								<td class="px-4 py-3">
									<span class="flex items-center gap-2">
										<PackageIcon class="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
										<span class="text-sm font-medium text-foreground">{pkg.name}</span>
									</span>
								</td>
							{/if}
							{#if col('version')}
								<td class="px-4 py-3 text-sm font-mono text-muted-foreground whitespace-nowrap"
									>{pkg.version || '-'}</td
								>
							{/if}
							{#if col('status')}
								<td class="px-4 py-3 w-px whitespace-nowrap">
									<PackageStatusBadge
										hasSecurityUpdate={pkg.has_security_update}
										availableVersion={pkg.available_version}
										updateChecked={pkg.update_checked}
									/>
								</td>
							{/if}
							{#if col('manager')}
								<td class="px-4 py-3 w-px whitespace-nowrap">
									<span
										class="inline-flex rounded-full border px-2 py-0.5 text-xs font-medium {getManagerColor(
											pkg.package_manager
										)}"
									>
										{getManagerLabel(pkg.package_manager)}
									</span>
								</td>
							{/if}
							{#if col('latest_version')}
								<td class="px-4 py-3 text-sm whitespace-nowrap">
									{#if pkg.available_version}
										<span class="inline-flex items-center gap-1">
											{#if pkg.has_security_update}
												<ShieldAlert class="h-3.5 w-3.5 text-destructive shrink-0" />
												<span class="font-mono font-medium text-destructive"
													>{pkg.available_version}</span
												>
											{:else}
												<ArrowUp class="h-3.5 w-3.5 text-warning shrink-0" />
												<span class="font-mono font-medium text-warning"
													>{pkg.available_version}</span
												>
											{/if}
										</span>
									{:else if pkg.update_checked}
										<span class="font-mono text-muted-foreground">{pkg.version || '—'}</span>
									{:else}
										<span class="text-muted-foreground/40">—</span>
									{/if}
								</td>
							{/if}
							{#if col('arch')}
								<td class="px-4 py-3 text-sm text-muted-foreground whitespace-nowrap"
									>{pkg.architecture || '-'}</td
								>
							{/if}
							{#if col('description')}
								<td class="px-4 py-3">
									<span
										class="block text-sm text-muted-foreground truncate max-w-64"
										title={pkg.description || ''}>{pkg.description || '-'}</span
									>
								</td>
							{/if}
							{#if col('first_seen')}
								<td class="px-4 py-3 text-right text-sm text-muted-foreground whitespace-nowrap"
									>{formatDateTime(pkg.first_seen, timeFormat)}</td
								>
							{/if}
							{#if col('last_seen')}
								<td class="px-4 py-3 text-right text-sm text-muted-foreground whitespace-nowrap"
									>{formatDateTime(pkg.last_seen, timeFormat)}</td
								>
							{/if}
						</tr>
					{:else}
						<tr>
							<td colspan={visibleColumns.size} class="py-16 text-center">
								<svg
									class="mx-auto h-10 w-10 text-muted-foreground/40 mb-3"
									fill="none"
									stroke="currentColor"
									viewBox="0 0 24 24"
								>
									<path
										stroke-linecap="round"
										stroke-linejoin="round"
										stroke-width="1.5"
										d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"
									/>
								</svg>
								<p class="text-sm text-muted-foreground">No packages found</p>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<Pagination
			{currentPage}
			{totalPages}
			totalItems={totalCount}
			pageSize={limit}
			itemLabel="packages"
			onPageChange={handlePageChange}
			onPageSizeChange={handlePageSizeChange}
			pageSizeOptions={PAGE_SIZE_OPTIONS}
		/>
	{/if}
</div>
