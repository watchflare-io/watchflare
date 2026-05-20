<script lang="ts">
	import UPlotChart from '$lib/components/UPlotChart.svelte';
	import type { AggregatedMetric, TimeRange } from '$lib/types';
	import type uPlot from 'uplot';

	let { data = [], timeRange }: { data: AggregatedMetric[]; timeRange?: TimeRange } = $props();

	let chartData = $derived.by(() => {
		if (data.length === 0) return [[], []] as uPlot.AlignedData;
		const timestamps: number[] = [];
		const disk: (number | null)[] = [];
		for (const d of data) {
			timestamps.push(new Date(d.timestamp).getTime() / 1000);
			disk.push(d.disk_total_bytes > 0 ? (d.disk_used_bytes / d.disk_total_bytes) * 100 : null);
		}
		return [timestamps, disk] as uPlot.AlignedData;
	});

	const series: uPlot.Series[] = [
		{
			label: 'Disk Usage',
			stroke: 'var(--chart-3)',
			fill: 'var(--chart-3)',
			width: 2,
			value: (_u: uPlot, v: number | null) => v != null ? v.toFixed(1) + '%' : '—',
		}
	];

	const scales: uPlot.Scales = { y: { range: [0, 100] } };

	const axes: uPlot.Axis[] = [
		{},
		{ values: (_u: uPlot, ticks: number[]) => ticks.map(v => v + '%') }
	];
</script>

<UPlotChart data={chartData} {series} {axes} {scales} {timeRange} />
