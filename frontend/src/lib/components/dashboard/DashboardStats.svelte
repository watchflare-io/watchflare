<script lang="ts">
	import { ArrowUp, ArrowDown } from 'lucide-svelte';
	import { type TimeRange, type ActiveIncident } from '$lib/types';

	interface TrendItem {
		direction: 'up' | 'down' | 'stable';
		delta: number;
	}

	interface Trends {
		cpu: TrendItem;
		memory: TrendItem;
		disk: TrendItem;
		loadAvg: TrendItem;
	}

	interface Stats {
		totalHosts: number;
		onlineHosts: number;
		offlineHosts: number;
		avgCPU: number;
		avgMemory: number;
		avgDisk: number;
		usedMemory: number;
		totalMemory: number;
		loadAvg: number;
		loadAvg5: number;
		loadAvg15: number;
	}

	const {
		stats,
		trends,
		timeRange,
		hasSufficientTrendData,
		packagesStats,
		activeIncidents
	}: {
		stats: Stats;
		trends: Trends;
		timeRange: TimeRange;
		hasSufficientTrendData: boolean;
		packagesStats: {
			outdatedCount: number;
			securityCount: number;
			outdatedHostsCount: number;
			securityHostsCount: number;
		};
		activeIncidents: ActiveIncident[];
	} = $props();

	function valueColor(pct: number): string {
		if (pct >= 85) return 'text-destructive';
		if (pct >= 70) return 'text-warning';
		return 'text-foreground';
	}

	const memoryPct = $derived(
		stats.totalMemory > 0 ? (stats.usedMemory / stats.totalMemory) * 100 : 0
	);
</script>

