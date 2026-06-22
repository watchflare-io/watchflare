import { writable, derived, get } from 'svelte/store';
import type { Metric } from '$lib/types';
import { getHostMetrics } from '$lib/api';
import { logger } from '$lib/utils';
import { MAX_METRICS_POINTS_DASHBOARD } from '$lib/constants';

interface MetricsState {
	// Map of host ID to array of metrics (for charts, time-range dependent)
	data: Record<string, Metric[]>;
	// Latest real-time metric per host (for table display, independent of time range)
	latest: Record<string, Metric>;
	loading: Record<string, boolean>;
	error: string | null;
}

function createMetricsStore() {
	const { subscribe, set, update } = writable<MetricsState>({
		data: {},
		latest: {},
		loading: {},
		error: null
	});

	return {
		subscribe,

		// Load metrics for a specific host
		async loadForHost(hostId: string, timeRange: string = '1h'): Promise<void> {
			update((state) => ({
				...state,
				loading: { ...state.loading, [hostId]: true },
				error: null
			}));

			try {
				const data = await getHostMetrics(hostId, { time_range: timeRange });
				const metricsArray = data.metrics || [];

				update((state) => {
					const lastPoint = metricsArray.length > 0 ? metricsArray[metricsArray.length - 1] : null;
					const existing = state.latest[hostId];
					// Refresh latest when the loaded point is newer than (or as recent as) the cached one.
					// Keeps the table in sync with current data, while preventing coarse aggregate points
					// (from long time ranges) from clobbering a fresher live SSE value.
					const refreshLatest =
						lastPoint &&
						(!existing || new Date(lastPoint.timestamp) >= new Date(existing.timestamp));
					return {
						...state,
						data: { ...state.data, [hostId]: metricsArray },
						latest: refreshLatest ? { ...state.latest, [hostId]: lastPoint } : state.latest,
						loading: { ...state.loading, [hostId]: false }
					};
				});
			} catch (err) {
				logger.error(`Failed to load metrics for host ${hostId}:`, err);

				update((state) => ({
					...state,
					data: { ...state.data, [hostId]: [] },
					loading: { ...state.loading, [hostId]: false },
					error: err instanceof Error ? err.message : 'Failed to load metrics'
				}));
			}
		},

		// Load metrics for multiple hosts
		async loadForHosts(hostIds: string[], timeRange: string = '1h'): Promise<void> {
			const promises = hostIds.map((id) => this.loadForHost(id, timeRange));
			await Promise.all(promises);
		},

		// Update metrics for a host (add new metric point from SSE)
		updateHostMetrics(hostId: string, metric: Metric): void {
			update((state) => {
				const existingMetrics = state.data[hostId] || [];
				let updatedMetrics = [...existingMetrics, metric];

				// Keep only last N points per host
				if (updatedMetrics.length > MAX_METRICS_POINTS_DASHBOARD) {
					updatedMetrics = updatedMetrics.slice(-MAX_METRICS_POINTS_DASHBOARD);
				}

				return {
					...state,
					data: { ...state.data, [hostId]: updatedMetrics },
					// Always update latest for real-time display
					latest: { ...state.latest, [hostId]: metric }
				};
			});
		},

		// Get metrics for a specific host
		getForHost(hostId: string): Metric[] {
			return get({ subscribe }).data[hostId] || [];
		},

		// Clear metrics for a specific host
		clearForHost(hostId: string): void {
			update((state) => {
				const newData = { ...state.data };
				delete newData[hostId];
				const newLoading = { ...state.loading };
				delete newLoading[hostId];

				return {
					...state,
					data: newData,
					loading: newLoading
				};
			});
		},

		// Clear all metrics
		clear(): void {
			set({ data: {}, latest: {}, loading: {}, error: null });
		}
	};
}

export const metricsStore = createMetricsStore();

// Derived store to get all metrics data (for charts, time-range dependent)
export const metricsData = derived(metricsStore, ($store) => $store.data);

// Derived store for latest real-time metric per host (for table display)
export const latestMetrics = derived(metricsStore, ($store) => $store.latest);
