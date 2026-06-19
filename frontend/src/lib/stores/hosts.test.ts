import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';

vi.mock('$lib/api', () => ({
	listHosts: vi.fn()
}));

import { hostsStore, hosts, onlineHosts, offlineHosts, hostsLoading } from './hosts';
import { listHosts } from '$lib/api';

const mockListHosts = vi.mocked(listHosts);

function makeHost(id: string, status: 'online' | 'offline' | 'pending' = 'online') {
	return {
		id,
		display_name: `Host ${id}`,
		hostname: `host-${id}`,
		status,
		ip_address_v4: '1.2.3.4',
		ip_address_v6: '',
		configured_ip: null,
		last_seen: new Date().toISOString(),
		agent_version: '0.1.0',
		latest_agent_version: null,
		agent_id: `agent-${id}`,
		os: 'linux',
		platform: 'ubuntu',
		platform_version: '22.04',
		kernel_version: '5.15',
		kernel_arch: 'x86_64',
		cpu_model: 'Intel',
		cpu_physical_count: 4,
		cpu_logical_count: 8,
		cpu_mhz: 2400,
		total_ram_bytes: 8000000000,
		container_runtime: '',
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString(),
		pending_command: null,
		ignore_ip_mismatch: false,
		paused: false,
		reactivated: false,
		clock_desync: false
	};
}

describe('hostsStore', () => {
	beforeEach(() => {
		hostsStore.clear();
		vi.clearAllMocks();
	});

	it('starts with empty state', () => {
		const state = get(hostsStore);
		expect(state.hosts).toEqual([]);
		expect(state.loading).toBe(false);
		expect(state.error).toBeNull();
	});

	it('load sets loading then updates hosts', async () => {
		const host = makeHost('h1');
		mockListHosts.mockResolvedValueOnce({ hosts: [host] });
		const promise = hostsStore.load();
		expect(get(hostsLoading)).toBe(true);
		await promise;
		expect(get(hostsLoading)).toBe(false);
		expect(get(hosts)).toHaveLength(1);
		expect(get(hosts)[0].host.id).toBe('h1');
	});

	it('load sets error on failure', async () => {
		mockListHosts.mockRejectedValueOnce(new Error('Network error'));
		await expect(hostsStore.load()).rejects.toThrow('Network error');
		const state = get(hostsStore);
		expect(state.loading).toBe(false);
		expect(state.error).toBe('Network error');
	});

	it('updateHost updates a specific host', () => {
		hostsStore.addHost(makeHost('h1'));
		hostsStore.updateHost('h1', { display_name: 'Updated' });
		expect(get(hosts)[0].host.display_name).toBe('Updated');
	});

	it('updateHost ignores unknown hostId', () => {
		hostsStore.addHost(makeHost('h1'));
		hostsStore.updateHost('unknown', { display_name: 'Updated' });
		expect(get(hosts)[0].host.display_name).toBe('Host h1');
	});

	it('updateStatus updates status and last_seen', () => {
		hostsStore.addHost(makeHost('h1', 'online'));
		const newLastSeen = new Date().toISOString();
		hostsStore.updateStatus('h1', 'offline', newLastSeen);
		const host = get(hosts)[0].host;
		expect(host.status).toBe('offline');
		expect(host.last_seen).toBe(newLastSeen);
	});

	it('addHost appends a host', () => {
		hostsStore.addHost(makeHost('h1'));
		hostsStore.addHost(makeHost('h2'));
		expect(get(hosts)).toHaveLength(2);
	});

	it('removeHost removes a host by id', () => {
		hostsStore.addHost(makeHost('h1'));
		hostsStore.addHost(makeHost('h2'));
		hostsStore.removeHost('h1');
		expect(get(hosts)).toHaveLength(1);
		expect(get(hosts)[0].host.id).toBe('h2');
	});

	it('clear resets state', () => {
		hostsStore.addHost(makeHost('h1'));
		hostsStore.clear();
		expect(get(hosts)).toHaveLength(0);
		expect(get(hostsStore).error).toBeNull();
	});
});

describe('derived host stores', () => {
	beforeEach(() => {
		hostsStore.clear();
	});

	it('onlineHosts filters online hosts', () => {
		hostsStore.addHost(makeHost('h1', 'online'));
		hostsStore.addHost(makeHost('h2', 'offline'));
		hostsStore.addHost(makeHost('h3', 'online'));
		expect(get(onlineHosts)).toHaveLength(2);
	});

	it('offlineHosts filters offline hosts', () => {
		hostsStore.addHost(makeHost('h1', 'online'));
		hostsStore.addHost(makeHost('h2', 'offline'));
		expect(get(offlineHosts)).toHaveLength(1);
		expect(get(offlineHosts)[0].host.id).toBe('h2');
	});
});