<!-- display:contents lets each card become a direct grid item of the parent bento grid -->
<div class="contents">
	<!-- Hosts -->
	<a href="/hosts" class="kpi-card">
		<p class="kpi-label">Hosts</p>
		<p class="kpi-value">
			{stats.onlineHosts}<span class="kpi-unit">/{stats.totalHosts}</span>
		</p>
		<p
			class="kpi-delta mt-auto pt-1 {stats.offlineHosts > 0 ? 'text-destructive' : 'text-success'}"
		>
			{#if stats.offlineHosts > 0}
				{stats.offlineHosts} offline
			{:else if stats.totalHosts > 0}
				all online
			{:else}
				no hosts yet
			{/if}
		</p>
	</a>

	<!-- CPU -->
	<div class="kpi-card">
		<p class="kpi-label">CPU</p>
		<p class="kpi-value {valueColor(stats.avgCPU)}">
			{stats.avgCPU.toFixed(1)}<span class="kpi-unit">%</span>
		</p>

		<div class="kpi-trend">
			{#if hasSufficientTrendData && trends.cpu.direction === 'up'}
				<ArrowUp class="h-3 w-3 text-destructive" />
				<span class="text-destructive">{Math.abs(trends.cpu.delta).toFixed(1)}%</span>
				<span class="text-muted-foreground">vs {timeRange}</span>
			{:else if hasSufficientTrendData && trends.cpu.direction === 'down'}
				<ArrowDown class="h-3 w-3 text-success" />
				<span class="text-success">{Math.abs(trends.cpu.delta).toFixed(1)}%</span>
				<span class="text-muted-foreground">vs {timeRange}</span>
			{:else if hasSufficientTrendData}
				<span class="text-muted-foreground">stable</span>
			{:else}
				<span class="text-muted-foreground">—</span>
			{/if}
		</div>
	</div>

	<!-- Memory -->
	<div class="kpi-card">
		<p class="kpi-label">Memory</p>
		<p class="kpi-value {valueColor(memoryPct)}">
			{memoryPct.toFixed(1)}<span class="kpi-unit">%</span>
		</p>

		<div class="kpi-trend">
			{#if hasSufficientTrendData && trends.memory.direction === 'up'}
				<ArrowUp class="h-3 w-3 text-destructive" />
				<span class="text-destructive">{Math.abs(trends.memory.delta).toFixed(1)}%</span>
				<span class="text-muted-foreground">vs {timeRange}</span>
			{:else if hasSufficientTrendData && trends.memory.direction === 'down'}
				<ArrowDown class="h-3 w-3 text-success" />
				<span class="text-success">{Math.abs(trends.memory.delta).toFixed(1)}%</span>
				<span class="text-muted-foreground">vs {timeRange}</span>
			{:else if hasSufficientTrendData}
				<span class="text-muted-foreground">stable</span>
			{:else}
				<span class="text-muted-foreground">—</span>
			{/if}
		</div>
	</div>

	<!-- Disk -->
	<div class="kpi-card">
		<p class="kpi-label">Disk</p>
		<p class="kpi-value {valueColor(stats.avgDisk)}">
			{stats.avgDisk.toFixed(1)}<span class="kpi-unit">%</span>
		</p>

		<div class="kpi-trend">
			{#if hasSufficientTrendData && trends.disk.direction === 'up'}
				<ArrowUp class="h-3 w-3 text-destructive" />
				<span class="text-destructive">{Math.abs(trends.disk.delta).toFixed(1)}%</span>
				<span class="text-muted-foreground">vs {timeRange}</span>
			{:else if hasSufficientTrendData && trends.disk.direction === 'down'}
				<ArrowDown class="h-3 w-3 text-success" />
				<span class="text-success">{Math.abs(trends.disk.delta).toFixed(1)}%</span>
				<span class="text-muted-foreground">vs {timeRange}</span>
			{:else if hasSufficientTrendData}
				<span class="text-muted-foreground">stable</span>
			{:else}
				<span class="text-muted-foreground">—</span>
			{/if}
		</div>
	</div>

	<!-- Load Average -->
	<div class="kpi-card">
		<p class="kpi-label">Load avg</p>
		<p class="kpi-value text-foreground tabular-nums">
			{stats.loadAvg.toFixed(2)}
			<span class="kpi-unit">· {stats.loadAvg5.toFixed(2)} · {stats.loadAvg15.toFixed(2)}</span>
		</p>
		<div class="kpi-trend">
			{#if hasSufficientTrendData && trends.loadAvg.direction === 'up'}
				<ArrowUp class="h-3 w-3 text-destructive" />
				<span class="text-destructive">{Math.abs(trends.loadAvg.delta).toFixed(2)}</span>
				<span class="text-muted-foreground">vs {timeRange}</span>
			{:else if hasSufficientTrendData && trends.loadAvg.direction === 'down'}
				<ArrowDown class="h-3 w-3 text-success" />
				<span class="text-success">{Math.abs(trends.loadAvg.delta).toFixed(2)}</span>
				<span class="text-muted-foreground">vs {timeRange}</span>
			{:else if hasSufficientTrendData}
				<span class="text-muted-foreground">stable</span>
			{:else}
				<span class="text-muted-foreground">—</span>
			{/if}
		</div>
	</div>

	<!-- Outdated Packages -->
	<a href="/packages?status=outdated" class="kpi-card">
		<p class="kpi-label">Outdated</p>
		<p class="kpi-value {packagesStats.outdatedCount > 0 ? 'text-warning' : 'text-foreground'}">
			{packagesStats.outdatedCount}
		</p>
		<p class="kpi-delta mt-auto pt-1 text-muted-foreground">
			{#if packagesStats.outdatedCount > 0}
				{packagesStats.outdatedHostsCount} host{packagesStats.outdatedHostsCount !== 1 ? 's' : ''} affected
			{:else}
				up to date
			{/if}
		</p>
	</a>

	<!-- Security Updates -->
	<a href="/packages?status=security" class="kpi-card">
		<p class="kpi-label">Security</p>
		<p class="kpi-value {packagesStats.securityCount > 0 ? 'text-destructive' : 'text-foreground'}">
			{packagesStats.securityCount}
		</p>
		<p class="kpi-delta mt-auto pt-1 text-muted-foreground">
			{#if packagesStats.securityCount > 0}
				{packagesStats.securityHostsCount} host{packagesStats.securityHostsCount !== 1 ? 's' : ''} affected
			{:else}
				no vulnerabilities
			{/if}
		</p>
	</a>

	<!-- Active Alerts -->
	<div class="kpi-card">
		<p class="kpi-label">Alerts</p>
		<p class="kpi-value {activeIncidents.length > 0 ? 'text-destructive' : 'text-foreground'}">
			{activeIncidents.length}
		</p>
		{#if activeIncidents.length > 0}
			<div class="kpi-delta mt-auto pt-1 flex flex-col gap-0.5">
				{#each activeIncidents.slice(0, 1) as incident}
					<p class="truncate text-muted-foreground">
						<span class="text-foreground font-medium">{incident.host_name}</span>
					</p>
				{/each}
				{#if activeIncidents.length > 1}
					<p class="text-muted-foreground">+{activeIncidents.length - 1} more</p>
				{/if}
			</div>
		{:else}
			<p class="kpi-delta mt-auto pt-1 text-muted-foreground">all clear</p>
		{/if}
	</div>
</div>
