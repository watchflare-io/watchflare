<script lang="ts">
	import { goto } from '$app/navigation';
	import * as api from '$lib/api.js';
	import { createHostSchema, validateForm } from '$lib/validation';
	import Modal from '$lib/components/Modal.svelte';

	const {
		open,
		onClose
	}: {
		open: boolean;
		onClose: () => void;
	} = $props();

	let name = $state('');
	let configuredIP = $state('');
	let allowAnyIP = $state(false);
	let error = $state('');
	let fieldErrors: Record<string, string> = $state({});
	let loading = $state(false);

	function reset() {
		name = '';
		configuredIP = '';
		allowAnyIP = false;
		error = '';
		fieldErrors = {};
	}

	function handleClose() {
		reset();
		onClose();
	}

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		fieldErrors = {};

		const result = validateForm(createHostSchema, {
			name,
			configuredIP,
			allowAnyIP
		});
		if (!result.success) {
			fieldErrors = result.errors;
			return;
		}

		loading = true;
		try {
			const response = await api.createHost(name, configuredIP || undefined, allowAnyIP);
			onClose();
			reset();
			goto(`/hosts/${response.host.id}`, {
				state: { newHostToken: response.token }
			});
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create host';
		} finally {
			loading = false;
		}
	}
</script>

<Modal {open} onClose={handleClose}>
	<h3 class="text-lg font-semibold text-foreground mb-4">Add New Host</h3>

	<form onsubmit={handleSubmit}>
		<div class="mb-4">
			<label for="host-name" class="block text-sm font-medium text-foreground mb-2">
				Name <span class="text-destructive">*</span>
			</label>
			<input
				id="host-name"
				type="text"
				bind:value={name}
				placeholder="e.g., web-server-01"
				aria-invalid={!!fieldErrors.name}
				aria-describedby={fieldErrors.name ? 'host-name-error' : undefined}
				class="w-full rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary {fieldErrors.name
					? 'border-destructive'
					: ''}"
			/>
			{#if fieldErrors.name}<p id="host-name-error" class="mt-1 text-xs text-destructive">
					{fieldErrors.name}
				</p>{/if}
		</div>

		<div class="mb-4">
			<label for="host-ip" class="block text-sm font-medium text-foreground mb-2">
				IP Address {#if !allowAnyIP}<span class="text-destructive">*</span>{/if}
			</label>
			<input
				id="host-ip"
				type="text"
				bind:value={configuredIP}
				disabled={allowAnyIP}
				placeholder="e.g., 192.168.1.100"
				aria-invalid={!!fieldErrors.configuredIP}
				aria-describedby={fieldErrors.configuredIP ? 'host-ip-error' : undefined}
				class="w-full rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50 {fieldErrors.configuredIP
					? 'border-destructive'
					: ''}"
			/>
			{#if fieldErrors.configuredIP}<p id="host-ip-error" class="mt-1 text-xs text-destructive">
					{fieldErrors.configuredIP}
				</p>{/if}
		</div>

		<div class="mb-5">
			<label class="flex items-center gap-2 cursor-pointer">
				<input type="checkbox" bind:checked={allowAnyIP} class="h-4 w-4 rounded border-gray-300" />
				<span class="text-sm text-foreground">Allow registration from any IP</span>
			</label>
		</div>

		{#if error}
			<div class="mb-4 rounded-lg border border-destructive bg-destructive/10 p-3">
				<p class="text-sm text-destructive">{error}</p>
			</div>
		{/if}

		<div class="flex gap-3 justify-end">
			<button
				type="button"
				onclick={handleClose}
				class="rounded-lg border bg-background px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-muted"
			>
				Cancel
			</button>
			<button
				type="submit"
				disabled={loading}
				class="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
			>
				{loading ? 'Creating...' : 'Create Host'}
			</button>
		</div>
	</form>
</Modal>
