import { writable, derived } from 'svelte/store';
import type { Host, HostWithMetrics, HostStatus } from '$lib/types';
import { listHosts, getHostStats } from '$lib/api';
import { logger } from '$lib/utils';

interface HostsState {
	hosts: HostWithMetrics[];
	loading: boolean;
	error: string | null;
}

function createHostsStore() {
	const { subscribe, set, update } = writable<HostsState>({
		hosts: [],
		loading: false,
		error: null
	});

	return {
		subscribe,

		// Load hosts from API
		async load(): Promise<void> {
			update((state) => ({ ...state, loading: true, error: null }));

			try {
				const data = await listHosts();
				const hosts = data.hosts.map((host) => ({
					host
				}));

				update((state) => ({
					...state,
					hosts,
					loading: false
				}));
			} catch (err) {
				const error = err instanceof Error ? err.message : 'Failed to load hosts';
				update((state) => ({ ...state, loading: false, error }));
				throw err;
			}
		},

		// Update a single host (from SSE events)
		updateHost(hostId: string, updates: Partial<Host>): void {
			update((state) => ({
				...state,
				hosts: state.hosts.map((item) =>
					item.host.id === hostId ? { ...item, host: { ...item.host, ...updates } } : item
				)
			}));
		},

		// Update host status
		updateStatus(hostId: string, status: HostStatus, lastSeen: string): void {
			update((state) => ({
				...state,
				hosts: state.hosts.map((item) =>
					item.host.id === hostId
						? {
								...item,
								host: {
									...item.host,
									status,
									last_seen: lastSeen
								}
							}
						: item
				)
			}));
		},

		// Add a new host
		addHost(host: Host): void {
			update((state) => ({
				...state,
				hosts: [...state.hosts, { host }]
			}));
		},

		// Remove a host
		removeHost(hostId: string): void {
			update((state) => ({
				...state,
				hosts: state.hosts.filter((item) => item.host.id !== hostId)
			}));
		},

		// Clear all hosts
		clear(): void {
			set({ hosts: [], loading: false, error: null });
		}
	};
}

export const hostsStore = createHostsStore();

// Lightweight store for dashboard counts
interface HostStatsState {
	total: number;
	online: number;
	loading: boolean;
}

function createHostStatsStore() {
	const { subscribe, update, set } = writable<HostStatsState>({
		total: 0,
		online: 0,
		loading: false
	});

	// Tracks per-host status to compute deltas on SSE updates (avoids API refetch on every heartbeat)
	const statusMap = new Map<string, HostStatus>();

	return {
		subscribe,

		async load(): Promise<void> {
			update((s) => ({ ...s, loading: true }));
			try {
				const data = await getHostStats();
				statusMap.clear();
				set({ total: data.total, online: data.online, loading: false });
			} catch (err) {
				logger.error('Failed to load host stats:', err);
				update((s) => ({ ...s, loading: false }));
			}
		},

		// Apply a host status change from SSE without an API call.
		// Heartbeats that keep the same status are no-ops.
		applyUpdate(id: string, newStatus: HostStatus): void {
			const prevStatus = statusMap.get(id);
			if (prevStatus === newStatus) return;
			statusMap.set(id, newStatus);

			if (prevStatus === undefined) {
				// First SSE for this host — counts are already correct from load()
				return;
			}

			update((s) => {
				let { total, online } = s;
				if (prevStatus !== 'pending') total--;
				if (prevStatus === 'online') online--;
				if (newStatus !== 'pending') total++;
				if (newStatus === 'online') online++;
				return { ...s, total, online };
			});
		}
	};
}

export const hostStatsStore = createHostStatsStore();

// Derived stores for convenience
export const hosts = derived(hostsStore, ($store) => $store.hosts);
export const onlineHosts = derived(hostsStore, ($store) =>
	$store.hosts.filter((item) => item.host.status === 'online')
);
export const offlineHosts = derived(hostsStore, ($store) =>
	$store.hosts.filter((item) => item.host.status === 'offline')
);
export const hostsLoading = derived(hostsStore, ($store) => $store.loading);
