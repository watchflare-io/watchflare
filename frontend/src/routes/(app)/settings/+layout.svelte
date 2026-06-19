<script lang="ts">
	import { page } from '$app/stores';
	import { settingsItems } from '$lib/navigation';

	const { children } = $props();
</script>

<div class="mb-6">
	<h1 class="text-xl sm:text-2xl font-semibold text-foreground mb-3">Settings</h1>
	<div class="flex gap-1 border-b border-border">
		{#each settingsItems as item}
			<a
				href={item.href}
				aria-current={$page.url.pathname === item.href ? 'page' : undefined}
				class="shrink-0 px-4 py-2.5 text-sm font-medium transition-colors border-b-2 -mb-px {$page
					.url.pathname === item.href
					? 'border-primary text-foreground'
					: 'border-transparent text-muted-foreground hover:text-foreground'}"
			>
				{item.label}
			</a>
		{/each}
	</div>
</div>

{@render children()}
