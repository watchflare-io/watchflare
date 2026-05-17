<script lang="ts">
	const {
		hasSecurityUpdate,
		availableVersion,
		updateChecked,
	}: {
		hasSecurityUpdate: boolean;
		availableVersion: string | null | undefined;
		updateChecked: boolean;
	} = $props();
</script>

{#if hasSecurityUpdate}
	<span class="shrink-0 inline-flex rounded-full border px-2 py-0.5 text-xs font-medium bg-destructive/10 text-destructive border-destructive/20">Security Update</span>
{:else if availableVersion}
	<span class="shrink-0 inline-flex rounded-full border px-2 py-0.5 text-xs font-medium bg-warning/10 text-warning border-warning/20">Outdated</span>
{:else if updateChecked}
	<span class="shrink-0 inline-flex rounded-full border px-2 py-0.5 text-xs font-medium bg-success/10 text-success border-success/20">Up to date</span>
{:else}
	<span class="shrink-0 inline-flex rounded-full border px-2 py-0.5 text-xs font-medium bg-muted text-muted-foreground border-border">Not checked</span>
{/if}
