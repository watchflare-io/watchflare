import { writable, derived } from 'svelte/store';
import type { AggregatedMetric, TimeRange } from '$lib/types';
import { getAggregatedMetrics } from '$lib/api';
import { hostStatsStore } from './hosts';
import { logger } from '$lib/utils';
import { MAX_AGGREGATED_POINTS } from '$lib/constants';

interface AggregatedState {
	// Current time range metrics (for charts)
	metrics: AggregatedMetric[];
	// Latest real-time metric (for stats cards, independent of time range)
	latestMetric: AggregatedMetric | null;
	// Current time range
	timeRange: TimeRange;
	loading: boolean;
	error: string | null;
}

function createAggregatedStore() {
	const { subscribe, set, update } = writable<AggregatedState>({
		metrics: [],
		latestMetric: null,
		timeRange: '1h',
		loading: false,
		error: null
	});

	// Bucket sizes in ms for each time range (must match backend intervals)
	const bucketMs: Record<string, number> = {
		'12h': 10 * 60 * 1000,
		'24h': 15 * 60 * 1000,
		'7d': 2 * 60 * 60 * 1000,
		'30d': 8 * 60 * 60 * 1000
	};

	// Guard to prevent concurrent loads (race condition from rapid SSE updates)
	let loadInFlight = false;

	// Extracted as local function so addMetricPoint can call it
	async function load(timeRange: TimeRange): Promise<void> {
		if (loadInFlight) return;
		loadInFlight = true;
		update(state => ({ ...state, loading: true, error: null, timeRange }));

		try {
			const data = await getAggregatedMetrics(timeRange);
			const metricsArray = data.metrics || [];

			update(state => ({
				...state,
				metrics: metricsArray,
				// Initialize latestMetric from loaded data if not yet set
				latestMetric: state.latestMetric || (metricsArray.length > 0 ? metricsArray[metricsArray.length - 1] : null),
				loading: false
			}));
		} catch (err) {
			const error = err instanceof Error ? err.message : 'Failed to load aggregated metrics';
			update(state => ({ ...state, loading: false, error }));
			logger.error('Failed to load aggregated metrics:', err);
		} finally {
			loadInFlight = false;
		}
	}

	return {
		subscribe,
		load,

		// Update metrics (add new point from SSE)
		addMetricPoint(metric: AggregatedMetric): void {
			let shouldReload = false;
			let reloadTimeRange: TimeRange = '1h';

			update(state => {
				// Always update latestMetric for real-time stats cards
				const newState = { ...state, latestMetric: metric };

				if (newState.timeRange === '1h') {
					// 1h view: add real-time 30s points
					let updatedMetrics = [...newState.metrics, metric];
					if (updatedMetrics.length > MAX_AGGREGATED_POINTS) {
						updatedMetrics = updatedMetrics.slice(-MAX_AGGREGATED_POINTS);
					}
					return { ...newState, metrics: updatedMetrics };
				}

				// For non-1h ranges: check if a new completed bucket exists
				const bucket = bucketMs[newState.timeRange];
				if (bucket && !newState.loading) {
					const now = Date.now();
					// Last completed bucket end (= labels use bucket end)
					const lastCompleteBucketEnd = Math.floor(now / bucket) * bucket;
					const lastPoint = newState.metrics[newState.metrics.length - 1];
					const lastPointTime = lastPoint ? new Date(lastPoint.timestamp).getTime() : 0;

					if (lastCompleteBucketEnd > lastPointTime) {
						// New completed bucket available - reload from API
						shouldReload = true;
						reloadTimeRange = newState.timeRange;
					}
				}

				return newState;
			});

			if (shouldReload) {
				load(reloadTimeRange);
			}
		},

		// Change time range
		setTimeRange(timeRange: TimeRange): void {
			update(state => ({ ...state, timeRange }));
		},

		// Clear all data
		clear(): void {
			set({
				metrics: [],
				latestMetric: null,
				timeRange: '1h',
				loading: false,
				error: null
			});
		}
	};
}

export const aggregatedStore = createAggregatedStore();

// Derived stores for convenience
export const aggregatedMetrics = derived(aggregatedStore, $store => $store.metrics);
export const currentTimeRange = derived(aggregatedStore, $store => $store.timeRange);

// Derived store for computed stats (memoized to avoid recalculation on irrelevant store changes)
let cachedStats: ReturnType<typeof computeStats> | null = null;
let cachedLastPoint: AggregatedMetric | null = null;
let cachedOnlineCount = -1;
let cachedTotalCount = -1;

function computeStats(
	lastPoint: AggregatedMetric | null,
	totalHosts: number,
	onlineHosts: number
) {
	const avgCPU = lastPoint?.cpu_usage_percent || 0;
	const totalMemory = lastPoint?.memory_total_bytes || 0;
	const usedMemory = lastPoint?.memory_used_bytes || 0;
	const totalDisk = lastPoint?.disk_total_bytes || 0;
	const usedDisk = lastPoint?.disk_used_bytes || 0;

	const loadAvg = lastPoint?.load_avg_1min || 0;
	const loadAvg5 = lastPoint?.load_avg_5min || 0;
	const loadAvg15 = lastPoint?.load_avg_15min || 0;

	return {
		totalHosts,
		onlineHosts,
		offlineHosts: totalHosts - onlineHosts,
		avgCPU,
		cpuTrend: 0,
		avgMemory: totalMemory > 0 ? (usedMemory / totalMemory) * 100 : 0,
		avgDisk: totalDisk > 0 ? (usedDisk / totalDisk) * 100 : 0,
		totalMemory,
		usedMemory,
		totalDisk,
		usedDisk,
		loadAvg,
		loadAvg5,
		loadAvg15,
	};
}

export const dashboardStats = derived(
	[aggregatedStore, hostStatsStore],
	([$aggregated, $hostStats]) => {
		// Use latestMetric (real-time SSE) for stats cards, independent of time range
		const lastPoint = $aggregated.latestMetric;
		const onlineCount = $hostStats.online;
		const totalCount = $hostStats.total;

		// Skip recalculation if inputs haven't changed (compare by value for SSE objects)
		if (
			cachedStats &&
			lastPoint?.timestamp === cachedLastPoint?.timestamp &&
			lastPoint?.cpu_usage_percent === cachedLastPoint?.cpu_usage_percent &&
			lastPoint?.memory_used_bytes === cachedLastPoint?.memory_used_bytes &&
			onlineCount === cachedOnlineCount &&
			totalCount === cachedTotalCount
		) {
			return cachedStats;
		}

		cachedLastPoint = lastPoint;
		cachedOnlineCount = onlineCount;
		cachedTotalCount = totalCount;
		cachedStats = computeStats(lastPoint, totalCount, onlineCount);
		return cachedStats;
	}
);
