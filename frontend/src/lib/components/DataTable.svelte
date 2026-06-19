<script lang="ts">
	import type { Snippet } from 'svelte';
	import { onMount } from 'svelte';
	import { cn } from '$lib/utils.js';

	const {
		grow = false,
		tableLoading = false,
		class: className,
		children,
		footer
	}: {
		grow?: boolean;
		tableLoading?: boolean;
		class?: string;
		children: Snippet;
		footer?: Snippet;
	} = $props();

	let el: HTMLElement;

	onMount(() => {
		if (!grow) return;

		// Bottom padding of <main> on sm+ is p-8 = 2rem = 32px
		const BOTTOM_PADDING = 32;

		function update() {
			const top = el.getBoundingClientRect().top + window.scrollY;
			el.style.maxHeight = `calc(100svh - ${top}px - ${BOTTOM_PADDING}px)`;
		}

		update();

		const ro = new ResizeObserver(update);
		ro.observe(document.body);
		window.addEventListener('resize', update);
		window.addEventListener('scroll', update, { passive: true });

		return () => {
			ro.disconnect();
			window.removeEventListener('resize', update);
			window.removeEventListener('scroll', update);
		};
	});
</script>

<div
	bind:this={el}
	class={cn('rounded-xl border bg-card flex flex-col min-h-64 overflow-hidden', className)}
>
	<div class={cn('overflow-auto flex-1 min-h-0', tableLoading && 'opacity-50 pointer-events-none')}>
		{@render children()}
	</div>
	{#if footer}
		{@render footer()}
	{/if}
</div>
