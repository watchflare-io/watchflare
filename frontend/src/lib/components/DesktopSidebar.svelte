<script lang="ts">
	import { page } from '$app/stores';
	import { sidebarCollapsed, sidebarTransitioning } from '$lib/stores/sidebar';
	import { Settings, ChevronDown } from 'lucide-svelte';
	import SSEStatusBadge from './SSEStatusBadge.svelte';
	import UserMenuButton from './UserMenuButton.svelte';
	import { navItems, settingsItems } from '$lib/navigation';
	import { alertCount } from '$lib/stores/alerts';
	import Logo from './Logo.svelte';

	const transitioning = $derived($sidebarTransitioning);
	const collapsed = $derived($sidebarCollapsed);
	const transitionClass = $derived(transitioning ? 'transition-all duration-300 ease-in-out' : '');
	const textClass = $derived(
		collapsed
			? `max-w-0 min-w-0 ml-0 opacity-0 ${transitionClass}`
			: `max-w-48 ml-3 opacity-100 ${transitionClass}`
	);

	let settingsOpen = $state($page.url.pathname.startsWith('/settings'));

	let settingsIconEl = $state<HTMLElement | null>(null);
	let flyoutOpen = $state(false);
	let flyoutY = $state(0);
	let flyoutCloseTimer: ReturnType<typeof setTimeout> | null = null;

	function openFlyout() {
		if (flyoutCloseTimer) clearTimeout(flyoutCloseTimer);
		if (settingsIconEl) {
			flyoutY = settingsIconEl.getBoundingClientRect().top;
		}
		flyoutOpen = true;
	}

	function scheduleFlyoutClose() {
		flyoutCloseTimer = setTimeout(() => {
			flyoutOpen = false;
		}, 150);
	}

	function cancelFlyoutClose() {
		if (flyoutCloseTimer) clearTimeout(flyoutCloseTimer);
	}

	$effect(() => {
		if ($page.url.pathname.startsWith('/settings')) {
			settingsOpen = true;
		}
	});

	function isActive(href: string): boolean {
		if (href === '/') {
			return $page.url.pathname === '/';
		}
		return $page.url.pathname.startsWith(href);
	}

	function isSubActive(href: string): boolean {
		return $page.url.pathname === href;
	}
</script>

<aside
	class="fixed left-0 top-0 z-40 py-4 pl-4 hidden lg:block h-svh bg-transparent {collapsed
		? 'w-20'
		: 'w-64'} {transitioning ? 'transition-[width] duration-300 ease-in-out' : ''}"
