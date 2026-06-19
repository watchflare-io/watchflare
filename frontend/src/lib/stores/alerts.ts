import { writable, derived } from 'svelte/store';
import type { ActiveIncident, DroppedMetric } from '$lib/types';
import { getDroppedMetrics, getActiveIncidents } from '$lib/api';
import { logger } from '$lib/utils';

interface AlertsState {
	droppedMetrics: DroppedMetric[];
	activeIncidents: ActiveIncident[];
	loading: boolean;
	error: string | null;
}

function createAlertsStore() {
	const { subscribe, set, update } = writable<AlertsState>({
		droppedMetrics: [],
		activeIncidents: [],
		loading: false,
		error: null
	});

	return {
		subscribe,

		async load(): Promise<void> {
			update((state) => ({ ...state, loading: true, error: null }));
			try {
				const data = await getDroppedMetrics();
				update((state) => ({
					...state,
					droppedMetrics: data.dropped_metrics || [],
					loading: false,
					error: null
				}));
			} catch (err) {
				logger.error('Failed to load dropped metrics:', err);
				update((state) => ({
					...state,
					loading: false,
					error: err instanceof Error ? err.message : 'Failed to load alerts'
				}));
			}
		},

		async loadIncidents(): Promise<void> {
			try {
				const data = await getActiveIncidents();
				update((state) => ({ ...state, activeIncidents: data.incidents }));
			} catch (err) {
				logger.error('Failed to load active incidents:', err);
			}
		},

		clear(): void {
			set({
				droppedMetrics: [],
				activeIncidents: [],
				loading: false,
				error: null
			});
		}
	};
}

export const alertsStore = createAlertsStore();
export const alertCount = derived(alertsStore, ($s) => $s.activeIncidents.length);
