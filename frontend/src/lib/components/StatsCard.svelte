<script>
	const { title, value, trend, trendLabel, icon, compact = false } = $props();

	const isPositive = $derived(trend >= 0);
	const trendColor = $derived(isPositive ? 'text-success' : 'text-destructive');
	const trendIcon = $derived(isPositive ? '↑' : '↓');
</script>

<div class="rounded-lg border bg-card {compact ? 'px-4 py-3' : 'p-6'}">
	<div class="flex {compact ? 'items-center' : 'items-start'} justify-between gap-3">
		<div class="{compact ? 'flex items-center gap-3 flex-1 min-w-0' : 'block flex-1 min-w-0'}">
			<p class="text-sm text-muted-foreground min-w-30 {compact ? '' : 'mb-1'}">{title}</p>
			<p class="{compact ? 'text-lg' : 'text-2xl'} font-semibold text-foreground leading-tight">{value}</p>
			{#if !compact}
				<div class="flex items-center gap-1 text-sm mt-2 h-5">
					{#if trend !== undefined}
						<span class="{trendColor} font-medium">{trendIcon}{Math.abs(trend).toFixed(1)}%</span>
						<span class="text-muted-foreground">{trendLabel || ''}</span>
					{/if}
				</div>
			{/if}
		</div>
		{#if icon}
			{@const Icon = icon}
			<div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
				<Icon class="h-5 w-5" />
			</div>
		{/if}
	</div>
</div>
