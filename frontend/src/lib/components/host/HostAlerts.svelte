<script lang="ts">
	import { formatDateTime } from '$lib/utils';
	import { userStore } from '$lib/stores/user';
	import { TriangleAlert, RefreshCw, X } from 'lucide-svelte';
	import type { Host } from '$lib/types';

	const timeFormat = $derived(($userStore.user?.time_format ?? '24h') as '12h' | '24h');

	const {
		host,
		showIPMismatchWarning,
		clockDesync = false,
		onUpdateIP,
		onIgnoreIP,
		onDismissReactivation
	}: {
		host: Host;
		showIPMismatchWarning: boolean;
		clockDesync?: boolean;
		onUpdateIP: () => void;
		onIgnoreIP: () => void;
		onDismissReactivation: () => void;
	} = $props();
</script>

{#if clockDesync}
	<div class="mb-4 rounded-lg border border-danger bg-danger/10 p-3">
		<div class="flex items-start gap-3">
			<TriangleAlert class="h-5 w-5 text-danger mt-0.5 shrink-0" />
			<div class="flex-1">
				<p class="text-sm font-medium text-foreground">Clock Synchronization Error</p>
				<p class="text-sm text-muted-foreground mt-1">
					The agent's system clock is out of sync with the Hub (&gt;5 min difference). Heartbeats
					are being rejected.
				</p>
			</div>
		</div>
	</div>
{/if}

{#if showIPMismatchWarning}
	<div class="mb-4 rounded-lg border border-warning bg-warning/10 p-3">
		<div class="flex items-start gap-3">
			<TriangleAlert class="h-5 w-5 text-warning mt-0.5 shrink-0" />
			<div class="flex-1">
				<p class="text-sm font-medium text-foreground">IP Address Mismatch</p>
				<p class="text-sm text-muted-foreground mt-1">
					Configured IP: {host.configured_ip} • Actual IP: {host.ip_address_v4}
				</p>
				<div class="mt-3 flex gap-2">
					<button
						type="button"
						onclick={onUpdateIP}
						class="rounded-lg bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary/90"
					>
						Update to {host.ip_address_v4}
					</button>
					<button
						type="button"
						onclick={onIgnoreIP}
						class="rounded-lg border bg-background px-3 py-1.5 text-xs font-medium text-foreground transition-colors hover:bg-muted"
					>
						Ignore
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}

{#if host.reactivated_at}
	<div class="mb-4 rounded-lg border border-primary bg-primary/10 p-3">
		<div class="flex items-start justify-between gap-3">
			<div class="flex items-start gap-3">
				<RefreshCw class="h-5 w-5 text-primary mt-0.5 shrink-0" />
				<div>
					<p class="text-sm font-medium text-foreground">Agent Reactivated</p>
					<p class="text-sm text-muted-foreground mt-1">
						Same physical host detected via UUID at {formatDateTime(
							host.reactivated_at,
							timeFormat
						)}
					</p>
				</div>
			</div>
			<button
				type="button"
				onclick={onDismissReactivation}
				class="text-primary hover:text-primary/80"
				aria-label="Dismiss reactivation notice"
			>
				<X class="h-5 w-5" />
			</button>
		</div>
	</div>
{/if}
