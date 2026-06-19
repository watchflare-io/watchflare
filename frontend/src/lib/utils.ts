import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';
import type { SSEEvent, TimeRange } from './types';
import { toasts } from './stores/toasts';
import { TOAST_LONG_DURATION } from './constants';

// Tailwind class name utility
export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

// Type utilities
export type WithoutChild<T> = T extends { child?: unknown } ? Omit<T, 'child'> : T;
export type WithoutChildren<T> = T extends { children?: unknown } ? Omit<T, 'children'> : T;
export type WithoutChildrenOrChild<T> = WithoutChildren<WithoutChild<T>>;
export type WithElementRef<T, U extends HTMLElement = HTMLElement> = T & { ref?: U | null };

// Time range utilities
export interface TimeRangeOption {
	value: TimeRange;
	label: string;
	seconds: number;
}

export const TIME_RANGES: TimeRangeOption[] = [
	{ value: '1h', label: '1 Hour', seconds: 3600 },
	{ value: '12h', label: '12 Hours', seconds: 43200 },
	{ value: '24h', label: '24 Hours', seconds: 86400 },
	{ value: '7d', label: '7 Days', seconds: 604800 },
	{ value: '30d', label: '30 Days', seconds: 2592000 }
];

export interface TimeRangeTimestamps {
	start: string;
	end: string;
}

export function getTimeRangeTimestamps(timeRange: TimeRange): TimeRangeTimestamps | null {
	const range = TIME_RANGES.find((r) => r.value === timeRange);
	if (!range) return null;

	const end = new Date();
	const start = new Date(end.getTime() - range.seconds * 1000);

	return {
		start: start.toISOString(),
		end: end.toISOString()
	};
}

export function getIntervalForTimeRange(timeRange: TimeRange): string {
	const intervals: Record<TimeRange, string> = {
		'1h': '', // Raw data (every 30s) - 120 points
		'12h': '10m', // Continuous aggregate 10min - 72 points
		'24h': '15m', // Continuous aggregate 15min - 96 points
		'7d': '2h', // Continuous aggregate 2h - 84 points
		'30d': '8h' // Continuous aggregate 8h - 90 points
	};
	return intervals[timeRange] || '';
}

// Format bytes to human-readable
export function formatBytes(bytes: number): string {
	if (bytes <= 0) return '0 B';
	const k = 1024;
	const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
	const i = Math.floor(Math.log(bytes) / Math.log(k));
	return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + ' ' + sizes[i];
}

// Format percentage
export function formatPercent(value: number): string {
	return Math.round(value * 10) / 10 + '%';
}

// Capitalize the first character of a string. Used to render backend error
// messages, which are lowercase by Go convention, with a leading capital.
export function capitalizeFirst(s: string): string {
	if (!s) return s;
	return s.charAt(0).toUpperCase() + s.slice(1);
}

// Format uptime
export function formatUptime(seconds: number): string {
	const days = Math.floor(seconds / 86400);
	const hours = Math.floor((seconds % 86400) / 3600);
	const minutes = Math.floor((seconds % 3600) / 60);

	if (days > 0) return `${days}d ${hours}h`;
	if (hours > 0) return `${hours}h ${minutes}m`;
	return `${minutes}m`;
}

// Host status badge class
export function getStatusClass(status: string): string {
	switch (status) {
		case 'online':
			return 'bg-success/10 text-success border-success/20';
		case 'pending':
			return 'bg-warning/10 text-warning border-warning/20';
		case 'ip_mismatch':
			return 'bg-warning/10 text-warning border-warning/20';
		case 'offline':
			return 'bg-danger/10 text-danger border-danger/20';
		case 'paused':
		case 'expired':
		default:
			return 'bg-muted text-muted-foreground border-border';
	}
}

// Metric threshold class (CPU, memory, disk percentages)
export function getMetricClass(
	percent: number,
	warningThreshold = 70,
	criticalThreshold = 90
): string {
	if (percent >= criticalThreshold) return 'text-danger font-semibold';
	if (percent >= warningThreshold) return 'text-warning font-medium';
	return 'text-foreground';
}

// Format timestamp as relative time ("5s ago", "3m ago", "2h ago", "1d ago")
export function formatRelativeTime(dateString: string | null | undefined): string {
	if (!dateString) return 'Never';
	const date = new Date(dateString);
	const now = new Date();
	const diff = Math.floor((now.getTime() - date.getTime()) / 1000);

	if (diff < 60) return `${diff}s ago`;
	if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
	if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
	return `${Math.floor(diff / 86400)}d ago`;
}

// Format offline duration ("Offline for 3m", "Offline for 2h", etc.)
// Accepts a nowMs parameter so the caller can pass a reactive timestamp for live updates.
export function formatOfflineDuration(lastSeen: string | null | undefined, nowMs: number): string {
	if (!lastSeen) return 'Offline';
	const diff = Math.floor((nowMs - new Date(lastSeen).getTime()) / 1000);
	if (diff <= 0) return 'Offline';
	if (diff < 60) return `Offline for ${diff}s`;
	if (diff < 3600) return `Offline for ${Math.floor(diff / 60)}m`;
	if (diff < 86400) return `Offline for ${Math.floor(diff / 3600)}h`;
	return `Offline for ${Math.floor(diff / 86400)}d`;
}

