<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import * as api from '$lib/api.js';
	import { handleSSEReactivation, logger } from '$lib/utils';
	import { HOSTS_PER_PAGE, SEARCH_DEBOUNCE_MS } from '$lib/constants';
	import type {
		Host,
		HostWithMetrics,
		SSEEvent,
		HostUpdateEvent,
		MetricsUpdateEvent,
		HostStatus
	} from '$lib/types';
	import { sseStore, metricsStore, latestMetrics } from '$lib/stores';
	import { alertsStore } from '$lib/stores/alerts';
	import HostTable from '$lib/components/HostTable.svelte';
	import HostFilters from '$lib/components/host/HostFilters.svelte';
	import Pagination from '$lib/components/Pagination.svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import AddHostModal from '$lib/components/host/AddHostModal.svelte';

	const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];
	let perPage = $state(HOSTS_PER_PAGE);

	let hosts: Host[] = $state([]);
	let total = $state(0);
	let currentPage = $state(1);
	let initialLoading = $state(true);
	let loading = $state(false);
	let latestAgentVersion: string | null = $state(null);
	let error = $state('');
	let showAddHost = $state(false);

	let sseUnsubscribe: (() => void) | null = null;
	let loadAbortController: AbortController | null = null;

	// Filter state
	let searchQuery = $state('');
	let statusFilter = $state<HostStatus | ''>('');
	let searchTimeout: ReturnType<typeof setTimeout> | null = null;

	// Agent update tracking
	let updatingHosts = $state(new Set<string>());
	const updateTimeouts = new Map<string, ReturnType<typeof setTimeout>>();

	// Delete modal
	let showDeleteConfirm = $state(false);
	let hostToDelete: Host | null = $state(null);

	// Rename modal
	let showRename = $state(false);
	let selectedHost: Host | null = $state(null);
	let newHostName = $state('');

	let totalPages = $derived(Math.max(1, Math.ceil(total / perPage)));

	let hostsWithMetrics = $derived<HostWithMetrics[]>(hosts.map((host) => ({ host })));

	async function loadPage(p: number) {
		loadAbortController?.abort();
		loadAbortController = new AbortController();
		const signal: AbortSignal = loadAbortController.signal;

		loading = true;
		error = '';
		try {
			const [response, versionResponse] = await Promise.all([
				api.listHosts({
					page: p,
					perPage: perPage,
					status: statusFilter || undefined,
					search: searchQuery || undefined,
					signal
				}),
				latestAgentVersion === null
					? api.getLatestAgentVersion().catch(() => ({ latest_version: '' }))
					: Promise.resolve({ latest_version: latestAgentVersion })
			]);
			if (signal.aborted) return;
			hosts = response.hosts || [];
			total = response.total || 0;
			currentPage = p;
			// Load latest metrics for visible hosts (fire and forget)
			const hostIds = hosts.map((h) => h.id);
			if (hostIds.length > 0) metricsStore.loadForHosts(hostIds);
			if (latestAgentVersion === null) latestAgentVersion = versionResponse.latest_version || null;
		} catch (err: unknown) {
			if (err instanceof Error && err.name === 'AbortError') return;
			error = err instanceof Error ? err.message : 'Failed to load hosts';
		} finally {
			if (!signal.aborted) {
				loading = false;
				initialLoading = false;
			}
		}
	}

	function handleSearchInput(e: Event) {
		const value = (e.target as HTMLInputElement).value;
		searchQuery = value;
		if (searchTimeout) clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => {
			loadPage(1);
		}, SEARCH_DEBOUNCE_MS);
	}

	function handleStatusChange(value: string) {
		statusFilter = value as HostStatus | '';
		loadPage(1);
	}

	function handlePageSizeChange(size: number) {
		if (!PAGE_SIZE_OPTIONS.includes(size)) return;
		perPage = size;
		loadPage(1);
	}

	function handleSSEMessage(event: SSEEvent) {
		handleSSEReactivation(event);

		if (event.type === 'host_update') {
			const update = event.data as HostUpdateEvent;
			const idx = hosts.findIndex((h) => h.id === update.id);
			if (idx !== -1) {
				const prev = hosts[idx];
				hosts[idx] = {
					...prev,
					status: update.status,
					ip_address_v4: update.ip_address_v4 ?? prev.ip_address_v4,
					ip_address_v6: update.ip_address_v6 ?? prev.ip_address_v6,
					configured_ip: update.configured_ip ?? prev.configured_ip,
					ignore_ip_mismatch: update.ignore_ip_mismatch ?? prev.ignore_ip_mismatch,
					last_seen: update.last_seen,
					agent_version: update.agent_version ?? prev.agent_version
				};
				hosts = [...hosts];
				// Clear update spinner when a new agent version is confirmed via SSE
				if (
					update.agent_version &&
					update.agent_version !== prev.agent_version &&
					updatingHosts.has(update.id)
				) {
					const t = updateTimeouts.get(update.id);
					if (t !== undefined) clearTimeout(t);
					updateTimeouts.delete(update.id);
					updatingHosts.delete(update.id);
					updatingHosts = new Set(updatingHosts);
				}
			}
		} else if (event.type === 'metrics_update') {
			const m = event.data as MetricsUpdateEvent;
			metricsStore.updateHostMetrics(m.host_id, m);
		}
	}

	function handleRename(host: Host) {
		selectedHost = host;
		newHostName = host.display_name;
		showRename = true;
	}

	function closeRenameModal() {
		showRename = false;
		newHostName = '';
		selectedHost = null;
	}

	async function handleRenameSubmit() {
		if (!selectedHost) return;
		const name = newHostName;
		try {
			await api.renameHost(selectedHost.id, name);
			const idx = hosts.findIndex((h) => h.id === selectedHost!.id);
			if (idx !== -1) {
				hosts[idx] = { ...hosts[idx], display_name: name };
				hosts = [...hosts];
			}
			closeRenameModal();
		} catch (err) {
			logger.error('Failed to rename host:', err);
			error = err instanceof Error ? err.message : 'Failed to rename host';
		}
	}

	async function handlePause(hostId: string) {
		try {
			await api.pauseHost(hostId);
			const idx = hosts.findIndex((h) => h.id === hostId);
			if (idx !== -1) {
				hosts[idx] = { ...hosts[idx], status: 'paused' };
				hosts = [...hosts];
			}
			alertsStore.loadIncidents();
		} catch (err) {
			logger.error('Failed to pause host:', err);
			error = err instanceof Error ? err.message : 'Failed to pause host';
		}
	}

	async function handleResume(hostId: string) {
		try {
			await api.resumeHost(hostId);
			const idx = hosts.findIndex((h) => h.id === hostId);
			if (idx !== -1) {
				hosts[idx] = { ...hosts[idx], status: 'pending' };
				hosts = [...hosts];
			}
			alertsStore.loadIncidents();
		} catch (err) {
			logger.error('Failed to resume host:', err);
			error = err instanceof Error ? err.message : 'Failed to resume host';
		}
	}

	function handleDeleteRequest(host: Host) {
		hostToDelete = host;
		showDeleteConfirm = true;
	}

	async function handleUpdateAgent(hostId: string) {
		try {
			await api.triggerAgentUpdate(hostId);
			updatingHosts = new Set([...updatingHosts, hostId]);
			// Safety timeout: clear the spinner after 2 minutes if SSE never arrives
			const t = setTimeout(() => {
				updatingHosts.delete(hostId);
				updatingHosts = new Set(updatingHosts);
				updateTimeouts.delete(hostId);
			}, 120_000);
			updateTimeouts.set(hostId, t);
		} catch (err) {
			logger.error('Failed to trigger agent update:', err);
			error = err instanceof Error ? err.message : 'Failed to trigger agent update';
		}
	}

	async function handleDelete() {
		if (!hostToDelete) return;
		try {
			await api.deleteHost(hostToDelete.id);
			const targetPage = hosts.length === 1 && currentPage > 1 ? currentPage - 1 : currentPage;
			await loadPage(targetPage);
			showDeleteConfirm = false;
			hostToDelete = null;
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Failed to delete host';
		}
	}

	onMount(async () => {
		await loadPage(1);
		sseUnsubscribe = sseStore.connect(handleSSEMessage);
	});

	onDestroy(() => {
		if (sseUnsubscribe) sseUnsubscribe();
		if (searchTimeout) clearTimeout(searchTimeout);
		loadAbortController?.abort();
		for (const t of updateTimeouts.values()) clearTimeout(t);
	});
