import { writable } from 'svelte/store';
import type { Theme } from '$lib/types';

const STORAGE_KEY = 'wf_auth_theme';

function applyTheme(theme: Theme): void {
	if (typeof document === 'undefined') return;
	if (theme === 'system') {
		const dark = window.matchMedia('(prefers-color-scheme: dark)').matches;
		document.documentElement.classList.toggle('dark', dark);
	} else {
		document.documentElement.classList.toggle('dark', theme === 'dark');
	}
}

function getStored(): Theme {
	if (typeof localStorage === 'undefined') return 'light';
	return (localStorage.getItem(STORAGE_KEY) as Theme) ?? 'light';
}

export const authTheme = writable<Theme>('light');

export function initAuthTheme(): void {
	const stored = getStored();
	authTheme.set(stored);
	applyTheme(stored);
}

export function cycleAuthTheme(): void {
	const next: Record<Theme, Theme> = { light: 'dark', dark: 'system', system: 'light' };
	authTheme.update((current) => {
		const theme = next[current];
		localStorage.setItem(STORAGE_KEY, theme);
		applyTheme(theme);
		return theme;
	});
}
