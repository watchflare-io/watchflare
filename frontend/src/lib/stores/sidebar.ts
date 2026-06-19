import { writable } from 'svelte/store';

export const sidebarCollapsed = writable(false);
export const mobileMenuOpen = writable(false);
export const sidebarTransitioning = writable(false);

let transitionTimeout: ReturnType<typeof setTimeout> | null = null;

export function toggleSidebarWithTransition() {
	sidebarTransitioning.set(true);
	sidebarCollapsed.update((val) => !val);
	if (transitionTimeout) clearTimeout(transitionTimeout);
	transitionTimeout = setTimeout(() => {
		sidebarTransitioning.set(false);
	}, 300);
}

export function resetSidebar() {
	sidebarCollapsed.set(false);
	mobileMenuOpen.set(false);
	sidebarTransitioning.set(false);
	if (transitionTimeout) {
		clearTimeout(transitionTimeout);
		transitionTimeout = null;
	}
}
