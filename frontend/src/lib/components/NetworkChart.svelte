<script lang="ts">
	import UPlotChart from '$lib/components/UPlotChart.svelte';
	import { formatRate } from '$lib/chart-utils';
	import { userStore } from '$lib/stores/user';
	import type { Metric, AggregatedMetric, TimeRange } from '$lib/types';
	import type uPlot from 'uplot';

	let { data = [], timeRange }: { data: (Metric | AggregatedMetric)[]; timeRange?: TimeRange } =
		$props();

	const networkUnit = $derived($userStore.user?.network_unit ?? 'bytes');

	let chartData = $derived.by(() => {
		if (data.length === 0) return [[], [], []] as uPlot.AlignedData;
		const timestamps: number[] = [];
		const rx: (number | null)[] = [];
		const tx: (number | null)[] = [];
		for (const d of data) {
			timestamps.push(new Date(d.timestamp).getTime() / 1000);
			rx.push(d.network_rx_bytes_per_sec);
			tx.push(d.network_tx_bytes_per_sec);
		}
		return [timestamps, rx, tx] as uPlot.AlignedData;
	});

	const series = $derived<uPlot.Series[]>([
		{
			label: 'Download (RX)',
			stroke: 'var(--chart-7)',
			fill: 'var(--chart-7)',
			width: 2,
			value: (_u: uPlot, v: number | null) => (v != null ? formatRate(v, networkUnit) : '—')
		},
		{
			label: 'Upload (TX)',
			stroke: 'var(--chart-8)',
			fill: 'var(--chart-8)',
			width: 2,
			value: (_u: uPlot, v: number | null) => (v != null ? formatRate(v, networkUnit) : '—')
		}
	]);

	const axes = $derived<uPlot.Axis[]>([
		{},
		{
			values: (_u: uPlot, ticks: number[]) => ticks.map((v) => formatRate(v, networkUnit)),
			size: networkUnit === 'bits' ? 88 : 70
		}
	]);
</script>

<UPlotChart data={chartData} {series} {axes} {timeRange} />
