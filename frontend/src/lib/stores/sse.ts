import { writable, derived } from 'svelte/store';
import { SSEManager, type ConnectionState } from '../sse/manager';
import { API_BASE_URL } from '../api';
import type { SSEEvent } from '../types';

interface SSEState {
	connectionState: ConnectionState;
	lastError: string | null;
	reconnectAttempts: number;
}

function createSSEStore() {
	const { subscribe, set, update } = writable<SSEState>({
		connectionState: 'disconnected',
		lastError: null,
		reconnectAttempts: 0
	});

	let manager: SSEManager | null = null;
	let subscribers: Set<(event: SSEEvent) => void> = new Set();
	let connectionCount = 0;
	let disconnectTimer: ReturnType<typeof setTimeout> | null = null;

	/**
	 * Broadcast message to all subscribers
	 */
	function broadcastMessage(event: SSEEvent): void {
		subscribers.forEach(callback => callback(event));
	}

	/**
	 * Initialize manager if not already created
	 */
	function initializeManager(): void {
		if (manager) return;

		// Create manager with configuration
		manager = new SSEManager(`${API_BASE_URL}/hosts/events`, {
			initialRetryDelay: 1000,  // Start with 1s
			maxRetryDelay: 30000,     // Cap at 30s
			maxRetries: Infinity,     // Retry indefinitely
		});

		// Register state change callback
		manager.onStateChange((state) => {
			update(s => ({
				...s,
				connectionState: state,
				reconnectAttempts: state === 'reconnecting' ? s.reconnectAttempts + 1 : 0
			}));
		});

		// Register error callback
		manager.onError((error) => {
			const errorMessage = error instanceof Error ? error.message : 'SSE connection error';
			update(s => ({
				...s,
				lastError: errorMessage
			}));
		});

		// Register message callback that broadcasts to all subscribers
		manager.onMessage(broadcastMessage);

		// Start connection
		manager.connect();
	}

	/**
	 * Actually disconnect the manager
	 */
	function performDisconnect(): void {
		if (manager) {
			manager.disconnect();
			manager = null;
		}

		set({
			connectionState: 'disconnected',
			lastError: null,
			reconnectAttempts: 0
		});
	}

	return {
		subscribe,

		/**
		 * Subscribe to SSE messages (multiple pages can subscribe)
		 */
		connect(onMessage: (event: SSEEvent) => void): () => void {
			// Cancel any pending disconnect
			if (disconnectTimer) {
				clearTimeout(disconnectTimer);
				disconnectTimer = null;
			}

			// Add subscriber
			subscribers.add(onMessage);
			connectionCount++;

			// Initialize manager on first connection
			if (connectionCount === 1) {
				initializeManager();
			}

			// Return unsubscribe function
			return () => {
				subscribers.delete(onMessage);
				connectionCount--;

				// Schedule disconnect with a delay to avoid reconnects during navigation
				if (connectionCount === 0) {
					// Wait 500ms before actually disconnecting
					// This allows navigation to complete without breaking the connection
					disconnectTimer = setTimeout(() => {
						// Double-check that no one reconnected during the delay
						if (connectionCount === 0) {
							performDisconnect();
						}
					}, 500);
				}
			};
		},

		/**
		 * Manual disconnect (disconnects all subscribers immediately)
		 */
		disconnect(): void {
			// Cancel any pending disconnect
			if (disconnectTimer) {
				clearTimeout(disconnectTimer);
				disconnectTimer = null;
			}

			subscribers.clear();
			connectionCount = 0;
			performDisconnect();
		},

		/**
		 * Get current connection state
		 */
		getState(): ConnectionState {
			return manager?.getState() || 'disconnected';
		},

		/**
		 * Clear error
		 */
		clearError(): void {
			update(s => ({ ...s, lastError: null }));
		}
	};
}

export const sseStore = createSSEStore();

// Derived stores for convenience
export const sseConnectionState = derived(sseStore, $store => $store.connectionState);
export const sseIsConnected = derived(sseStore, $store => $store.connectionState === 'connected');
export const sseIsReconnecting = derived(sseStore, $store => $store.connectionState === 'reconnecting');
export const sseLastError = derived(sseStore, $store => $store.lastError);
