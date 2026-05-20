import { writable } from 'svelte/store';

interface UIState {
	loading: boolean;
	rightSidebarOpen: boolean;
}

function createUIStore() {
	const { subscribe, set, update } = writable<UIState>({
		loading: false,
		rightSidebarOpen: false,
	});

	return {
		subscribe,

		setLoading(loading: boolean): void {
			update(state => ({ ...state, loading }));
		},

		toggleRightSidebar(): void {
			update(state => ({ ...state, rightSidebarOpen: !state.rightSidebarOpen }));
		},

		setRightSidebar(open: boolean): void {
			update(state => ({ ...state, rightSidebarOpen: open }));
		},

		reset(): void {
			set({ loading: false, rightSidebarOpen: false });
		}
	};
}

export const uiStore = createUIStore();