>
	<div class="flex h-full flex-col overflow-hidden bg-surface rounded-2xl border">
		<!-- Logo -->
		<div class="flex h-16 items-center border-b px-2.75">
			<Logo class="h-10 w-10 shrink-0" />
			<span
				class="text-lg font-semibold text-foreground whitespace-nowrap overflow-hidden {textClass}"
				>Watchflare</span
			>
		</div>

		<!-- Navigation -->
		<nav class="flex-1 flex flex-col gap-1 p-2">
			{#each navItems as item}
				{@const Icon = item.icon}
				{@const badge = item.href === '/incidents' ? $alertCount : 0}
				<a
					href={item.href}
					aria-current={isActive(item.href) ? 'page' : undefined}
					class="flex items-center rounded-lg py-3.25 px-3.25 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary {isActive(
						item.href
					)
						? 'bg-primary text-primary-foreground'
						: 'text-surface-foreground hover:bg-surface-accent'}"
					title={item.label}
				>
					<span class="relative shrink-0">
						<Icon class="h-5 w-5" />
						{#if badge > 0 && collapsed}
							<span class="absolute -top-0.5 -right-0.5 h-2 w-2 rounded-full bg-destructive"></span>
						{/if}
					</span>
					<span class="whitespace-nowrap overflow-hidden flex-1 {textClass}">{item.label}</span>
					{#if badge > 0 && !collapsed}
						<span
							class="shrink-0 ml-1 min-w-5 h-5 rounded-full px-1.5 text-xs font-medium flex items-center justify-center {isActive(
								item.href
							)
								? 'bg-primary-foreground/20 text-primary-foreground'
								: 'bg-destructive text-destructive-foreground'}"
						>
							{badge}
						</span>
					{/if}
				</a>
			{/each}

			<!-- Settings group -->
			{#if collapsed}
				<!-- Collapsed: icon only, flyout on hover -->
				<div
					bind:this={settingsIconEl}
					onmouseenter={openFlyout}
					onmouseleave={scheduleFlyoutClose}
					role="presentation"
				>
					<a
						href="/settings"
						aria-current={isActive('/settings') ? 'page' : undefined}
						class="flex items-center rounded-lg py-3.25 px-3.25 text-sm font-medium transition-colors {isActive(
							'/settings'
						)
							? 'bg-primary text-primary-foreground'
							: 'text-surface-foreground hover:bg-surface-accent'}"
						title="Settings"
					>
						<Settings class="h-5 w-5 shrink-0" />
					</a>
				</div>
			{:else}
				<!-- Expanded: group with sub-items -->
				<div>
					<button
						type="button"
						aria-expanded={settingsOpen}
						onclick={() => {
							settingsOpen = !settingsOpen;
						}}
						class="w-full flex items-center rounded-lg py-3.25 px-3.25 text-sm font-medium transition-colors text-surface-foreground hover:bg-surface-accent"
					>
						<Settings class="h-5 w-5 shrink-0" />
						<span class="whitespace-nowrap overflow-hidden flex-1 text-left {textClass}"
							>Settings</span
						>
						<ChevronDown
							class="h-4 w-4 shrink-0 mr-1 transition-transform duration-200 {settingsOpen
								? 'rotate-180'
								: ''}"
						/>
					</button>
					{#if settingsOpen}
						<div class="ml-6 mt-1 mb-1 flex flex-col gap-0.5 border-l border-border pl-2">
							{#each settingsItems as sub}
								<a
									href={sub.href}
									aria-current={isSubActive(sub.href) ? 'page' : undefined}
									class="rounded-lg py-3.25 px-3 text-sm font-medium transition-colors {isSubActive(
										sub.href
									)
										? 'bg-primary text-primary-foreground'
										: 'text-surface-foreground hover:bg-surface-accent'}"
								>
									{sub.label}
								</a>
							{/each}
						</div>
					{/if}
				</div>
			{/if}
		</nav>

		<!-- SSE Connection Status + User Menu -->
		<div class="border-t">
			<!-- SSE Status Badge -->
			<div class="px-2 pt-3 pb-1">
				<SSEStatusBadge {textClass} />
			</div>

			<!-- User Menu -->
			<div class="px-2 pb-3">
				<UserMenuButton {collapsed} {textClass} />
			</div>
		</div>
	</div>

	<!-- Settings flyout (outside overflow-hidden, fixed positioning) -->
	{#if collapsed && flyoutOpen}
		<div
			style="top: {flyoutY}px"
			class="fixed left-20 ml-2 z-50 w-44 rounded-lg border bg-surface shadow-lg overflow-hidden"
			onmouseenter={cancelFlyoutClose}
			onmouseleave={scheduleFlyoutClose}
			role="menu"
			tabindex="-1"
		>
			<div
				class="px-3 py-2 text-xs font-semibold text-muted-foreground border-b"
				role="presentation"
			>
				Settings
			</div>
			{#each settingsItems as sub}
				<a
					href={sub.href}
					role="menuitem"
					onclick={() => (flyoutOpen = false)}
					class="block px-3 py-2 text-sm font-medium transition-colors {$page.url.pathname ===
					sub.href
						? 'bg-primary text-primary-foreground'
						: 'text-surface-foreground hover:bg-surface-accent'}"
				>
					{sub.label}
				</a>
			{/each}
		</div>
	{/if}
</aside>
