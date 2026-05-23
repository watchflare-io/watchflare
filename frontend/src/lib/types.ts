/**
 * TypeScript type definitions for Watchflare Frontend
 */

// ===== User & Authentication =====

export type Theme = "light" | "dark" | "system";
export type TimeFormat = "24h" | "12h";
export type TemperatureUnit = "celsius" | "fahrenheit";
export type NetworkUnit = "bytes" | "bits";
export type DiskUnit = "bytes" | "bits";

export interface User {
  id: number;
  email: string;
  username: string;
  default_time_range: TimeRange;
  theme: Theme;
  time_format: TimeFormat;
  temperature_unit: TemperatureUnit;
  network_unit: NetworkUnit;
  disk_unit: DiskUnit;
  gauge_warning_threshold: number;
  gauge_critical_threshold: number;
  created_at: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  email: string;
  password: string;
}

export interface ChangePasswordRequest {
  current_password: string;
  new_password: string;
}

// ===== Host =====

export type HostStatus =
  | "online"
  | "offline"
  | "pending"
  | "paused"
  | "ip_mismatch";
export type EnvironmentType =
  | "physical"
  | "physical_with_containers"
  | "vm"
  | "vm_with_containers";

export interface Host {
  id: string;
  display_name: string;
  hostname: string;
  os: string | null;
  platform: string | null;
  platform_version: string | null;
  platform_family: string | null;
  kernel_version: string | null;
  kernel_arch: string | null;
  ip_address_v4: string;
  ip_address_v6: string | null;
  configured_ip: string;
  ignore_ip_mismatch: boolean;
  status: HostStatus;
  last_seen: string | null;
  created_at: string;
  environment_type: EnvironmentType;
  container_runtime: string | null;
  virtualization_system: string | null;
  virtualization_role: string | null;
  host_id: string | null;
  cpu_model_name: string | null;
  cpu_physical_count: number | null;
  cpu_logical_count: number | null;
  cpu_mhz: number | null;
  reactivated_at: string | null;
  agent_version: string | null;
  agent_uuid: string;
  // Package update counts — included in list responses
  outdated_count?: number;
  security_count?: number;
}

export interface HostWithMetrics {
  host: Host;
  latestMetric?: Metric;
}

export interface CreateHostRequest {
  display_name: string;
  configured_ip?: string;
  allow_any_ip?: boolean;
}

export interface UpdateConfiguredIPRequest {
  configured_ip: string;
}

// ===== Metrics =====

export type TimeRange = "1h" | "12h" | "24h" | "7d" | "30d";

export interface SensorReading {
  key: string;
  temperature_celsius: number;
}

export interface SensorDataPoint {
  timestamp: string;
  sensor_readings: SensorReading[];
}

export interface Metric {
  id: number;
  host_id: string;
  timestamp: string;
  cpu_usage_percent: number;
  cpu_iowait_percent: number;
  cpu_steal_percent: number;
  memory_used_bytes: number;
  memory_total_bytes: number;
  memory_available_bytes: number;
  memory_buffers_bytes: number;
  memory_cached_bytes: number;
  swap_total_bytes: number;
  swap_used_bytes: number;
  disk_used_bytes: number;
  disk_total_bytes: number;
  load_avg_1min: number;
  load_avg_5min: number;
  load_avg_15min: number;
  uptime_seconds: number;
  processes_count: number;
  disk_read_bytes_per_sec: number;
  disk_write_bytes_per_sec: number;
  network_rx_bytes_per_sec: number;
  network_tx_bytes_per_sec: number;
  cpu_temperature_celsius: number;
  sensor_readings?: SensorReading[];
}

export interface AggregatedMetric {
  timestamp: string;
  cpu_usage_percent: number;
  memory_used_bytes: number;
  memory_total_bytes: number;
  memory_available_bytes: number;
  disk_used_bytes: number;
  disk_total_bytes: number;
  load_avg_1min: number;
  load_avg_5min: number;
  load_avg_15min: number;
  disk_read_bytes_per_sec: number;
  disk_write_bytes_per_sec: number;
  network_rx_bytes_per_sec: number;
  network_tx_bytes_per_sec: number;
  cpu_temperature_celsius: number;
  host_count: number;
}

export interface ContainerMetric {
  id: string;
  host_id: string;
  timestamp: string;
  container_id: string;
  container_name: string;
  image: string;
  cpu_percent: number;
  memory_used_bytes: number;
  memory_limit_bytes: number;
  network_rx_bytes_per_sec: number;
  network_tx_bytes_per_sec: number;
  runtime?: string;
  status?: string;
  health?: string;
  ports?: string;
}

