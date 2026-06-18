import type { SSEEvent, Metric, ContainerMetric, SensorReading } from '../types';
import { API_BASE_URL } from '../api';
import { logger } from '../utils';

/**
 * Connection states for SSE
 */
export type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'reconnecting' | 'error';

/**
 * Minified sensor reading format from backend SSE
 */
interface MinifiedSensorReading {
	k: string;  // sensor key
	v: number;  // temperature_celsius
}

/**
 * Minified metrics format from backend SSE
 */
interface MinifiedMetrics {
	h: string;       // host_id
	t: number;       // timestamp (Unix epoch)
	c: number;       // cpu_usage_percent
	iw: number;      // cpu_iowait_percent (Linux only)
	sl: number;      // cpu_steal_percent (Linux VMs only)
	mu: number;      // memory_used_bytes
	mt: number;      // memory_total_bytes
	ma: number;      // memory_available_bytes
	mb: number;      // memory_buffers_bytes (Linux only)
	mc: number;      // memory_cached_bytes (Linux only)
	st: number;      // swap_total_bytes
	su: number;      // swap_used_bytes
	du: number;      // disk_used_bytes
	dt: number;      // disk_total_bytes
	l1: number;      // load_avg_1min
	l5: number;      // load_avg_5min
	l15: number;     // load_avg_15min
	dr: number;      // disk_read_bytes_per_sec
	dw: number;      // disk_write_bytes_per_sec
	nr: number;      // network_rx_bytes_per_sec
	nt: number;      // network_tx_bytes_per_sec
	tmp: number;     // cpu_temperature_celsius
	u: number;       // uptime_seconds
	pr: number;      // processes_count
	sr?: MinifiedSensorReading[]; // all sensor readings
}

/**
 * Minified container metrics format from backend SSE
 */
interface MinifiedContainerMetric {
	i: string;    // container_id
	n: string;    // container_name
	c: number;    // cpu_percent
	mu: number;   // memory_used_bytes
	ml: number;   // memory_limit_bytes
	nr: number;   // network_rx_bytes_per_sec
	nt: number;   // network_tx_bytes_per_sec
	r?: string;   // runtime ("docker", "podman")
	st?: string;  // status ("Up 2 hours")
	hl?: string;  // health ("healthy", "unhealthy", "starting", "")
	po?: string;  // ports ("8080:80/tcp, 443:443/tcp")
}

interface MinifiedContainerMetricsUpdate {
	h: string;   // host_id
	t: number;   // timestamp (Unix epoch)
	m: MinifiedContainerMetric[];
}

/**
 * Decode minified container metrics to full format
 */
function decodeMinifiedContainerMetrics(minified: MinifiedContainerMetricsUpdate): { host_id: string; metrics: ContainerMetric[] } {
	const timestamp = new Date(minified.t * 1000).toISOString();
	return {
		host_id: minified.h,
		metrics: minified.m.map(cm => ({
			id: '',
			host_id: minified.h,
			timestamp,
			container_id: cm.i,
			container_name: cm.n,
			image: '',
			cpu_percent: cm.c,
			memory_used_bytes: cm.mu,
			memory_limit_bytes: cm.ml,
			network_rx_bytes_per_sec: cm.nr ?? 0,
			network_tx_bytes_per_sec: cm.nt ?? 0,
			runtime: cm.r ?? '',
			status: cm.st ?? '',
			health: cm.hl ?? '',
			ports: cm.po ?? '',
		}))
	};
}

/**
 * Decode minified SSE metrics format to full format
 */
