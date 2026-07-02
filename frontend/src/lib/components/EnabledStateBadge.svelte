<script lang="ts">
	const { state }: { state: string } = $props();

	const cls = $derived(
		state === 'enabled'
			? 'bg-success/10 text-success border-success/20'
			: 'bg-muted text-muted-foreground border-border'
	);
</script>

{#if state}
	<span class="shrink-0 inline-flex rounded-full border px-2 py-0.5 text-xs font-medium {cls}"
		>{state}</span
	>
{:else}
	<span class="text-sm text-muted-foreground">-</span>
{/if}
