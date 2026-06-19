<script lang="ts">
	import { DropdownMenu as DropdownMenuPrimitive } from 'bits-ui';
	import { cn } from '$lib/utils.js';

	let {
		ref = $bindable(null),
		class: className,
		sideOffset = 6,
		children,
		...restProps
	}: DropdownMenuPrimitive.ContentProps & { sideOffset?: number } = $props();
</script>

<DropdownMenuPrimitive.Content
	bind:ref
	data-slot="dropdown-menu-content"
	{sideOffset}
	class={cn(
		'bg-surface z-50 min-w-[12rem] origin-(--bits-dropdown-menu-content-transform-origin) overflow-y-auto rounded-lg border p-1.5 shadow-md',
		'data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95',
		'data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95',
		'data-[side=bottom]:slide-in-from-top-2 data-[side=top]:slide-in-from-bottom-2',
		className
	)}
	{...restProps}
>
	{@render children?.()}
</DropdownMenuPrimitive.Content>
