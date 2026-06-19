<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		uiStore,
		sidebarCollapsed,
		sidebarTransitioning,
		userStore,
		alertsStore,
		sseStore
	} from '$lib/stores';
	import type { SSEEvent } from '$lib/types';
	import DesktopSidebar from '$lib/components/DesktopSidebar.svelte';
	import MobileSidebar from '$lib/components/MobileSidebar.svelte';
	import Header from '$lib/components/Header.svelte';
	import AlertsDrawer from '$lib/components/AlertsDrawer.svelte';

	const { children } = $props();

	let rightSidebarOpen = $derived($uiStore.rightSidebarOpen);
	let userReady = $state(false);
	let incidentPollInterval: ReturnType<typeof setInterval> | undefined;
	let sseUnsubscribe: (() => void) | undefined;

	onMount(async () => {
		try {
			await userStore.load();
			userReady = true;
		} catch {
			window.location.href = '/login';
		}
		alertsStore.loadIncidents();
		incidentPollInterval = setInterval(() => alertsStore.loadIncidents(), 30_000);
		sseUnsubscribe = sseStore.connect((event: SSEEvent) => {
			if (event.type === 'incidents_changed') {
				alertsStore.loadIncidents();
			}
		});
	});

	onDestroy(() => {
		clearInterval(incidentPollInterval);
		sseUnsubscribe?.();
	});
</script>

{#if userReady}
	<div class="min-h-dvh bg-background">
		<Header />

		<DesktopSidebar />
		<MobileSidebar />

		<AlertsDrawer open={rightSidebarOpen} onClose={() => uiStore.setRightSidebar(false)} />

		<main
			class="min-h-svh pt-26 p-5 sm:p-8 sm:pt-28 {$sidebarCollapsed
				? 'lg:ml-20'
				: 'lg:ml-64'} {$sidebarTransitioning ? 'transition-[margin] duration-300 ease-in-out' : ''}"
		>
			{@render children()}
		</main>
	</div>
{/if}
