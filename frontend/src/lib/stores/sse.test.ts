import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';

// Captured callbacks from SSEManager so tests can simulate events
let capturedStateChange: ((state: string) => void) | null = null;
let capturedError: ((err: Error) => void) | null = null;
let capturedMessage: ((event: unknown) => void) | null = null;

const mockConnect = vi.fn();
const mockDisconnect = vi.fn();
const mockGetState = vi.fn().mockReturnValue('connected');

vi.mock('$lib/sse/manager', () => ({
	SSEManager: vi.fn().mockImplementation(function () {
		return {
			connect: mockConnect,
			disconnect: mockDisconnect,
			getState: mockGetState,
			onStateChange: function (cb: (state: string) => void) {
				capturedStateChange = cb;
			},
			onError: function (cb: (err: Error) => void) {
				capturedError = cb;
			},
			onMessage: function (cb: (event: unknown) => void) {
				capturedMessage = cb;
			}
		};
	})
}));

import { sseStore, sseConnectionState, sseIsConnected, sseLastError } from './sse';

describe('sseStore', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.clearAllMocks();
		capturedStateChange = null;
		capturedError = null;
		capturedMessage = null;
		sseStore.disconnect(); // reset singleton state
	});

	afterEach(() => {
		sseStore.disconnect();
		vi.useRealTimers();
	});

	it('starts disconnected', () => {
		const state = get(sseStore);
		expect(state.connectionState).toBe('disconnected');
		expect(state.lastError).toBeNull();
		expect(state.reconnectAttempts).toBe(0);
	});

	it('connect registers a subscriber and initializes manager', () => {
		const cb = vi.fn();
		const unsubscribe = sseStore.connect(cb);
		expect(mockConnect).toHaveBeenCalledOnce();
		unsubscribe();
	});

	it('connect returns unsubscribe function', () => {
		const cb = vi.fn();
		const unsubscribe = sseStore.connect(cb);
		expect(typeof unsubscribe).toBe('function');
		unsubscribe();
	});

	it('multiple connects share one manager', () => {
		const cb1 = vi.fn();
		const cb2 = vi.fn();
		const unsub1 = sseStore.connect(cb1);
		const unsub2 = sseStore.connect(cb2);
		expect(mockConnect).toHaveBeenCalledOnce();
		unsub1();
		unsub2();
	});

	it('message is broadcast to all subscribers', () => {
		const cb1 = vi.fn();
		const cb2 = vi.fn();
		const unsub1 = sseStore.connect(cb1);
		const unsub2 = sseStore.connect(cb2);

		const event = { type: 'host_update', data: {} };
		capturedMessage?.(event);

		expect(cb1).toHaveBeenCalledWith(event);
		expect(cb2).toHaveBeenCalledWith(event);
		unsub1();
		unsub2();
	});

	it('unsubscribed callback no longer receives messages', () => {
		const cb = vi.fn();
		const unsubscribe = sseStore.connect(cb);
		unsubscribe();
		vi.advanceTimersByTime(600); // trigger auto-disconnect
		const cb2 = vi.fn();
		const unsub2 = sseStore.connect(cb2);
		capturedMessage?.({ type: 'host_update', data: {} });
		expect(cb).not.toHaveBeenCalled();
		unsub2();
	});

	it('last subscriber triggers delayed disconnect', () => {
		const cb = vi.fn();
		const unsubscribe = sseStore.connect(cb);
		unsubscribe();
		// Not yet disconnected (within 500ms delay)
		expect(mockDisconnect).not.toHaveBeenCalled();
		vi.advanceTimersByTime(500);
		expect(mockDisconnect).toHaveBeenCalledOnce();
	});

	it('re-connecting cancels the pending disconnect', () => {
		const cb1 = vi.fn();
		const unsub1 = sseStore.connect(cb1);
		unsub1(); // schedules disconnect in 500ms

		const cb2 = vi.fn();
		const unsub2 = sseStore.connect(cb2); // cancels pending disconnect
		vi.advanceTimersByTime(500);
		expect(mockDisconnect).not.toHaveBeenCalled();
		unsub2();
	});

	it('manual disconnect clears immediately', () => {
		const cb = vi.fn();
		sseStore.connect(cb);
		sseStore.disconnect();
		expect(mockDisconnect).toHaveBeenCalledOnce();
		expect(get(sseStore).connectionState).toBe('disconnected');
	});

	it('state change from manager updates store', () => {
		const cb = vi.fn();
		const unsub = sseStore.connect(cb);
		capturedStateChange?.('reconnecting');
		expect(get(sseConnectionState)).toBe('reconnecting');
		expect(get(sseStore).reconnectAttempts).toBe(1);
		unsub();
	});

	it('sseIsConnected is true when state is connected', () => {
		const cb = vi.fn();
		const unsub = sseStore.connect(cb);
		capturedStateChange?.('connected');
		expect(get(sseIsConnected)).toBe(true);
		unsub();
	});

	it('error from manager updates lastError', () => {
		const cb = vi.fn();
		const unsub = sseStore.connect(cb);
		capturedError?.(new Error('connection refused'));
		expect(get(sseLastError)).toBe('connection refused');
		unsub();
	});

	it('clearError resets lastError', () => {
		const cb = vi.fn();
		const unsub = sseStore.connect(cb);
		capturedError?.(new Error('oops'));
		sseStore.clearError();
		expect(get(sseLastError)).toBeNull();
		unsub();
	});
});
