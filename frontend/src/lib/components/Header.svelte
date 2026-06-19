<script lang="ts">
	import {
		mobileMenuOpen,
		sidebarCollapsed,
		toggleSidebarWithTransition,
		sidebarTransitioning,
	} from '$lib/stores/sidebar';
	import { uiStore, alertCount } from '$lib/stores';
	import { userStore, themeStore } from '$lib/stores/user';
	import { Search, Sun, Moon, Monitor } from 'lucide-svelte';
	import type { Theme } from '$lib/types';
	import CommandPalette from './CommandPalette.svelte';
	import Logo from './Logo.svelte';

	function toggleMenu() {
		mobileMenuOpen.update((val) => !val);
	}

	function toggleLeftSidebar() {
		toggleSidebarWithTransition();
	}

	function toggleAlerts() {
		uiStore.toggleRightSidebar();
	}

	function openSearch() {
		window.dispatchEvent(new CustomEvent('watchflare:open-search'));
	}

	function handleKeydown(e: KeyboardEvent) {
		if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
			e.preventDefault();
			openSearch();
		}
	}

	const isMac =
		typeof navigator !== 'undefined' && navigator.platform?.includes('Mac');

	const THEME_CYCLE: Theme[] = ['light', 'dark', 'system'];

	function cycleTheme() {
		const current = $themeStore;
		const next =
			THEME_CYCLE[(THEME_CYCLE.indexOf(current) + 1) % THEME_CYCLE.length];
		userStore.updateTheme(next);
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<header
	class="fixed left-0 right-0 top-0 z-30 h-fit pt-4 px-2 sm:px-4 bg-transparent {$sidebarCollapsed
		? 'lg:left-20'
		: 'lg:left-64'} {$sidebarTransitioning
		? 'transition-[left] duration-300 ease-in-out'
		: ''}"
>
	<div
		class="flex h-16 items-center gap-3 px-4 py-3 bg-surface rounded-lg border"
	>
		<!-- Left: Mobile burger + Desktop left sidebar toggle -->
		<div class="flex items-center gap-2 shrink-0">
			<!-- Burger button (mobile only) -->
			<button
				type="button"
				onclick={toggleMenu}
				class="flex h-9.5 w-9.5 items-center justify-center rounded-lg text-foreground transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary lg:hidden"
				aria-label="Toggle menu"
			>
				{#if $mobileMenuOpen}
					<svg
						class="h-5 w-5"
						fill="none"
						stroke="currentColor"
						viewBox="0 0 24 24"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M6 18L18 6M6 6l12 12"
						/>
					</svg>
				{:else}
					<svg
						class="h-5 w-5"
						fill="none"
						stroke="currentColor"
						viewBox="0 0 24 24"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M4 6h16M4 12h16M4 18h16"
						/>
					</svg>
				{/if}
			</button>

			<!-- Left sidebar toggle (desktop only) -->
			<button
				type="button"
				onclick={toggleLeftSidebar}
				class="hidden lg:flex h-9.5 w-9.5 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
				aria-label={$sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
			>
				<svg
					class="h-5 w-5"
					fill="none"
					stroke="currentColor"
					viewBox="0 0 24 24"
					stroke-width="2"
				>
					<rect x="3" y="3" width="18" height="18" rx="2" />
					<path d="M9 3v18" />
				</svg>
			</button>
		</div>

		<!-- Search button -->
		<button
			type="button"
			onclick={openSearch}
			class="flex items-center justify-center w-9.5 h-9.5 rounded-lg text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary sm:justify-start sm:w-auto lg:w-64 sm:gap-2.5 sm:rounded-lg sm:border sm:border-transparent sm:bg-foreground/4 sm:px-3 sm:text-sm sm:hover:border-border"
		>
			<Search class="h-4 w-4 shrink-0" />
			<span class="hidden sm:inline">Search...</span>
			<kbd
				class="ml-auto hidden sm:inline-flex items-center font-mono text-[11px] leading-relaxed px-1.5 py-0.5 rounded border bg-background text-muted-foreground"
			>
				{isMac ? '⌘K' : 'Ctrl K'}
			</kbd>
		</button>

		<!-- Logo (mobile/tablet only, centered absolutely) -->
		<a
			href="/"
			class="absolute left-1/2 -translate-x-1/2 lg:hidden"
			aria-label="Watchflare"
		>
			<Logo class="h-10 w-10" />
		</a>

		<!-- Right actions -->
		<div class="flex items-center gap-1 shrink-0 ms-auto h-full">
			<!-- Theme toggle -->
			<button
				type="button"
				onclick={cycleTheme}
				class="flex h-9.5 w-9.5 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
				aria-label="Toggle theme"
				title={$themeStore === 'light'
					? 'Light'
					: $themeStore === 'dark'
						? 'Dark'
						: 'System'}
			>
				{#if $themeStore === 'light'}
					<Sun class="h-4 w-4" />
				{:else if $themeStore === 'dark'}
					<Moon class="h-4 w-4" />
				{:else}
					<Monitor class="h-4 w-4" />
				{/if}
			</button>

			<!-- Alerts -->
			<button
				type="button"
				onclick={toggleAlerts}
				class="relative flex h-9.5 w-9.5 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
				aria-label="Toggle alerts"
			>
				<svg
					class="h-5 w-5"
					fill="none"
					stroke="currentColor"
					viewBox="0 0 24 24"
					stroke-width="2"
				>
					<path
						stroke-linecap="round"
						stroke-linejoin="round"
						d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
					/>
				</svg>
				{#if $alertCount > 0}
					<span
						class="absolute top-1 right-1 h-2.5 w-2.5 rounded-full bg-destructive"
					></span>
				{/if}
			</button>
		</div>
	</div>
</header>

<CommandPalette />
