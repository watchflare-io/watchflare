<script lang="ts">
	import UPlotChart from '$lib/components/UPlotChart.svelte';
	import { formatBytes } from '$lib/utils';
	import type { Metric, TimeRange } from '$lib/types';
	import type uPlot from 'uplot';

	let { data = [], timeRange }: { data: Metric[]; timeRange?: TimeRange } = $props();

	let chartData = $derived.by(() => {
		if (data.length === 0) return [[], []] as uPlot.AlignedData;
		const timestamps: number[] = [];
		const swap: (number | null)[] = [];
		for (const d of data) {
			timestamps.push(new Date(d.timestamp).getTime() / 1000);
			swap.push(d.swap_used_bytes);
		}
		return [timestamps, swap] as uPlot.AlignedData;
	});

	function roundBytes(max: number): number {
		const GB = 1024 ** 3;
		const MB = 1024 ** 2;
		const unit = max >= GB ? GB : MB;
		return Math.round(max / unit) * unit;
	}

	const scales: uPlot.Scales = {
		y: {
			range: () => {
				const max = data.length > 0 ? Math.max(...data.map((d) => d.swap_total_bytes)) : 0;
				return [0, max > 0 ? roundBytes(max) : 1] as uPlot.Range.MinMax;
			}
		}
	};

	const series: uPlot.Series[] = [
		{
			label: 'Swap Used',
			stroke: 'var(--chart-17)',
			fill: 'var(--chart-17)',
			width: 2,
			fillTo: 0,
			value: (_u: uPlot, v: number | null) => v != null ? formatBytes(v) : '—',
		}
	];

	const axes: uPlot.Axis[] = [
		{},
		{
			values: (_u: uPlot, vals: number[]) => vals.map(v => formatBytes(v)),
			size: 70,
		}
	];
</script>

<UPlotChart data={chartData} {series} {axes} {scales} {timeRange} />