export function decodeMinifiedMetrics(minified: MinifiedMetrics): Metric {
	const sensorReadings: SensorReading[] | undefined = minified.sr?.map(sr => ({
		key: sr.k,
		temperature_celsius: sr.v
	}));

	return {
		id: 0,
		host_id: minified.h,
		timestamp: new Date(minified.t * 1000).toISOString(),
		cpu_usage_percent: minified.c,
		cpu_iowait_percent: minified.iw ?? 0,
		cpu_steal_percent: minified.sl ?? 0,
		memory_used_bytes: minified.mu,
		memory_total_bytes: minified.mt,
		memory_available_bytes: minified.ma,
		memory_buffers_bytes: minified.mb ?? 0,
		memory_cached_bytes: minified.mc ?? 0,
		swap_total_bytes: minified.st ?? 0,
		swap_used_bytes: minified.su ?? 0,
		disk_used_bytes: minified.du,
		disk_total_bytes: minified.dt,
		load_avg_1min: minified.l1,
		load_avg_5min: minified.l5,
		load_avg_15min: minified.l15,
		disk_read_bytes_per_sec: minified.dr ?? 0,
		disk_write_bytes_per_sec: minified.dw ?? 0,
		network_rx_bytes_per_sec: minified.nr ?? 0,
		network_tx_bytes_per_sec: minified.nt ?? 0,
		cpu_temperature_celsius: minified.tmp ?? 0,
		uptime_seconds: minified.u,
		processes_count: minified.pr ?? 0,
		sensor_readings: sensorReadings
	};
}

/**
 * Configuration for SSE Manager
 */
interface SSEManagerConfig {
	/** Initial retry delay in ms (default: 1000) */
	initialRetryDelay?: number;
	/** Maximum retry delay in ms (default: 30000) */
	maxRetryDelay?: number;
	/** Maximum number of retry attempts (default: Infinity) */
	maxRetries?: number;
}

/**
 * SSE Manager with automatic reconnection and exponential backoff.
 * Pass a url to connect to a specific SSE endpoint (e.g. per-host stream).
 * Defaults to the global host events stream.
 */
export class SSEManager {
	private url: string;
	private eventSource: EventSource | null = null;
	private state: ConnectionState = 'disconnected';
	private retryCount = 0;
	private retryDelay: number;
	private retryTimer: ReturnType<typeof setTimeout> | null = null;
	private shouldReconnect = true;

	private config: Required<SSEManagerConfig>;
	private onMessageCallback?: (event: SSEEvent) => void;
	private onStateChangeCallback?: (state: ConnectionState) => void;
	private onErrorCallback?: (error: Event | Error) => void;

	constructor(url = `${API_BASE_URL}/hosts/events`, config: SSEManagerConfig = {}) {
		this.url = url;
		this.config = {
			initialRetryDelay: config.initialRetryDelay ?? 1000,
			maxRetryDelay: config.maxRetryDelay ?? 30000,
			maxRetries: config.maxRetries ?? Infinity,
		};
		this.retryDelay = this.config.initialRetryDelay;
	}

	/**
	 * Connect to SSE endpoint
	 */
	connect(): void {
		if (this.eventSource) {
			return; // Already connected
		}

		this.shouldReconnect = true;
		this.setState('connecting');

		try {
			this.eventSource = new EventSource(this.url, {
				withCredentials: true
			});

			this.setupEventListeners();
		} catch (err) {
			logger.error('Failed to create EventSource:', err);
			this.handleError(err instanceof Error ? err : new Error('Failed to connect'));
		}
	}

	/**
	 * Disconnect from SSE endpoint
	 */
	disconnect(): void {
		this.shouldReconnect = false;
		this.cleanup();
		this.setState('disconnected');
	}

	/**
	 * Register message callback
	 */
	onMessage(callback: (event: SSEEvent) => void): void {
		this.onMessageCallback = callback;
	}

	/**
	 * Register state change callback
	 */
	onStateChange(callback: (state: ConnectionState) => void): void {
		this.onStateChangeCallback = callback;
	}

	/**
	 * Register error callback
	 */
	onError(callback: (error: Event | Error) => void): void {
		this.onErrorCallback = callback;
	}

	/**
	 * Get current connection state
	 */
	getState(): ConnectionState {
		return this.state;
	}

