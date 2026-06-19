import { describe, it, expect } from 'vitest';
import { decodeMinifiedMetrics } from './manager';

const baseMinified = {
	h: 'host-abc',
	t: 1700000000,
	c: 42.5,
	iw: 3.2,
	sl: 1.1,
	mu: 4000000000,
	mt: 8000000000,
	ma: 4000000000,
	mb: 512000000,
	mc: 1024000000,
	st: 2000000000,
	su: 500000000,
	du: 250000000000,
	dt: 500000000000,
	l1: 1.1,
	l5: 1.5,
	l15: 1.9,
	dr: 1024,
	dw: 2048,
	nr: 512,
	nt: 256,
	tmp: 65.0,
	u: 3600,
	pr: 142
};

describe('decodeMinifiedMetrics', () => {
	it('maps host_id and timestamp', () => {
		const result = decodeMinifiedMetrics(baseMinified);
		expect(result.host_id).toBe('host-abc');
		expect(result.timestamp).toBe(new Date(1700000000 * 1000).toISOString());
	});

	it('maps cpu and memory fields', () => {
		const result = decodeMinifiedMetrics(baseMinified);
		expect(result.cpu_usage_percent).toBe(42.5);
		expect(result.cpu_iowait_percent).toBe(3.2);
		expect(result.cpu_steal_percent).toBe(1.1);
		expect(result.memory_used_bytes).toBe(4000000000);
		expect(result.memory_total_bytes).toBe(8000000000);
		expect(result.memory_available_bytes).toBe(4000000000);
		expect(result.memory_buffers_bytes).toBe(512000000);
		expect(result.memory_cached_bytes).toBe(1024000000);
	});

	it('maps swap fields', () => {
		const result = decodeMinifiedMetrics(baseMinified);
		expect(result.swap_total_bytes).toBe(2000000000);
		expect(result.swap_used_bytes).toBe(500000000);
	});

	it('maps processes_count', () => {
		const result = decodeMinifiedMetrics(baseMinified);
		expect(result.processes_count).toBe(142);
	});

	it('maps disk fields', () => {
		const result = decodeMinifiedMetrics(baseMinified);
		expect(result.disk_used_bytes).toBe(250000000000);
		expect(result.disk_total_bytes).toBe(500000000000);
		expect(result.disk_read_bytes_per_sec).toBe(1024);
		expect(result.disk_write_bytes_per_sec).toBe(2048);
	});

	it('maps network fields', () => {
		const result = decodeMinifiedMetrics(baseMinified);
		expect(result.network_rx_bytes_per_sec).toBe(512);
		expect(result.network_tx_bytes_per_sec).toBe(256);
	});

	it('maps load avg and uptime', () => {
		const result = decodeMinifiedMetrics(baseMinified);
		expect(result.load_avg_1min).toBe(1.1);
		expect(result.load_avg_5min).toBe(1.5);
		expect(result.load_avg_15min).toBe(1.9);
		expect(result.uptime_seconds).toBe(3600);
	});

	it('maps cpu_temperature_celsius', () => {
		const result = decodeMinifiedMetrics(baseMinified);
		expect(result.cpu_temperature_celsius).toBe(65.0);
	});

	it('defaults swap/processes/linux-only fields to 0 when absent (??)', () => {
		const result = decodeMinifiedMetrics({
			...baseMinified,
			iw: undefined as unknown as number,
			sl: undefined as unknown as number,
			mb: undefined as unknown as number,
			mc: undefined as unknown as number,
			st: undefined as unknown as number,
			su: undefined as unknown as number,
			pr: undefined as unknown as number
		});
		expect(result.cpu_iowait_percent).toBe(0);
		expect(result.cpu_steal_percent).toBe(0);
		expect(result.memory_buffers_bytes).toBe(0);
		expect(result.memory_cached_bytes).toBe(0);
		expect(result.swap_total_bytes).toBe(0);
		expect(result.swap_used_bytes).toBe(0);
		expect(result.processes_count).toBe(0);
	});

	it('decodes sensor readings when present', () => {
		const result = decodeMinifiedMetrics({
			...baseMinified,
			sr: [{ k: 'cpu', v: 72.0 }]
		});
		expect(result.sensor_readings).toHaveLength(1);
		expect(result.sensor_readings![0].key).toBe('cpu');
		expect(result.sensor_readings![0].temperature_celsius).toBe(72.0);
	});

	it('leaves sensor_readings undefined when absent', () => {
		const result = decodeMinifiedMetrics(baseMinified);
		expect(result.sensor_readings).toBeUndefined();
	});
});