</script>

<svelte:head>
	<title>Hosts - Watchflare</title>
</svelte:head>

<!-- Header -->
<div class="mb-6 flex items-center justify-between">
	<div>
		<h1 class="text-xl sm:text-2xl font-semibold text-foreground">Hosts</h1>
		<p class="text-sm text-muted-foreground mt-1">Manage and monitor your hosts</p>
	</div>
	<button
		type="button"
		onclick={() => (showAddHost = true)}
		class="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
	>
		Add Host
	</button>
</div>

<!-- Filters -->
<HostFilters
	{searchQuery}
	{statusFilter}
	onSearchInput={handleSearchInput}
	onStatusChange={handleStatusChange}
/>

{#if initialLoading}
	<!-- Skeleton -->
	<div class="rounded-xl border bg-card animate-pulse">
		<div class="border-b bg-table-header px-4 py-2.5 flex gap-8">
			{#each Array(5) as _}
				<div class="h-4 w-16 rounded bg-muted"></div>
			{/each}
		</div>
		{#each Array(8) as _}
			<div class="border-b px-4 py-3 flex gap-8">
				<div class="h-4 w-24 rounded bg-muted"></div>
				<div class="h-4 w-12 rounded bg-muted"></div>
				<div class="h-4 w-10 rounded bg-muted"></div>
				<div class="h-4 w-10 rounded bg-muted"></div>
				<div class="h-4 w-10 rounded bg-muted"></div>
			</div>
		{/each}
	</div>
{:else if error}
	<div role="alert" class="rounded-lg border border-destructive bg-destructive/10 p-4">
		<p class="text-sm text-destructive">{error}</p>
	</div>
{:else if hosts.length === 0 && currentPage === 1 && !searchQuery && !statusFilter}
	<div
		class="flex flex-col items-center justify-center rounded-lg border bg-card py-20 text-center"
	>
		<svg
			class="h-12 w-12 text-muted-foreground/50 mb-4"
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
		<h3 class="text-lg font-medium text-foreground mb-2">No hosts configured yet</h3>
		<p class="text-sm text-muted-foreground mb-6">Add your first host to start monitoring</p>
		<button
			type="button"
			onclick={() => (showAddHost = true)}
			class="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90"
		>
			Add Your First Host
		</button>
	</div>
{:else if hosts.length === 0}
	<div
		class="flex flex-col items-center justify-center rounded-lg border bg-card py-12 text-center"
	>
		<p class="text-sm text-muted-foreground">No hosts match your filters</p>
	</div>
{:else}
	<div class="rounded-xl border bg-card overflow-hidden">
		<HostTable
			hosts={hostsWithMetrics}
			latestMetrics={$latestMetrics}
			showFilters={false}
			tableLoading={loading}
			{latestAgentVersion}
			onRename={handleRename}
			onPause={handlePause}
			onResume={handleResume}
			onDelete={handleDeleteRequest}
			onUpdateAgent={handleUpdateAgent}
			{updatingHosts}
		/>
		<Pagination
			{currentPage}
			{totalPages}
			totalItems={total}
			pageSize={perPage}
			itemLabel="hosts"
			onPageChange={loadPage}
			onPageSizeChange={handlePageSizeChange}
			pageSizeOptions={PAGE_SIZE_OPTIONS}
		/>
	</div>
{/if}

<AddHostModal open={showAddHost} onClose={() => (showAddHost = false)} />

<!-- Delete Confirmation -->
<ConfirmDialog
	open={showDeleteConfirm}
	title="Confirm Delete"
	onConfirm={handleDelete}
	onClose={() => {
		showDeleteConfirm = false;
		hostToDelete = null;
	}}
	confirmLabel="Delete Host"
	confirmVariant="destructive"
>
	<p class="text-sm text-muted-foreground mb-4">
		Are you sure you want to delete "{hostToDelete?.display_name}"?
	</p>
	{#if hostToDelete?.status === 'online'}
		<div class="mb-4 rounded-md border border-primary/20 bg-primary/5 p-3">
			<p class="text-sm text-foreground">
				Note: This will remove the host from the database, but the agent will remain installed on
				the host. You will need to uninstall it manually.
			</p>
		</div>
	{/if}
	<p class="text-sm font-medium text-destructive">This action cannot be undone.</p>
</ConfirmDialog>

<!-- Rename Modal -->
<Modal open={showRename} onClose={closeRenameModal}>
	<h3 class="text-lg font-semibold text-foreground mb-3">Rename Host</h3>
	<div class="mb-4">
		<label for="newname" class="block text-sm font-medium text-foreground mb-2">New Name</label>
		<input
			id="newname"
			type="text"
			bind:value={newHostName}
			placeholder="e.g., production-web-01"
			class="w-full rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
		/>
	</div>
	<div class="flex gap-3 justify-end">
		<button
			type="button"
			onclick={closeRenameModal}
			class="rounded-lg border bg-background px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-muted"
		>
			Cancel
		</button>
		<button
			type="button"
			onclick={handleRenameSubmit}
			disabled={newHostName.length < 2}
			class="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
		>
			Rename
		</button>
	</div>
</Modal>
