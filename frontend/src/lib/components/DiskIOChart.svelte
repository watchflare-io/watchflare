<script lang="ts">
	import UPlotChart from '$lib/components/UPlotChart.svelte';
	import { formatRate } from '$lib/chart-utils';
	import { userStore } from '$lib/stores/user';
	import type { Metric, AggregatedMetric, TimeRange } from '$lib/types';
	import type uPlot from 'uplot';

	let { data = [], timeRange }: { data: (Metric | AggregatedMetric)[]; timeRange?: TimeRange } =
		$props();

	const diskUnit = $derived($userStore.user?.disk_unit ?? 'bytes');

	let chartData = $derived.by(() => {
		if (data.length === 0) return [[], [], []] as uPlot.AlignedData;
		const timestamps: number[] = [];
		const read: (number | null)[] = [];
		const write: (number | null)[] = [];
		for (const d of data) {
			timestamps.push(new Date(d.timestamp).getTime() / 1000);
			read.push(d.disk_read_bytes_per_sec);
			write.push(d.disk_write_bytes_per_sec);
		}
		return [timestamps, read, write] as uPlot.AlignedData;
	});

	const series = $derived<uPlot.Series[]>([
		{
			label: 'Read',
			stroke: 'var(--chart-9)',
			fill: 'var(--chart-9)',
			width: 2,
			value: (_u: uPlot, v: number | null) => v != null ? formatRate(v, diskUnit) : '—',
		},
		{
			label: 'Write',
			stroke: 'var(--chart-10)',
			fill: 'var(--chart-10)',
			width: 2,
			value: (_u: uPlot, v: number | null) => v != null ? formatRate(v, diskUnit) : '—',
		}
	]);

	const axes = $derived<uPlot.Axis[]>([
		{},
		{
			values: (_u: uPlot, ticks: number[]) => ticks.map(v => formatRate(v, diskUnit)),
			size: diskUnit === 'bits' ? 88 : 70,
		}
	]);
</script>

<UPlotChart data={chartData} {series} {axes} {timeRange} />
