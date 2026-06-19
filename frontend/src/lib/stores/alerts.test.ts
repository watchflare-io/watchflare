import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';

vi.mock('$lib/api', () => ({
	getDroppedMetrics: vi.fn(),
	getActiveIncidents: vi.fn()
}));

vi.mock('$lib/utils', () => ({
	logger: { error: vi.fn(), warn: vi.fn(), log: vi.fn() }
}));

import { alertsStore, alertCount } from './alerts';
import { getDroppedMetrics, getActiveIncidents } from '$lib/api';

const mockGetDroppedMetrics = vi.mocked(getDroppedMetrics);
const mockGetActiveIncidents = vi.mocked(getActiveIncidents);

function makeDroppedMetric(hostId: string) {
	return {
		host_id: hostId,
		host_name: `host-${hostId}`,
		dropped_at: new Date().toISOString(),
		reason: 'test'
	};
}

function makeIncident(id: string) {
	return {
		id,
		host_id: 'h1',
		host_name: 'host-1',
		rule_id: 'r1',
		rule_name: 'CPU > 90%',
		metric: 'cpu',
		threshold: 90,
		current_value: 95,
		started_at: new Date().toISOString(),
		acknowledged: false
	};
}

describe('alertsStore', () => {
	beforeEach(() => {
		alertsStore.clear();
		vi.clearAllMocks();
	});

	it('starts with empty state', () => {
		const state = get(alertsStore);
		expect(state.droppedMetrics).toEqual([]);
		expect(state.activeIncidents).toEqual([]);
		expect(state.loading).toBe(false);
		expect(state.error).toBeNull();
	});

	it('clear resets state', () => {
		alertsStore.clear();
		const state = get(alertsStore);
		expect(state.droppedMetrics).toEqual([]);
		expect(state.activeIncidents).toEqual([]);
	});

	it('load sets loading flag, then resolves with dropped metrics', async () => {
		mockGetDroppedMetrics.mockResolvedValueOnce({ dropped_metrics: [makeDroppedMetric('h1')] });
		const promise = alertsStore.load();
		expect(get(alertsStore).loading).toBe(true);
		await promise;
		const state = get(alertsStore);
		expect(state.loading).toBe(false);
		expect(state.droppedMetrics).toHaveLength(1);
		expect(state.error).toBeNull();
	});

	it('load sets error on API failure', async () => {
		mockGetDroppedMetrics.mockRejectedValueOnce(new Error('API error'));
		await alertsStore.load();
		const state = get(alertsStore);
		expect(state.loading).toBe(false);
		expect(state.error).toBe('API error');
	});

	it('load handles null dropped_metrics', async () => {
		mockGetDroppedMetrics.mockResolvedValueOnce({ dropped_metrics: null as unknown as [] });
		await alertsStore.load();
		expect(get(alertsStore).droppedMetrics).toEqual([]);
	});

	it('loadIncidents populates active incidents', async () => {
		mockGetActiveIncidents.mockResolvedValueOnce({ incidents: [makeIncident('i1')] });
		await alertsStore.loadIncidents();
		expect(get(alertsStore).activeIncidents).toHaveLength(1);
		expect(get(alertsStore).activeIncidents[0].id).toBe('i1');
	});

	it('loadIncidents handles API failure silently', async () => {
		mockGetActiveIncidents.mockRejectedValueOnce(new Error('Network error'));
		await expect(alertsStore.loadIncidents()).resolves.not.toThrow();
	});
});

describe('alertCount derived store', () => {
	beforeEach(() => {
		alertsStore.clear();
		vi.clearAllMocks();
	});

	it('reflects number of active incidents', async () => {
		expect(get(alertCount)).toBe(0);
		mockGetActiveIncidents.mockResolvedValueOnce({
			incidents: [makeIncident('i1'), makeIncident('i2')]
		});
		await alertsStore.loadIncidents();
		expect(get(alertCount)).toBe(2);
	});
});
