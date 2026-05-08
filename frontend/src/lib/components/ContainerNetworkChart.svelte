<script lang="ts">
	import UPlotChart from '$lib/components/UPlotChart.svelte';
	import { formatRate, dynamicChartColor } from '$lib/chart-utils';
	import { userStore } from '$lib/stores/user';
	import type { TimeRange } from '$lib/types';
	import type uPlot from 'uplot';

	let { pivotedData = [], seriesKeys = [], timeRange }: {
		pivotedData: Record<string, unknown>[];
		seriesKeys: string[];
		timeRange?: TimeRange;
	} = $props();

	const networkUnit = $derived($userStore.user?.network_unit ?? 'bytes');

	let chartData = $derived.by(() => {
		if (pivotedData.length === 0 || seriesKeys.length === 0) return [[]] as uPlot.AlignedData;
		const timestamps: number[] = [];
		const columns: (number | null)[][] = seriesKeys.map(() => []);
		for (const row of pivotedData) {
			timestamps.push((row.date as Date).getTime() / 1000);
			for (let i = 0; i < seriesKeys.length; i++) {
				const val = row[seriesKeys[i]];
				columns[i].push(val != null ? val as number : null);
			}
		}
		return [timestamps, ...columns] as uPlot.AlignedData;
	});

	let series = $derived(
		seriesKeys.map((key, i): uPlot.Series => ({
			label: key,
			stroke: dynamicChartColor(i, seriesKeys.length, 240),
			fill: dynamicChartColor(i, seriesKeys.length, 240),
			width: 2,
			value: (_u: uPlot, v: number | null) => v != null ? formatRate(v, networkUnit) : '—',
		}))
	);

	const axes = $derived<uPlot.Axis[]>([
		{},
		{
			values: (_u: uPlot, ticks: number[]) => ticks.map(v => formatRate(v, networkUnit)),
			size: networkUnit === 'bits' ? 88 : 70,
		}
	]);
</script>

<UPlotChart data={chartData} {series} {axes} {timeRange} />