	/**
	 * Setup event listeners on EventSource
	 */
	private setupEventListeners(): void {
		if (!this.eventSource) return;

		this.eventSource.addEventListener('open', () => {
			logger.log('SSE connection opened');
			this.setState('connected');
			this.retryCount = 0;
			this.retryDelay = this.config.initialRetryDelay;
		});

		this.eventSource.addEventListener('connected', (e: MessageEvent) => {
			const data = JSON.parse(e.data) as { client_id: string };
			logger.log('SSE connected:', data.client_id);
		});

		this.eventSource.addEventListener('host_update', (e: MessageEvent) => {
			const data = JSON.parse(e.data);
			this.bufferEvent({ type: 'host_update', data });
		});

		this.eventSource.addEventListener('metrics_update', (e: MessageEvent) => {
			const minified = JSON.parse(e.data) as MinifiedMetrics;
			const data = decodeMinifiedMetrics(minified);
			this.bufferEvent({ type: 'metrics_update', data });
		});

		this.eventSource.addEventListener('aggregated_metrics_update', (e: MessageEvent) => {
			const data = JSON.parse(e.data);
			this.bufferEvent({ type: 'aggregated_metrics_update', data });
		});

		this.eventSource.addEventListener('container_metrics_update', (e: MessageEvent) => {
			const minified = JSON.parse(e.data) as MinifiedContainerMetricsUpdate;
			const data = decodeMinifiedContainerMetrics(minified);
			this.bufferEvent({ type: 'container_metrics_update', data });
		});

		this.eventSource.addEventListener('package_inventory_update', (e: MessageEvent) => {
			const data = JSON.parse(e.data);
			this.bufferEvent({ type: 'package_inventory_update', data });
		});

		this.eventSource.addEventListener('incidents_changed', () => {
			this.bufferEvent({ type: 'incidents_changed', data: {} });
		});

		this.eventSource.onerror = (error: Event) => {
			logger.error('SSE error:', error);
			this.handleError(error);
		};
	}

	/**
	 * Emit event to the registered message callback
	 */
	private bufferEvent(event: SSEEvent): void {
		this.onMessageCallback?.(event);
	}

	/**
	 * Handle error and trigger reconnection
	 */
	private handleError(error: Event | Error): void {
		if (this.onErrorCallback) {
			this.onErrorCallback(error);
		}

		// Check if we should reconnect
		if (!this.shouldReconnect) {
			this.setState('disconnected');
			return;
		}

		if (this.retryCount >= this.config.maxRetries) {
			logger.error('Max retry attempts reached');
			this.setState('error');
			this.shouldReconnect = false;
			return;
		}

		this.setState('reconnecting');
		this.scheduleReconnect();
	}

	/**
	 * Schedule reconnection with exponential backoff
	 */
	private scheduleReconnect(): void {
		if (this.retryTimer) {
			clearTimeout(this.retryTimer);
		}

		logger.log(`Reconnecting in ${this.retryDelay}ms... (attempt ${this.retryCount + 1})`);

		this.retryTimer = setTimeout(() => {
			this.retryCount++;
			this.cleanup();
			this.connect();

			// Exponential backoff: double the delay, cap at maxRetryDelay
			this.retryDelay = Math.min(this.retryDelay * 2, this.config.maxRetryDelay);
		}, this.retryDelay);
	}

	/**
	 * Set connection state and notify listeners
	 */
	private setState(state: ConnectionState): void {
		if (this.state === state) return;

		this.state = state;
		logger.log(`SSE state: ${state}`);

		if (this.onStateChangeCallback) {
			this.onStateChangeCallback(state);
		}
	}

	/**
	 * Cleanup resources
	 */
	private cleanup(): void {
		if (this.retryTimer) {
			clearTimeout(this.retryTimer);
			this.retryTimer = null;
		}

		if (this.eventSource) {
			this.eventSource.close();
			this.eventSource = null;
		}
	}
}
