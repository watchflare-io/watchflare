<script lang="ts">
	import UPlotChart from '$lib/components/UPlotChart.svelte';
	import { dynamicChartColor } from '$lib/chart-utils';
	import type { TimeRange } from '$lib/types';
	import type uPlot from 'uplot';

	let { pivotedData = [], containerNames = [], timeRange }: {
		pivotedData: Record<string, unknown>[];
		containerNames: string[];
		timeRange?: TimeRange;
	} = $props();

	let chartData = $derived.by(() => {
		if (pivotedData.length === 0 || containerNames.length === 0) return [[]] as uPlot.AlignedData;
		const timestamps: number[] = [];
		const columns: (number | null)[][] = containerNames.map(() => []);
		for (const row of pivotedData) {
			timestamps.push((row.date as Date).getTime() / 1000);
			for (let i = 0; i < containerNames.length; i++) {
				const val = row[containerNames[i]];
				columns[i].push(val != null ? val as number : null);
			}
		}
		return [timestamps, ...columns] as uPlot.AlignedData;
	});

	let series = $derived(
		containerNames.map((name, i): uPlot.Series => ({
			label: name,
			stroke: dynamicChartColor(i, containerNames.length, 0),
			fill: dynamicChartColor(i, containerNames.length, 0),
			width: 2,
			value: (_u: uPlot, v: number | null) => v != null ? v.toFixed(1) + '%' : '—',
		}))
	);

	const axes: uPlot.Axis[] = [
		{},
		{ values: (_u: uPlot, ticks: number[]) => ticks.map(v => v.toFixed(0) + '%') }
	];

</script>

<UPlotChart data={chartData} {series} {axes} {timeRange} />
