import { describe, it, expect } from 'vitest';
import {
	formatBytes,
	formatPercent,
	formatUptime,
	getMetricClass,
	getStatusClass,
	formatRelativeTime,
	formatOfflineDuration,
	formatDateTime,
	getIntervalForTimeRange,
	getTimeRangeTimestamps,
	parsePortBadges,
	capitalizeFirst,
	isSystemContainer,
	cpuBarClass,
	memBarClass,
	healthBadgeClass,
	memoryPercent,
	containerIsLive,
	filterContainers,
	toggleCategory
} from './utils';
import type { GlobalContainer, NotificationCategory } from './types';

describe('capitalizeFirst', () => {
	it('capitalizes a lowercase backend message', () => {
		expect(capitalizeFirst('invalid credentials')).toBe('Invalid credentials');
	});

	it('leaves an already-capitalized string unchanged', () => {
		expect(capitalizeFirst('Save failed.')).toBe('Save failed.');
	});

	it('returns an empty string unchanged', () => {
		expect(capitalizeFirst('')).toBe('');
	});

	it('only touches the first character', () => {
		expect(capitalizeFirst('name is required')).toBe('Name is required');
	});
});

describe('formatBytes', () => {
	it('formats 0 bytes', () => {
		expect(formatBytes(0)).toBe('0 B');
	});

	it('formats bytes', () => {
		expect(formatBytes(500)).toBe('500 B');
	});

	it('formats kilobytes', () => {
		expect(formatBytes(1024)).toBe('1 KB');
	});

	it('formats megabytes', () => {
		expect(formatBytes(1048576)).toBe('1 MB');
	});

	it('formats gigabytes', () => {
		expect(formatBytes(1073741824)).toBe('1 GB');
	});

	it('formats terabytes', () => {
		expect(formatBytes(1099511627776)).toBe('1 TB');
	});

	it('formats with decimals', () => {
		expect(formatBytes(1536)).toBe('1.5 KB');
	});
});

describe('formatPercent', () => {
	it('formats integer percentage', () => {
		expect(formatPercent(50)).toBe('50%');
	});

	it('formats decimal percentage with one decimal place', () => {
		expect(formatPercent(33.33)).toBe('33.3%');
	});

	it('formats zero', () => {
		expect(formatPercent(0)).toBe('0%');
	});

	it('formats 100%', () => {
		expect(formatPercent(100)).toBe('100%');
	});
});

describe('formatUptime', () => {
	it('formats minutes only', () => {
		expect(formatUptime(300)).toBe('5m');
	});

	it('formats hours and minutes', () => {
		expect(formatUptime(3660)).toBe('1h 1m');
	});

	it('formats days and hours', () => {
		expect(formatUptime(90000)).toBe('1d 1h');
	});

	it('formats zero seconds', () => {
		expect(formatUptime(0)).toBe('0m');
	});
});

describe('getMetricClass', () => {
	it('returns foreground below 70%', () => {
		expect(getMetricClass(50)).toBe('text-foreground');
	});

	it('returns warning at 70%', () => {
		expect(getMetricClass(70)).toContain('text-warning');
	});

	it('returns warning between 70-89%', () => {
		expect(getMetricClass(85)).toContain('text-warning');
	});

	it('returns danger at 90%', () => {
		expect(getMetricClass(90)).toContain('text-danger');
	});

	it('returns danger above 90%', () => {
		expect(getMetricClass(99)).toContain('text-danger');
	});
});

describe('getStatusClass', () => {
	it('returns success classes for online', () => {
		expect(getStatusClass('online')).toContain('bg-success');
	});

	it('returns danger classes for offline', () => {
		expect(getStatusClass('offline')).toContain('bg-danger');
	});

	it('returns warning classes for pending', () => {
		expect(getStatusClass('pending')).toContain('bg-warning');
	});

	it('returns muted classes for unknown status', () => {
		expect(getStatusClass('unknown')).toContain('bg-muted');
	});
});

describe('formatRelativeTime', () => {
	it('returns "Never" for null/undefined', () => {
		expect(formatRelativeTime(null)).toBe('Never');
		expect(formatRelativeTime(undefined)).toBe('Never');
	});

	it('returns seconds ago for recent timestamps', () => {
		const now = new Date().toISOString();
		expect(formatRelativeTime(now)).toBe('0s ago');
	});
});

