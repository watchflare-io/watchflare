<script lang="ts">
	import { Select as SelectPrimitive } from 'bits-ui';
	import { cn } from '$lib/utils.js';

	let { class: className, label, children, ...restProps }: SelectPrimitive.ItemProps = $props();
</script>

<SelectPrimitive.Item
	data-slot="select-item"
	{label}
	class={cn(
		'flex w-full cursor-pointer select-none items-center rounded-md px-3 py-2 text-sm text-foreground outline-none',
		'data-highlighted:bg-muted',
		'data-disabled:pointer-events-none data-disabled:opacity-50',
		className
	)}
	{...restProps}
>
	{@render children?.()}
</SelectPrimitive.Item>
