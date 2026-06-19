<script lang="ts">
	import { DropdownMenu as DropdownMenuPrimitive } from 'bits-ui';
	import { cn } from '$lib/utils.js';

	let { class: className, children, ...restProps }: DropdownMenuPrimitive.ItemProps = $props();
</script>

<DropdownMenuPrimitive.Item
	data-slot="dropdown-menu-item"
	class={cn(
		'flex w-full cursor-pointer select-none items-center gap-2 rounded-md px-3 py-2 text-sm text-foreground outline-none',
		'data-highlighted:bg-muted',
		'data-disabled:pointer-events-none data-disabled:opacity-50',
		className
	)}
	{...restProps}
>
	{@render children?.()}
</DropdownMenuPrimitive.Item>