describe('formatOfflineDuration', () => {
	it('returns "Offline" for null', () => {
		expect(formatOfflineDuration(null, Date.now())).toBe('Offline');
	});

	it('returns "Offline" for undefined', () => {
		expect(formatOfflineDuration(undefined, Date.now())).toBe('Offline');
	});

	it('returns "Offline" when diff is 0 or negative (clock skew)', () => {
		const future = new Date(Date.now() + 5000).toISOString();
		expect(formatOfflineDuration(future, Date.now())).toBe('Offline');
	});

	it('formats seconds', () => {
		const lastSeen = new Date(Date.now() - 30_000).toISOString();
		expect(formatOfflineDuration(lastSeen, Date.now())).toBe('Offline for 30s');
	});

	it('formats minutes', () => {
		const lastSeen = new Date(Date.now() - 5 * 60_000).toISOString();
		expect(formatOfflineDuration(lastSeen, Date.now())).toBe('Offline for 5m');
	});

	it('formats hours', () => {
		const lastSeen = new Date(Date.now() - 3 * 3600_000).toISOString();
		expect(formatOfflineDuration(lastSeen, Date.now())).toBe('Offline for 3h');
	});

	it('formats days', () => {
		const lastSeen = new Date(Date.now() - 2 * 86400_000).toISOString();
		expect(formatOfflineDuration(lastSeen, Date.now())).toBe('Offline for 2d');
	});
});

describe('formatDateTime', () => {
	it('returns "-" for null/undefined', () => {
		expect(formatDateTime(null)).toBe('-');
		expect(formatDateTime(undefined)).toBe('-');
	});

	it('formats a valid date', () => {
		const result = formatDateTime('2024-01-15T12:00:00Z');
		expect(result).toBeTruthy();
		expect(result).not.toBe('-');
	});
});

describe('getIntervalForTimeRange', () => {
	it('returns empty for 1h (raw data)', () => {
		expect(getIntervalForTimeRange('1h')).toBe('');
	});

	it('returns 10m for 12h', () => {
		expect(getIntervalForTimeRange('12h')).toBe('10m');
	});

	it('returns 15m for 24h', () => {
		expect(getIntervalForTimeRange('24h')).toBe('15m');
	});

	it('returns 2h for 7d', () => {
		expect(getIntervalForTimeRange('7d')).toBe('2h');
	});

	it('returns 8h for 30d', () => {
		expect(getIntervalForTimeRange('30d')).toBe('8h');
	});
});

describe('getTimeRangeTimestamps', () => {
	it('returns start and end for valid range', () => {
		const result = getTimeRangeTimestamps('1h');
		expect(result).not.toBeNull();
		if (result) {
			const start = new Date(result.start);
			const end = new Date(result.end);
			const diffMs = end.getTime() - start.getTime();
			// Should be approximately 1 hour (3600000ms)
			expect(diffMs).toBeGreaterThan(3599000);
			expect(diffMs).toBeLessThan(3601000);
		}
	});

	it('returns null for invalid range', () => {
		const result = getTimeRangeTimestamps('invalid' as any);
		expect(result).toBeNull();
	});
});

describe('parsePortBadges', () => {
	it('returns empty array for empty string', () => {
		expect(parsePortBadges('')).toEqual([]);
	});

	it('extracts public port from published mapping', () => {
		expect(parsePortBadges('8080:80/tcp')).toEqual(['8080']);
	});

	it('extracts port from exposed-only binding (no public port)', () => {
		expect(parsePortBadges('80/tcp')).toEqual(['80']);
	});

	it('handles port with no protocol suffix', () => {
		expect(parsePortBadges('8080')).toEqual(['8080']);
	});

	it('handles same public and private port', () => {
		expect(parsePortBadges('443:443/tcp')).toEqual(['443']);
	});

	it('handles multiple published ports', () => {
		expect(parsePortBadges('80:80/tcp, 443:443/tcp')).toEqual(['80', '443']);
	});

	it('handles mixed published and exposed-only ports', () => {
		expect(parsePortBadges('8080:80/tcp, 9000/tcp')).toEqual(['8080', '9000']);
	});
});

describe('isSystemContainer', () => {
	it('returns true for LXC containers', () => {
		expect(isSystemContainer({ environment_type: 'container', container_runtime: 'lxc' })).toBe(
			true
		);
	});

	it('returns false for other container runtimes (docker, systemd-nspawn)', () => {
		expect(isSystemContainer({ environment_type: 'container', container_runtime: 'docker' })).toBe(
			false
		);
		expect(
			isSystemContainer({ environment_type: 'container', container_runtime: 'systemd-nspawn' })
		).toBe(false);
	});

	it('returns false for non-container environments', () => {
		expect(isSystemContainer({ environment_type: 'vm', container_runtime: null })).toBe(false);
		expect(isSystemContainer({ environment_type: 'physical', container_runtime: null })).toBe(
			false
		);
	});
});

