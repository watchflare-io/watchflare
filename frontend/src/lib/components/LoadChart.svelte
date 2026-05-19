<script lang="ts">
	import UPlotChart from '$lib/components/UPlotChart.svelte';
	import type { AggregatedMetric, TimeRange } from '$lib/types';
	import type uPlot from 'uplot';

	let { data = [], timeRange }: { data: AggregatedMetric[]; timeRange?: TimeRange } = $props();

	let chartData = $derived.by(() => {
		if (data.length === 0) return [[], [], [], []] as uPlot.AlignedData;
		const timestamps: number[] = [];
		const load1: (number | null)[] = [];
		const load5: (number | null)[] = [];
		const load15: (number | null)[] = [];
		for (const d of data) {
			timestamps.push(new Date(d.timestamp).getTime() / 1000);
			load1.push(d.load_avg_1min);
			load5.push(d.load_avg_5min);
			load15.push(d.load_avg_15min);
		}
		return [timestamps, load1, load5, load15] as uPlot.AlignedData;
	});

	const series: uPlot.Series[] = [
		{
			label: '1 min',
			stroke: 'var(--chart-4)',
			width: 2,
			value: (_u: uPlot, v: number | null) => v != null ? v.toFixed(2) : '—',
		},
		{
			label: '5 min',
			stroke: 'var(--chart-19)',
			width: 2,
			value: (_u: uPlot, v: number | null) => v != null ? v.toFixed(2) : '—',
		},
		{
			label: '15 min',
			stroke: 'var(--chart-6)',
			width: 2,
			value: (_u: uPlot, v: number | null) => v != null ? v.toFixed(2) : '—',
		},
	];

	const axes: uPlot.Axis[] = [
		{},
		{ values: (_u: uPlot, ticks: number[]) => ticks.map(v => v.toFixed(1)) }
	];
</script>

<UPlotChart data={chartData} {series} {axes} {timeRange} />
