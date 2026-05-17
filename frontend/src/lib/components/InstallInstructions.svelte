<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import OsIcon from '$lib/components/icons/OsIcon.svelte';
	import * as api from '$lib/api.js';
	import { logger } from '$lib/utils';
	import { AGENT_STATUS_POLL_INTERVAL } from '$lib/constants';
	import type { Host } from '$lib/types';

	const { host, token, agentKey = '', backendHost }: {
		host: Host;
		token: string;
		agentKey?: string;
		backendHost: string;
	} = $props();

	let selectedOS = $state('linux');
	let copied = $state(false);
	let copyTimeout: ReturnType<typeof setTimeout> | null = $state(null);
	let polledStatus: string | null = $state(null);
	let hostStatus = $derived(polledStatus ?? host.status);
	let pollInterval: ReturnType<typeof setInterval> | null = null;

	// Instructions for each OS
	let linuxCmd = $derived(`curl -sSL https://get.watchflare.io | sudo bash -s -- \\
  --token ${token} \\
  --host ${backendHost} \\
  --port 50051`);

	let macosCmd = $derived(`curl -sSL https://get.watchflare.io/brew | bash -s -- \\
  --token ${token} \\
  --host ${backendHost} \\
  --port 50051`);

	function handleCopy(text: string) {
		navigator.clipboard.writeText(text);
		copied = true;

		if (copyTimeout) clearTimeout(copyTimeout);
		copyTimeout = setTimeout(() => {
			copied = false;
		}, 2000);
	}

	async function pollHostStatus() {
		try {
			const response = await api.getHost(host.id);
			polledStatus = response.status;

			if (hostStatus === 'online') {
				if (pollInterval) clearInterval(pollInterval);
			}
		} catch (err) {
			logger.error('Failed to poll host status:', err);
		}
	}

	onMount(() => {
		if (hostStatus !== 'online') {
			pollInterval = setInterval(pollHostStatus, AGENT_STATUS_POLL_INTERVAL);
		}
	});

	onDestroy(() => {
		if (pollInterval) clearInterval(pollInterval);
		if (copyTimeout) clearTimeout(copyTimeout);
	});
</script>

{#if hostStatus === 'online'}
	<div class="flex items-center gap-3 rounded-lg border border-success bg-success/10 p-4">
		<svg class="h-4 w-4 shrink-0 text-success" fill="currentColor" viewBox="0 0 20 20">
			<path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
		</svg>
		<div>
			<p class="text-sm font-medium text-success">Agent connected</p>
			<p class="text-xs text-muted-foreground mt-0.5">Your host is now online and sending metrics</p>
		</div>
	</div>
{:else}
	<div class="rounded-lg border bg-card p-4 sm:p-6">
		<h3 class="text-sm font-semibold text-foreground mb-4">Installation</h3>

		<!-- OS Tabs -->
		<div class="flex gap-1 border-b mb-4">
			<button
       type="button"
				class="flex items-center gap-1.5 px-3 py-2 text-sm font-medium border-b-2 -mb-px transition-colors {selectedOS === 'linux'
					? 'border-primary text-foreground'
					: 'border-transparent text-muted-foreground hover:text-foreground'}"
				onclick={() => (selectedOS = 'linux')}
			>
				<OsIcon os="linux" class="h-4 w-4 shrink-0" />
				Linux
			</button>
			<button
       type="button"
				class="flex items-center gap-1.5 px-3 py-2 text-sm font-medium border-b-2 -mb-px transition-colors {selectedOS === 'macos'
					? 'border-primary text-foreground'
					: 'border-transparent text-muted-foreground hover:text-foreground'}"
				onclick={() => (selectedOS = 'macos')}
			>
				<OsIcon os="macos" class="h-4 w-4 shrink-0" />
				macOS
			</button>
			<button class="flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-muted-foreground opacity-40 cursor-not-allowed" disabled>
				<OsIcon os="windows" class="h-4 w-4 shrink-0" />
				Windows
			</button>
			<button class="flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-muted-foreground opacity-40 cursor-not-allowed" disabled>
				<OsIcon os="docker" class="h-4 w-4 shrink-0" />
				Docker
			</button>
		</div>

		<!-- Command -->
		<div class="relative">
			<pre class="bg-foreground text-background px-4 py-3 rounded-lg font-mono text-xs leading-relaxed overflow-x-auto pr-20">{selectedOS === 'linux' ? linuxCmd : macosCmd}</pre>
			<button
       type="button"
				class="absolute top-2 right-2 px-2.5 py-1.5 bg-muted-foreground/30 text-white rounded text-xs font-medium transition-colors hover:bg-muted-foreground/50"
				onclick={() => handleCopy(selectedOS === 'linux' ? linuxCmd : macosCmd)}
			>
				{copied ? 'Copied!' : 'Copy'}
			</button>
		</div>

		<!-- Waiting indicator -->
		{#if hostStatus === 'offline'}
			<div class="mt-4 flex items-center gap-2 text-sm text-muted-foreground">
				<div class="h-4 w-4 border-2 border-border border-t-warning rounded-full animate-spin shrink-0"></div>
				Agent registered — waiting for service to start
			</div>
		{:else}
			<div class="mt-4 flex items-center gap-2 text-sm text-muted-foreground">
				<div class="h-4 w-4 border-2 border-border border-t-primary rounded-full animate-spin shrink-0"></div>
				Waiting for agent to connect...
			</div>
		{/if}
	</div>
{/if}