describe('toggleCategory', () => {
	it('adds a category when absent', () => {
		expect(toggleCategory(['alerts'], 'transactional')).toEqual(['alerts', 'transactional']);
	});

	it('removes a category when present', () => {
		expect(toggleCategory(['alerts', 'transactional'], 'alerts')).toEqual(['transactional']);
	});

	it('can empty the list', () => {
		expect(toggleCategory(['alerts'], 'alerts')).toEqual([]);
	});

	it('does not mutate the input', () => {
		const input: NotificationCategory[] = ['alerts'];
		toggleCategory(input, 'transactional');
		expect(input).toEqual(['alerts']);
	});
});

describe('cpuBarClass', () => {
	it('maps cpu load to color tiers', () => {
		expect(cpuBarClass(90)).toBe('bg-danger');
		expect(cpuBarClass(60)).toBe('bg-warning');
		expect(cpuBarClass(10)).toBe('bg-success');
	});
});

describe('memBarClass', () => {
	it('maps memory percent to color tiers', () => {
		expect(memBarClass(95)).toBe('bg-danger');
		expect(memBarClass(75)).toBe('bg-warning');
		expect(memBarClass(20)).toBe('bg-primary');
	});
});

describe('healthBadgeClass', () => {
	it('returns healthy/unhealthy/starting/fallback classes', () => {
		expect(healthBadgeClass('healthy')).toContain('text-success');
		expect(healthBadgeClass('unhealthy')).toContain('text-destructive');
		expect(healthBadgeClass('starting')).toContain('text-warning');
		expect(healthBadgeClass('')).toContain('text-muted-foreground');
	});
});

describe('memoryPercent', () => {
	it('returns 0 when limit is 0', () => {
		expect(memoryPercent(100, 0)).toBe(0);
	});
	it('clamps to 100 and computes ratio', () => {
		expect(memoryPercent(50, 100)).toBe(50);
		expect(memoryPercent(200, 100)).toBe(100);
	});
});

function makeContainer(over: Partial<GlobalContainer> = {}): GlobalContainer {
	return {
		host_id: 'h1',
		host_name: 'host-1',
		host_status: 'online',
		container_id: 'c1',
		container_name: 'web',
		image: 'nginx:latest',
		cpu_percent: 0,
		memory_used_bytes: 0,
		memory_limit_bytes: 0,
		network_rx_bytes_per_sec: 0,
		network_tx_bytes_per_sec: 0,
		runtime: 'docker',
		status: 'Up 2 hours',
		health: 'healthy',
		ports: '',
		updated_at: new Date().toISOString(),
		...over
	};
}

describe('containerIsLive', () => {
	it('is live only when host is online', () => {
		expect(containerIsLive('online')).toBe(true);
		expect(containerIsLive('offline')).toBe(false);
		expect(containerIsLive('paused')).toBe(false);
	});
});

describe('filterContainers', () => {
	const list = [
		makeContainer({
			container_id: 'c1',
			container_name: 'web',
			image: 'nginx',
			host_id: 'h1',
			runtime: 'docker',
			host_status: 'online'
		}),
		makeContainer({
			container_id: 'c2',
			container_name: 'db',
			image: 'postgres',
			host_id: 'h2',
			runtime: 'podman',
			host_status: 'offline'
		})
	];
	const base = { search: '', host: '', runtime: '', liveness: 'all' as const };

	it('returns all with empty filters', () => {
		expect(filterContainers(list, base)).toHaveLength(2);
	});
	it('search matches name or image, case-insensitive', () => {
		expect(filterContainers(list, { ...base, search: 'WEB' })).toHaveLength(1);
		expect(filterContainers(list, { ...base, search: 'postgres' })[0].container_id).toBe('c2');
	});
	it('filters by host and runtime', () => {
		expect(filterContainers(list, { ...base, host: 'h2' })[0].container_id).toBe('c2');
		expect(filterContainers(list, { ...base, runtime: 'docker' })[0].container_id).toBe('c1');
	});
	it('filters by liveness', () => {
		expect(filterContainers(list, { ...base, liveness: 'live' })[0].container_id).toBe('c1');
		expect(filterContainers(list, { ...base, liveness: 'stale' })[0].container_id).toBe('c2');
	});
});
