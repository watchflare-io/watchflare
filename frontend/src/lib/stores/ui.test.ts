import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { uiStore } from './ui';

describe('uiStore', () => {
	beforeEach(() => {
		uiStore.reset();
	});

	it('has correct initial state', () => {
		const state = get(uiStore);
		expect(state.loading).toBe(false);
		expect(state.rightSidebarOpen).toBe(false);
	});

	it('setLoading(true) sets loading to true', () => {
		uiStore.setLoading(true);
		expect(get(uiStore).loading).toBe(true);
	});

	it('setLoading(false) sets loading back to false', () => {
		uiStore.setLoading(true);
		uiStore.setLoading(false);
		expect(get(uiStore).loading).toBe(false);
	});

	it('toggleRightSidebar opens the sidebar', () => {
		uiStore.toggleRightSidebar();
		expect(get(uiStore).rightSidebarOpen).toBe(true);
	});

	it('toggleRightSidebar closes the sidebar', () => {
		uiStore.toggleRightSidebar();
		uiStore.toggleRightSidebar();
		expect(get(uiStore).rightSidebarOpen).toBe(false);
	});

	it('setRightSidebar(true) opens sidebar', () => {
		uiStore.setRightSidebar(true);
		expect(get(uiStore).rightSidebarOpen).toBe(true);
	});

	it('setRightSidebar(false) closes sidebar', () => {
		uiStore.setRightSidebar(true);
		uiStore.setRightSidebar(false);
		expect(get(uiStore).rightSidebarOpen).toBe(false);
	});

	it('reset clears all state', () => {
		uiStore.setLoading(true);
		uiStore.setRightSidebar(true);
		uiStore.reset();
		const state = get(uiStore);
		expect(state.loading).toBe(false);
		expect(state.rightSidebarOpen).toBe(false);
	});
});
