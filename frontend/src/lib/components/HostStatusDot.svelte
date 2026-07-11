<script lang="ts">
	const { status, title = undefined }: { status: string; title?: string } = $props();

	const color = $derived(
		status === 'online'
			? 'bg-success'
			: status === 'offline'
				? 'bg-danger'
				: status === 'pending' || status === 'ip_mismatch'
					? 'bg-warning'
					: 'bg-muted-foreground'
	);
</script>

<span
	class="inline-block h-2 w-2 shrink-0 rounded-full {color}"
	role="img"
	aria-label={title ?? status}
	{title}
></span>