// Format date as locale string
export function formatDateTime(
	dateString: string | null | undefined,
	timeFormat: '12h' | '24h' = '24h'
): string {
	if (!dateString) return '-';
	return new Date(dateString).toLocaleString('en-US', {
		year: 'numeric',
		month: 'short',
		day: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
		hour12: timeFormat === '12h'
	});
}

// Package manager display helpers
export const MANAGER_LABELS: Record<string, string> = {
	brew: 'Homebrew',
	dpkg: 'apt/dpkg',
	rpm: 'dnf/rpm',
	pacman: 'Pacman',
	apk: 'Alpine apk',
	zypper: 'Zypper',
	snap: 'Snap',
	flatpak: 'Flatpak',
	appimage: 'AppImage',
	npm: 'npm',
	yarn: 'Yarn',
	pnpm: 'pnpm',
	pip: 'pip',
	poetry: 'Poetry',
	pipx: 'pipx',
	uv: 'uv',
	conda: 'Conda',
	mamba: 'Mamba',
	gem: 'RubyGems',
	cargo: 'Cargo',
	composer: 'Composer',
	nuget: 'NuGet',
	maven: 'Maven',
	macports: 'MacPorts',
	pkgutil: 'macOS pkgutil',
	macos_apps: 'macOS Apps',
	nix: 'Nix',
	cli_tools: 'CLI Tools'
};

export const MANAGER_COLORS: Record<string, string> = {
	brew: 'bg-(--chart-4)/10 text-(--chart-4) border-(--chart-4)/20',
	dpkg: 'bg-(--chart-2)/10 text-(--chart-2) border-(--chart-2)/20',
	rpm: 'bg-(--chart-1)/10 text-(--chart-1) border-(--chart-1)/20',
	pacman: 'bg-(--chart-5)/10 text-(--chart-5) border-(--chart-5)/20',
	apk: 'bg-(--chart-3)/10 text-(--chart-3) border-(--chart-3)/20',
	zypper: 'bg-(--chart-1)/10 text-(--chart-1) border-(--chart-1)/20',
	snap: 'bg-(--chart-2)/10 text-(--chart-2) border-(--chart-2)/20',
	flatpak: 'bg-(--chart-3)/10 text-(--chart-3) border-(--chart-3)/20',
	npm: 'bg-(--chart-5)/10 text-(--chart-5) border-(--chart-5)/20',
	yarn: 'bg-(--chart-4)/10 text-(--chart-4) border-(--chart-4)/20',
	pnpm: 'bg-(--chart-2)/10 text-(--chart-2) border-(--chart-2)/20',
	pip: 'bg-(--chart-3)/10 text-(--chart-3) border-(--chart-3)/20',
	poetry: 'bg-(--chart-4)/10 text-(--chart-4) border-(--chart-4)/20',
	cargo: 'bg-(--chart-1)/10 text-(--chart-1) border-(--chart-1)/20',
	gem: 'bg-(--chart-5)/10 text-(--chart-5) border-(--chart-5)/20',
	cli_tools: 'bg-(--chart-2)/10 text-(--chart-2) border-(--chart-2)/20'
};

export function getManagerLabel(manager: string): string {
	return MANAGER_LABELS[manager] || manager;
}

export function getManagerColor(manager: string): string {
	return MANAGER_COLORS[manager] || 'bg-muted text-muted-foreground border-border';
}

// SSE reactivation toast (shared across pages)
export function handleSSEReactivation(event: SSEEvent): void {
	if (event.type === 'host_update' && event.data.reactivated && event.data.hostname) {
		toasts.add(
			`Agent "${event.data.hostname}" was reactivated (same physical host detected via UUID)`,
			'info',
			TOAST_LONG_DURATION
		);
	}
}

// Dev-only logger (silenced in production builds)
export const logger = {
	error: (...args: unknown[]) => {
		if (import.meta.env.DEV) console.error(...args);
	},
	warn: (...args: unknown[]) => {
		if (import.meta.env.DEV) console.warn(...args);
	},
	log: (...args: unknown[]) => {
		if (import.meta.env.DEV) console.log(...args);
	}
};

// isAgentOutdated returns true if current < latest using semver comparison.
// Returns false if either version is missing or unparseable.
export function isAgentOutdated(
	current: string | null | undefined,
	latest: string | null | undefined
): boolean {
	if (!current || !latest || current === 'dev') return false;
	const parse = (v: string): [number, number, number] => {
		const parts = v
			.replace(/^v/, '')
			.split('.')
			.map((p) => parseInt(p.split('-')[0], 10));
		return [parts[0] || 0, parts[1] || 0, parts[2] || 0];
	};
	const [cMaj, cMin, cPat] = parse(current);
	const [lMaj, lMin, lPat] = parse(latest);
	if (lMaj !== cMaj) return lMaj > cMaj;
	if (lMin !== cMin) return lMin > cMin;
	return lPat > cPat;
}

// parsePortBadges extracts public port numbers from agent port strings.
// Input formats: "8080:80/tcp" (PublicPort:PrivatePort/Protocol), "80/tcp" (PrivatePort/Protocol only)
// Returns: array of public port strings (or private port if no public binding)
export function parsePortBadges(ports: string): string[] {
	if (!ports) return [];
	return ports.split(', ').map((p) => {
		const colonIdx = p.indexOf(':');
		if (colonIdx !== -1) {
			return p.substring(0, colonIdx);
		}
		const slashIdx = p.indexOf('/');
		return slashIdx !== -1 ? p.substring(0, slashIdx) : p;
	});
}
