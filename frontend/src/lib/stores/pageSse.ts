import { writable } from 'svelte/store';
import type { ConnectionState } from '$lib/sse/manager';

/**
 * Overrides the global SSE connection state displayed in the sidebar.
 * Set by pages that maintain their own SSE connection (e.g. host detail).
 * Reset to null on unmount so the sidebar falls back to the global store.
 */
export const pageSseState = writable<ConnectionState | null>(null);
