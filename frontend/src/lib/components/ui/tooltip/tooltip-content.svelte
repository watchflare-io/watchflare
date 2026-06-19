<script lang="ts">
	import { Tooltip as TooltipPrimitive } from 'bits-ui';
	import { cn } from '$lib/utils.js';
	import TooltipPortal from './tooltip-portal.svelte';
	import type { ComponentProps } from 'svelte';
	import type { WithoutChildrenOrChild } from '$lib/utils.js';

	let {
		ref = $bindable(null),
		class: className,
		sideOffset = 6,
		side = 'top',
		children,
		portalProps,
		...restProps
	}: TooltipPrimitive.ContentProps & {
		portalProps?: WithoutChildrenOrChild<ComponentProps<typeof TooltipPortal>>;
	} = $props();
</script>

<TooltipPortal {...portalProps}>
	<TooltipPrimitive.Content
		bind:ref
		data-slot="tooltip-content"
		{sideOffset}
		{side}
		class={cn(
			'bg-popover text-popover-foreground border shadow-sm z-50 w-fit rounded-md px-3 py-1.5 text-xs text-balance',
			'animate-in fade-in-0 zoom-in-95 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95',
			'data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-end-2 data-[side=right]:slide-in-from-start-2 data-[side=top]:slide-in-from-bottom-2',
			className
		)}
		{...restProps}
	>
		{@render children?.()}
		<TooltipPrimitive.Arrow>
			{#snippet children()}
				<svg
					width="12"
					height="6"
					viewBox="0 0 12 8"
					style="display: block; margin-top: {side === 'top'
						? '-1px'
						: side === 'bottom'
							? '1px'
							: '0'};"
				>
					<path d="M0,0 L5.5,6.3 Q6,7.2 6.5,6.3 L12,0 Z" class="fill-popover" />
					<path
						d="M0,0 L5.5,6.3 Q6,7.2 6.5,6.3 L12,0"
						class="stroke-border fill-none"
						stroke-width="1"
						stroke-linejoin="round"
						stroke-linecap="butt"
						vector-effect="non-scaling-stroke"
					/>
				</svg>
			{/snippet}
		</TooltipPrimitive.Arrow>
	</TooltipPrimitive.Content>
</TooltipPortal>
