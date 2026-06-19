<script lang="ts">
	import { Select as SelectPrimitive } from 'bits-ui';
	import { cn } from '$lib/utils.js';
	import { ChevronDown } from 'lucide-svelte';

	let {
		ref = $bindable(null),
		class: className,
		items = [],
		children,
		...restProps
	}: SelectPrimitive.TriggerProps & { items?: string[] } = $props();
</script>

<SelectPrimitive.Trigger
	bind:ref
	data-slot="select-trigger"
	class={cn(
		'flex items-center justify-between gap-2 rounded-lg border bg-surface px-3 py-2 text-sm text-foreground outline-none focus-visible:ring-2 focus-visible:ring-primary/50 disabled:cursor-not-allowed disabled:opacity-50 [&>span]:line-clamp-1 min-w-max',
		className
	)}
	{...restProps}
>
	<div class="relative text-left">
		{@render children?.()}
		{#if items.length > 0}
			<div class="invisible h-0 overflow-hidden" aria-hidden="true">
				{#each items as item}
					<div>{item}</div>
				{/each}
			</div>
		{/if}
	</div>
	<ChevronDown class="h-4 w-4 shrink-0 opacity-50" />
</SelectPrimitive.Trigger>