export interface MetricsQueryParams {
  time_range?: TimeRange;
  limit?: number;
  offset?: number;
}

// ===== Dropped Metrics =====

export interface DroppedMetric {
  hostname: string;
  total_dropped: number;
  first_dropped_at: string;
  last_dropped_at: string;
  downtime_duration: number;
}

// ===== Packages =====

export interface Package {
  id: number;
  host_id: string;
  name: string;
  version: string;
  architecture: string;
  package_manager: string;
  source: string;
  installed_at: string | null;
  package_size: number;
  description: string;
  available_version: string | null;
  has_security_update: boolean;
  update_checked: boolean;
  first_seen: string;
  last_seen: string;
}

export interface PackageManagerStat {
  package_manager: string;
  count: number;
}

export interface PackageStats {
  total_packages: number;
  recent_changes?: number;
  outdated_count: number;
  security_updates_count: number;
  by_package_manager: PackageManagerStat[];
  last_collection?: { timestamp: string } | null;
}

export interface PackageCollection {
  id: number;
  host_id: string;
  timestamp: string;
  collection_type: string;
  package_count: number;
  changes_count: number;
  duration_ms: number;
  status: string;
  error_message: string;
}

export interface PackageHistory {
  id: number;
  host_id: string;
  timestamp: string;
  name: string;
  version: string;
  architecture: string;
  package_manager: string;
  source: string;
  package_size: number;
  description: string;
  change_type: "added" | "removed" | "updated" | "initial";
}

// ===== SSE Events =====

export type SSEEventType =
  | "connected"
  | "host_update"
  | "metrics_update"
  | "aggregated_metrics_update"
  | "container_metrics_update"
  | "package_inventory_update";

export interface SSEEvent {
  type: SSEEventType;
  data: unknown;
}

export interface HostUpdateEvent {
  id: string;
  status: HostStatus;
  last_seen: string;
  ip_address_v4?: string;
  ip_address_v6?: string;
  configured_ip?: string;
  ignore_ip_mismatch?: boolean;
  reactivated?: boolean;
  hostname?: string;
  clock_desync?: boolean;
  agent_version?: string;
}

export interface MetricsUpdateEvent extends Metric {
  host_id: string;
}

export interface AggregatedMetricsUpdateEvent extends AggregatedMetric {
  // Same as AggregatedMetric
}

// ===== API Responses =====

export interface APIResponse<T> {
  success?: boolean;
  data?: T;
  error?: string;
  message?: string;
}

export interface LoginResponse {
  message: string;
  user: User;
}

export interface RegisterResponse {
  message: string;
  user: User;
}

export interface CreateHostResponse {
  message: string;
  host: Host;
  token: string;
  agent_key: string;
  backend_host: string;
}

export interface RegenerateTokenResponse {
  message: string;
  token: string;
}

export interface GetHostResponse {
  host: Host;
  clock_desync: boolean;
  latest_metrics: Metric | null;
}

export interface ListHostsResponse {
  hosts: Host[];
  total: number;
  page: number;
  per_page: number;
}

export interface HostStatsResponse {
  total: number;       // all hosts excluding pending
  online: number;
  offline: number;
  pending: number;
  paused: number;
  ip_mismatch: number;
}

export interface GetMetricsResponse {
  metrics: Metric[];
}

export interface GetAggregatedMetricsResponse {
  metrics: AggregatedMetric[];
}

export interface GetDroppedMetricsResponse {
  dropped_metrics: DroppedMetric[];
}

export interface GetContainerMetricsResponse {
  metrics: ContainerMetric[];
}

export interface GetSensorReadingsResponse {
  data: SensorDataPoint[];
}

export interface Pagination {
  page: number;
  limit: number;
  total: number;
  pages: number;
}

export interface GetPackagesResponse {
  packages: Package[];
  pagination: Pagination;
}

export interface GetPackageStatsResponse extends PackageStats {
  last_collection: PackageCollection | null;
}

export interface GetPackageCollectionsResponse {
  collections: PackageCollection[];
  pagination: Pagination;
}

export interface GetPackageHistoryResponse {
  history: PackageHistory[];
  pagination: Pagination;
}

// Global package view — deduplicated across all hosts
export type GlobalPackageStatus =
  | "security"
  | "outdated"
  | "up_to_date"
  | "not_checked";

export interface GlobalPackage {
  name: string;
  package_manager: string;
  host_count: number;
  available_version: string;
  current_version: string;
  has_security_update: boolean;
  update_checked: boolean;
}

