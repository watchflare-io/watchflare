import { writable } from 'svelte/store';
import type { Toast, ToastType, ToastStore } from '$lib/types';
import { TOAST_DEFAULT_DURATION } from '$lib/constants';

// Toast store for managing toast notifications
function createToastStore(): ToastStore {
	const { subscribe, update } = writable<Toast[]>([]);

	let nextId = 0;

	return {
		subscribe,
		add: (
			message: string,
			type: ToastType = 'info',
			duration: number = TOAST_DEFAULT_DURATION
		): number => {
			const id = nextId++;
			const toast: Toast = { id, message, type };

			update((toasts) => [...toasts, toast]);

			// Auto-remove after duration
			if (duration > 0) {
				setTimeout(() => {
					update((toasts) => toasts.filter((t) => t.id !== id));
				}, duration);
			}

			return id;
		},
		remove: (id: number): void => {
			update((toasts) => toasts.filter((t) => t.id !== id));
		},
		clear: (): void => {
			update(() => []);
		}
	};
}

export const toasts = createToastStore();
