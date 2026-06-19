<script lang="ts">
	import type { Snippet } from 'svelte';
	import Modal from './Modal.svelte';
	import { Button } from '$lib/components/ui/button';

	const {
		open,
		title,
		onConfirm,
		onClose,
		confirmLabel = 'Confirm',
		confirmVariant = 'primary',
		children
	}: {
		open: boolean;
		title: string;
		onConfirm: () => void;
		onClose: () => void;
		confirmLabel?: string;
		confirmVariant?: 'destructive' | 'primary';
		children: Snippet;
	} = $props();
</script>

<Modal {open} {onClose}>
	<h3 class="text-lg font-semibold text-foreground mb-3">{title}</h3>
	{@render children()}
	<div class="flex gap-3 justify-end mt-6">
		<Button variant="outline" onclick={onClose}>Cancel</Button>
		<Button
			variant={confirmVariant === 'destructive' ? 'destructive' : 'default'}
			onclick={onConfirm}
		>
			{confirmLabel}
		</Button>
	</div>
</Modal>