export interface ListGlobalPackagesResponse {
  packages: GlobalPackage[];
  pagination: Pagination;
  total_packages: number; // global unfiltered count
  outdated_count: number; // global unfiltered
  security_count: number; // global unfiltered
  outdated_hosts_count: number; // global unfiltered — hosts with ≥1 outdated/security package
  security_hosts_count: number; // global unfiltered — hosts with ≥1 security package
  available_managers: string[]; // global unfiltered, for the manager filter dropdown
}

export interface ListGlobalPackagesParams {
  q?: string;
  status?: GlobalPackageStatus[];
  manager?: string[];
  limit?: number;
  offset?: number;
  sort_by?: string;
  sort_order?: "asc" | "desc";
}

export interface CurrentUserResponse {
  user: User;
}

// ===== SMTP Settings =====

export type SmtpTLSMode = "none" | "starttls" | "tls";
export type SmtpAuthType = "plain" | "login";

export interface SmtpSettings {
  host: string;
  port: number;
  username: string;
  password_set: boolean; // true if a password is stored — plaintext is never returned
  from_address: string;
  from_name: string;
  tls_mode: SmtpTLSMode;
  auth_type: SmtpAuthType;
  helo_name: string;
  notification_email: string;
  enabled: boolean;
}

export interface UpdateSMTPSettingsRequest {
  host: string;
  port: number;
  username: string;
  password?: string; // omitted or empty = keep existing password
  from_address: string;
  from_name: string;
  tls_mode: SmtpTLSMode;
  auth_type: SmtpAuthType;
  helo_name: string;
  notification_email: string;
  enabled: boolean;
}

export interface GetSMTPSettingsResponse {
  smtp: SmtpSettings;
}

// ===== Alert Rules =====

export type AlertMetricType =
  | "host_down"
  | "cpu_usage"
  | "memory_usage"
  | "disk_usage"
  | "load_avg"
  | "load_avg_5"
  | "load_avg_15"
  | "temperature";

export const ALERT_METRIC_TYPES: AlertMetricType[] = [
  "host_down",
  "cpu_usage",
  "memory_usage",
  "disk_usage",
  "load_avg",
  "load_avg_5",
  "load_avg_15",
  "temperature",
];

export const ALERT_METRIC_LABELS: Record<AlertMetricType, string> = {
  host_down: "Host offline",
  cpu_usage: "CPU usage",
  memory_usage: "Memory usage",
  disk_usage: "Disk usage",
  load_avg: "Load avg (1m)",
  load_avg_5: "Load avg (5m)",
  load_avg_15: "Load avg (15m)",
  temperature: "CPU temperature",
};

export interface AlertRule {
  metric_type: AlertMetricType;
  enabled: boolean;
  threshold: number;
  duration_minutes: number;
  updated_at: string;
}

export interface EffectiveAlertRule {
  metric_type: AlertMetricType;
  enabled: boolean;
  threshold: number;
  duration_minutes: number;
  is_override: boolean;
}

export interface GetAlertRulesResponse {
  rules: AlertRule[];
}

export interface GetHostAlertRulesResponse {
  rules: EffectiveAlertRule[];
}

export interface ActiveIncident {
  id: string;
  host_id: string;
  host_name: string;
  metric_type: AlertMetricType;
  started_at: string;
  threshold_value: number;
  current_value: number;
}

export interface GetActiveIncidentsResponse {
  incidents: ActiveIncident[];
}

export interface GlobalIncident {
  id: string;
  host_id: string;
  host_name: string;
  metric_type: AlertMetricType;
  started_at: string;
  resolved_at: string | null;
  threshold_value: number;
  current_value: number;
}

export interface GetAllIncidentsResponse {
  incidents: GlobalIncident[];
  total_count: number;
  limit: number;
  offset: number;
}

export interface HostIncident {
  id: string;
  metric_type: AlertMetricType;
  started_at: string;
  resolved_at: string | null;
  threshold_value: number;
  current_value: number;
}

export type IncidentStatusFilter = "all" | "active" | "resolved";

export interface GetHostIncidentsResponse {
  incidents: HostIncident[];
  total_count: number;
  limit: number;
  offset: number;
}

// ===== Toast Notifications =====

export type ToastType = "info" | "success" | "warning" | "error";

export interface Toast {
  id: number;
  message: string;
  type: ToastType;
}

export interface ToastStore {
  subscribe: (fn: (toasts: Toast[]) => void) => () => void;
  add: (message: string, type?: ToastType, duration?: number) => number;
  remove: (id: number) => void;
  clear: () => void;
}

// ===== Component Props =====

export interface ChartProps {
  data: Metric[] | AggregatedMetric[];
}

export interface HostTableProps {
  hosts: HostWithMetrics[];
  metricsData: Record<string, Metric[]>;
}
