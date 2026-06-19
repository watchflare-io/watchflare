<script lang="ts">
	import UPlotChart from '$lib/components/UPlotChart.svelte';
	import type { Metric, AggregatedMetric, TimeRange } from '$lib/types';
	import type uPlot from 'uplot';

	let { data = [], timeRange }: { data: (Metric | AggregatedMetric)[]; timeRange?: TimeRange } =
		$props();

	let chartData = $derived.by(() => {
		if (data.length === 0) return [[], [], [], []] as uPlot.AlignedData;
		const timestamps: number[] = [];
		const l1: (number | null)[] = [];
		const l5: (number | null)[] = [];
		const l15: (number | null)[] = [];
		for (const d of data) {
			timestamps.push(new Date(d.timestamp).getTime() / 1000);
			l1.push(d.load_avg_1min);
			l5.push(d.load_avg_5min);
			l15.push(d.load_avg_15min);
		}
		return [timestamps, l1, l5, l15] as uPlot.AlignedData;
	});

	const series: uPlot.Series[] = [
		{
			label: '1 min',
			stroke: 'var(--chart-4)',
			width: 2,
			value: (_u: uPlot, v: number | null) => (v != null ? v.toFixed(2) : '—')
		},
		{
			label: '5 min',
			stroke: 'var(--chart-19)',
			width: 2,
			value: (_u: uPlot, v: number | null) => (v != null ? v.toFixed(2) : '—')
		},
		{
			label: '15 min',
			stroke: 'var(--chart-6)',
			width: 2,
			value: (_u: uPlot, v: number | null) => (v != null ? v.toFixed(2) : '—')
		}
	];
</script>

<UPlotChart data={chartData} {series} {timeRange} />
