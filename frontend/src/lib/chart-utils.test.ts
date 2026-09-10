import { describe, it, expect } from 'vitest';
import { formatRate, formatTemperature, formatTooltipDate, escapeHtml } from './chart-utils';

describe('formatRate (bytes mode)', () => {
	it('returns 0 B/s for 0', () => {
		expect(formatRate(0)).toBe('0 B/s');
	});

	it('returns 0 B/s for negative', () => {
		expect(formatRate(-1)).toBe('0 B/s');
	});

	it('formats bytes per second', () => {
		expect(formatRate(500)).toBe('500 B/s');
	});

	it('formats KB/s', () => {
		expect(formatRate(1024)).toBe('1 KB/s');
	});

	it('formats MB/s', () => {
		expect(formatRate(1024 * 1024)).toBe('1 MB/s');
	});

	it('formats GB/s', () => {
		expect(formatRate(1024 * 1024 * 1024)).toBe('1 GB/s');
	});

	it('formats with one decimal', () => {
		expect(formatRate(1536)).toBe('1.5 KB/s');
	});
});

describe('formatRate (bits mode)', () => {
	it('returns 0 bps for 0', () => {
		expect(formatRate(0, 'bits')).toBe('0 bps');
	});

	it('formats bps', () => {
		expect(formatRate(100, 'bits')).toBe('800 bps');
	});

	it('formats Kbps', () => {
		expect(formatRate(1000, 'bits')).toBe('8 Kbps');
	});

	it('formats Mbps', () => {
		expect(formatRate(1_000_000, 'bits')).toBe('8 Mbps');
	});

	it('formats Gbps', () => {
		expect(formatRate(1_000_000_000, 'bits')).toBe('8 Gbps');
	});

	it('formats with one decimal', () => {
		// 1500 bytes/s * 8 = 12000 bits/s = 12 Kbps
		expect(formatRate(1500, 'bits')).toBe('12 Kbps');
	});
});

describe('formatTemperature', () => {
	it('returns N/A for 0', () => {
		expect(formatTemperature(0)).toBe('N/A');
	});

	it('formats celsius by default', () => {
		expect(formatTemperature(42.5)).toBe('42.5°C');
	});

	it('formats celsius explicitly', () => {
		expect(formatTemperature(100, 'celsius')).toBe('100.0°C');
	});

	it('converts to fahrenheit', () => {
		expect(formatTemperature(0, 'fahrenheit')).toBe('N/A');
	});

	it('converts 100°C to 212°F', () => {
		expect(formatTemperature(100, 'fahrenheit')).toBe('212.0°F');
	});

	it('converts 37°C to 98.6°F', () => {
		expect(formatTemperature(37, 'fahrenheit')).toBe('98.6°F');
	});
});

describe('formatTooltipDate', () => {
	it('returns a non-empty string', () => {
		const result = formatTooltipDate(new Date('2024-06-15T10:30:45Z'));
		expect(result).toBeTruthy();
		expect(typeof result).toBe('string');
	});

	it('includes time components', () => {
		const result = formatTooltipDate(new Date('2024-06-15T10:30:45Z'));
		// Should contain minutes and seconds
		expect(result).toMatch(/\d+:\d+:\d+/);
	});

	it('accepts 12h format', () => {
		const result = formatTooltipDate(new Date('2024-06-15T10:30:45Z'), '12h');
		expect(result).toBeTruthy();
	});
});

describe('escapeHtml', () => {
	it('escapes ampersand, angles, and quotes', () => {
		expect(escapeHtml(`a&b<c>"d"'e'`)).toBe('a&amp;b&lt;c&gt;&quot;d&quot;&#39;e&#39;');
	});

	it('leaves safe labels unchanged', () => {
		expect(escapeHtml('nginx:latest')).toBe('nginx:latest');
	});

	it('neutralizes a script payload used as a series label', () => {
		expect(escapeHtml('<img src=x onerror=alert(1)>')).toBe('&lt;img src=x onerror=alert(1)&gt;');
	});
});
