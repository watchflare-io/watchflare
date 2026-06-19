import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';

// Mock the api module before importing the store
vi.mock('$lib/api', () => ({
	getHostMetrics: vi.fn()
}));

// Mock the constants module
vi.mock('$lib/constants', () => ({
	MAX_METRICS_POINTS_DASHBOARD: 5
}));

// Mock the utils module
vi.mock('$lib/utils', () => ({
	logger: { error: vi.fn(), warn: vi.fn(), log: vi.fn() }
}));

import { metricsStore, metricsData } from './metrics';

function fakeMetric(hostId: string, cpu: number) {
	return {
		host_id: hostId,
		timestamp: new Date().toISOString(),
		cpu_usage_percent: cpu,
		cpu_iowait_percent: 0,
		cpu_steal_percent: 0,
		memory_total_bytes: 1000,
		memory_used_bytes: 500,
		memory_available_bytes: 500,
		memory_buffers_bytes: 0,
		memory_cached_bytes: 0,
		swap_total_bytes: 0,
		swap_used_bytes: 0,
		disk_total_bytes: 2000,
		disk_used_bytes: 1000,
		processes_count: 0
	};
}

describe('metricsStore', () => {
	beforeEach(() => {
		metricsStore.clear();
	});

	it('starts with empty data', () => {
		const state = get(metricsStore);
		expect(state.data).toEqual({});
		expect(state.loading).toEqual({});
		expect(state.error).toBeNull();
	});

	it('updates host metrics via updateHostMetrics', () => {
		const metric = fakeMetric('s1', 50);
		metricsStore.updateHostMetrics('s1', metric);
		const data = get(metricsData);
		expect(data['s1']).toHaveLength(1);
		expect(data['s1'][0].cpu_usage_percent).toBe(50);
	});

	it('caps metrics at MAX_METRICS_POINTS_DASHBOARD', () => {
		// MAX is mocked to 5
		for (let i = 0; i < 8; i++) {
			metricsStore.updateHostMetrics('s1', fakeMetric('s1', i * 10));
		}
		const data = get(metricsData);
		expect(data['s1']).toHaveLength(5);
		// Should keep the last 5 (i=3..7 → 30,40,50,60,70)
		expect(data['s1'][0].cpu_usage_percent).toBe(30);
		expect(data['s1'][4].cpu_usage_percent).toBe(70);
	});

	it('keeps metrics separate per host', () => {
		metricsStore.updateHostMetrics('s1', fakeMetric('s1', 10));
		metricsStore.updateHostMetrics('s2', fakeMetric('s2', 20));
		const data = get(metricsData);
		expect(data['s1']).toHaveLength(1);
		expect(data['s2']).toHaveLength(1);
		expect(data['s1'][0].cpu_usage_percent).toBe(10);
		expect(data['s2'][0].cpu_usage_percent).toBe(20);
	});

	it('clears metrics for a specific host', () => {
		metricsStore.updateHostMetrics('s1', fakeMetric('s1', 10));
		metricsStore.updateHostMetrics('s2', fakeMetric('s2', 20));
		metricsStore.clearForHost('s1');
		const data = get(metricsData);
		expect(data['s1']).toBeUndefined();
		expect(data['s2']).toHaveLength(1);
	});

	it('clears all metrics', () => {
		metricsStore.updateHostMetrics('s1', fakeMetric('s1', 10));
		metricsStore.updateHostMetrics('s2', fakeMetric('s2', 20));
		metricsStore.clear();
		expect(get(metricsData)).toEqual({});
	});

	it('getForHost returns empty array for unknown host', () => {
		expect(metricsStore.getForHost('unknown')).toEqual([]);
	});
});
