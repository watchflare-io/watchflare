import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';

vi.mock('$lib/api', () => ({
	getAggregatedMetrics: vi.fn()
}));

vi.mock('$lib/utils', () => ({
	logger: { error: vi.fn(), warn: vi.fn(), log: vi.fn() }
}));

vi.mock('$lib/constants', () => ({
	MAX_AGGREGATED_POINTS: 5
}));

import { aggregatedStore, aggregatedMetrics, currentTimeRange, dashboardStats } from './aggregated';
import { hostsStore } from './hosts';
import { getAggregatedMetrics } from '$lib/api';

const mockGetAggregatedMetrics = vi.mocked(getAggregatedMetrics);

function makeAggMetric(cpuPercent: number, overrides: Partial<Record<string, unknown>> = {}) {
	return {
		timestamp: new Date().toISOString(),
		cpu_usage_percent: cpuPercent,
		memory_total_bytes: 8_000_000_000,
		memory_used_bytes: 4_000_000_000,
		disk_total_bytes: 100_000_000_000,
		disk_used_bytes: 50_000_000_000,
		network_rx_bytes_per_sec: 0,
		network_tx_bytes_per_sec: 0,
		...overrides
	} as any;
}

describe('aggregatedStore', () => {
	beforeEach(() => {
		aggregatedStore.clear();
		hostsStore.clear();
		vi.clearAllMocks();
	});

	it('starts with empty state', () => {
		const state = get(aggregatedStore);
		expect(state.metrics).toEqual([]);
		expect(state.latestMetric).toBeNull();
		expect(state.loading).toBe(false);
		expect(state.error).toBeNull();
		expect(state.timeRange).toBe('1h');
	});

	it('load populates metrics and sets latestMetric from last point', async () => {
		const m1 = makeAggMetric(20);
		const m2 = makeAggMetric(40);
		mockGetAggregatedMetrics.mockResolvedValueOnce({ metrics: [m1, m2] });
		await aggregatedStore.load('1h');
		const state = get(aggregatedStore);
		expect(state.metrics).toHaveLength(2);
		expect(state.latestMetric?.cpu_usage_percent).toBe(40);
		expect(state.loading).toBe(false);
	});

	it('load sets error on failure', async () => {
		mockGetAggregatedMetrics.mockRejectedValueOnce(new Error('fetch failed'));
		await aggregatedStore.load('1h');
		expect(get(aggregatedStore).error).toBe('fetch failed');
	});

	it('setTimeRange updates timeRange', () => {
		aggregatedStore.setTimeRange('7d');
		expect(get(currentTimeRange)).toBe('7d');
	});

	it('clear resets all state', () => {
		aggregatedStore.setTimeRange('24h');
		aggregatedStore.clear();
		const state = get(aggregatedStore);
		expect(state.metrics).toEqual([]);
		expect(state.latestMetric).toBeNull();
		expect(state.timeRange).toBe('1h');
	});

	it('addMetricPoint in 1h view appends point', () => {
		const m = makeAggMetric(50);
		aggregatedStore.addMetricPoint(m);
		expect(get(aggregatedMetrics)).toHaveLength(1);
	});

	it('addMetricPoint in 1h view caps at MAX_AGGREGATED_POINTS (5)', () => {
		for (let i = 0; i < 7; i++) {
			aggregatedStore.addMetricPoint(makeAggMetric(i * 10));
		}
		expect(get(aggregatedMetrics)).toHaveLength(5);
	});

	it('addMetricPoint always updates latestMetric', () => {
		const m = makeAggMetric(75);
		aggregatedStore.addMetricPoint(m);
		expect(get(aggregatedStore).latestMetric?.cpu_usage_percent).toBe(75);
	});

	it('addMetricPoint in non-1h view updates latestMetric without appending', () => {
		aggregatedStore.setTimeRange('7d');
		const m = makeAggMetric(60);
		aggregatedStore.addMetricPoint(m);
		// metrics array stays empty (only latestMetric is updated for non-1h without bucket completion)
		expect(get(aggregatedStore).latestMetric?.cpu_usage_percent).toBe(60);
	});
});

describe('dashboardStats derived store', () => {
	beforeEach(() => {
		aggregatedStore.clear();
		hostsStore.clear();
	});

	it('returns zeros when no data', () => {
		const stats = get(dashboardStats);
		expect(stats.avgCPU).toBe(0);
		expect(stats.totalHosts).toBe(0);
		expect(stats.onlineHosts).toBe(0);
	});

	it('reflects latestMetric data', () => {
		aggregatedStore.addMetricPoint(makeAggMetric(55));
		const stats = get(dashboardStats);
		expect(stats.avgCPU).toBe(55);
		expect(stats.avgMemory).toBe(50); // 4GB/8GB
		expect(stats.avgDisk).toBe(50); // 50GB/100GB
	});

	it('cpuTrend is 0 when no 24h baseline', () => {
		aggregatedStore.addMetricPoint(makeAggMetric(55));
		const stats = get(dashboardStats);
		expect(stats.cpuTrend).toBe(0);
	});

	it('offlineHosts = totalHosts - onlineHosts', () => {
		const stats = get(dashboardStats);
		expect(stats.offlineHosts).toBe(stats.totalHosts - stats.onlineHosts);
	});
});
