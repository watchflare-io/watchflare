import { writable, derived } from 'svelte/store';
import type { User, TimeRange, Theme } from '$lib/types';
import { getCurrentUser, updatePreferences, type UpdatePreferencesPayload } from '$lib/api';
import { logger } from '$lib/utils';

interface UserState {
	user: User | null;
	loading: boolean;
	error: string | null;
}

let mediaQuery: MediaQueryList | null = null;
let mediaListener: ((e: MediaQueryListEvent) => void) | null = null;

function applyTheme(theme: Theme): void {
	if (typeof document === 'undefined') return;

	// Clean up previous system listener
	if (mediaListener && mediaQuery) {
		mediaQuery.removeEventListener('change', mediaListener);
		mediaListener = null;
	}

	if (theme === 'system') {
		mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
		const apply = (dark: boolean) => {
			document.documentElement.classList.toggle('dark', dark);
		};
		apply(mediaQuery.matches);
		mediaListener = (e) => apply(e.matches);
		mediaQuery.addEventListener('change', mediaListener);
	} else {
		document.documentElement.classList.toggle('dark', theme === 'dark');
	}
}

// Standalone reactive store for theme — always in sync with user preferences
export const themeStore = writable<Theme>('system');

function createUserStore() {
	const { subscribe, set, update } = writable<UserState>({
		user: null,
		loading: false,
		error: null
	});

	return {
		subscribe,

		// Load current user from API
		async load(): Promise<void> {
			update((state) => ({ ...state, loading: true, error: null }));

			try {
				const userData = await getCurrentUser();
				if (!userData || !userData.user) {
					throw new Error('No user data received');
				}

				const user = userData.user;
				const theme = user.theme || 'system';
				applyTheme(theme);
				themeStore.set(theme);

				update((state) => ({
					...state,
					user,
					loading: false
				}));
			} catch (err) {
				const error = err instanceof Error ? err.message : 'Failed to load user';
				update((state) => ({ ...state, loading: false, error }));
				throw err;
			}
		},

		// Update user preferences (partial — only sends provided fields)
		async updatePreferences(payload: UpdatePreferencesPayload): Promise<void> {
			// Optimistic update for theme
			if (payload.theme) {
				themeStore.set(payload.theme as Theme);
				applyTheme(payload.theme as Theme);
			}

			update((state) => {
				if (!state.user) return state;
				return {
					...state,
					user: {
						...state.user,
						...(payload.default_time_range && {
							default_time_range: payload.default_time_range as TimeRange
						}),
						...(payload.theme && { theme: payload.theme as Theme }),
						...(payload.time_format && {
							time_format: payload.time_format as User['time_format']
						}),
						...(payload.temperature_unit && {
							temperature_unit: payload.temperature_unit as User['temperature_unit']
						}),
						...(payload.network_unit && {
							network_unit: payload.network_unit as User['network_unit']
						}),
						...(payload.disk_unit && {
							disk_unit: payload.disk_unit as User['disk_unit']
						}),
						...(payload.gauge_warning_threshold !== undefined && {
							gauge_warning_threshold: payload.gauge_warning_threshold
						}),
						...(payload.gauge_critical_threshold !== undefined && {
							gauge_critical_threshold: payload.gauge_critical_threshold
						})
					}
				};
			});

			try {
				const res = await updatePreferences(payload);
				// Sync store with server response to ensure consistency
				update((state) => ({ ...state, user: res.user }));
				if (res.user.theme) {
					themeStore.set(res.user.theme);
					applyTheme(res.user.theme);
				}
			} catch (err) {
				logger.error('Failed to update preferences:', err);
				throw err;
			}
		},

		// Update theme only
		async updateTheme(theme: Theme): Promise<void> {
			applyTheme(theme);
			themeStore.set(theme);
			update((state) => {
				if (state.user) return { ...state, user: { ...state.user, theme } };
				return state;
			});

			try {
				const res = await updatePreferences({ theme });
				update((state) => ({ ...state, user: res.user }));
			} catch (err) {
				logger.error('Failed to update theme:', err);
			}
		},

		// Update user in store directly (e.g. from API response)
		setUser(user: User): void {
			update((state) => ({ ...state, user }));
		},

		// Clear user data (logout)
		clear(): void {
			set({ user: null, loading: false, error: null });
			themeStore.set('system');
		}
	};
}

export const userStore = createUserStore();

// Derived stores for convenience
export const currentUser = derived(userStore, ($store) => $store.user);
export const userLoading = derived(userStore, ($store) => $store.loading);
