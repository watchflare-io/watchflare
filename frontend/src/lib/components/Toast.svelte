<script lang="ts">
	import { toasts } from '$lib/stores/toasts';
	import { fly } from 'svelte/transition';
	import { CheckCircle, AlertTriangle, XCircle, Info, X } from 'lucide-svelte';
	import type { ToastType } from '$lib/types';

	function getIcon(type: ToastType) {
		switch (type) {
			case 'success': return CheckCircle;
			case 'warning': return AlertTriangle;
			case 'error':   return XCircle;
			default:        return Info;
		}
	}

	function getColorClasses(type: ToastType): string {
		switch (type) {
			case 'success': return 'bg-success/10 border-success/30 text-success';
			case 'warning': return 'bg-warning/10 border-warning/30 text-warning';
			case 'error':   return 'bg-destructive/10 border-destructive/30 text-destructive';
			default:        return 'bg-primary/10 border-primary/30 text-primary';
		}
	}
</script>

<div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2 max-w-sm w-full pointer-events-none px-4 sm:px-0">
	{#each $toasts as toast (toast.id)}
		{@const Icon = getIcon(toast.type)}
		<div
			transition:fly={{ y: 16, duration: 250 }}
			class="pointer-events-auto flex items-start gap-3 px-4 py-3 rounded-lg border {getColorClasses(toast.type)}"
		>
			<Icon class="h-4 w-4 shrink-0 mt-0.5" />
			<p class="flex-1 text-sm leading-snug">{toast.message}</p>
			<button
       type="button"
				onclick={() => toasts.remove(toast.id)}
				class="shrink-0 opacity-50 hover:opacity-100 transition-opacity"
				aria-label="Dismiss"
			>
				<X class="h-4 w-4" />
			</button>
		</div>
	{/each}
</div>
