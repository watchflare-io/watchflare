import type {
  LoginResponse,
  TOTPSetupResponse,
  TOTPEnableResponse,
  TOTPRegenerateResponse,
  RegisterResponse,
  CreateHostResponse,
  RegenerateTokenResponse,
  GetHostResponse,
  ListHostsResponse,
  HostStatsResponse,
  GetMetricsResponse,
  GetAggregatedMetricsResponse,
  GetDroppedMetricsResponse,
  GetContainerMetricsResponse,
  GetSensorReadingsResponse,
  GetPackagesResponse,
  GetPackageStatsResponse,
  GetPackageCollectionsResponse,
  GetPackageHistoryResponse,
  ListGlobalPackagesResponse,
  ListGlobalPackagesParams,
  CurrentUserResponse,
  MetricsQueryParams,
  User,
  GetSMTPSettingsResponse,
  UpdateSMTPSettingsRequest,
  GetAlertRulesResponse,
  GetHostAlertRulesResponse,
  GetActiveIncidentsResponse,
  GetAllIncidentsResponse,
  GetHostIncidentsResponse,
  IncidentStatusFilter,
  AlertMetricType,
  HostStatus,
  ListNotificationChannelsResponse,
  NotificationChannelResponse,
  CreateNotificationChannelInput,
  UpdateNotificationChannelInput,
} from "./types";
import { capitalizeFirst } from "./utils";

export const API_BASE_URL = import.meta.env.VITE_API_URL || "/api/v1";

// Build query string from params, filtering out undefined/null/empty values
export function buildQueryString(
  params: Record<string, string | number | boolean | undefined | null>,
): string {
  const qs = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") {
      qs.append(key, String(value));
    }
  }
  const str = qs.toString();
  return str ? "?" + str : "";
}

interface ApiRequestOptions extends RequestInit {
  headers?: Record<string, string>;
}

// Custom API error class
export class ApiError extends Error {
  constructor(
    public status: number,
    public statusText: string,
    public data?: { error?: string; message?: string },
  ) {
    super(
      capitalizeFirst(
        data?.error || data?.message || statusText || "API request failed",
      ),
    );
    this.name = "ApiError";
  }

  get isAuthError(): boolean {
    return this.status === 401;
  }

  get isForbidden(): boolean {
    return this.status === 403;
  }

  get isNotFound(): boolean {
    return this.status === 404;
  }

  get isServerError(): boolean {
    return this.status >= 500;
  }
}

// Check if initial setup is required (no users exist)
export async function getAppConfig(): Promise<{ cookie_secure: boolean }> {
  try {
    const response = await fetch(`${API_BASE_URL}/config`, {
      credentials: "include",
    });
    return await response.json();
  } catch {
    return { cookie_secure: true }; // assume secure on error (no false alarm)
  }
}

export async function checkSetupRequired(): Promise<boolean> {
  try {
    const response = await fetch(`${API_BASE_URL}/auth/setup-required`, {
      credentials: "include",
    });
    const data = await response.json();
    return data.setup_required;
  } catch (err) {
    if (import.meta.env.DEV)
      console.error("Failed to check setup status:", err);
    return false;
  }
}

// Handle authentication errors
async function handleAuthError(): Promise<never> {
  try {
    const setupRequired = await checkSetupRequired();
    if (setupRequired) {
      // No users exist, redirect to registration
      window.location.href = "/register";
      throw new ApiError(401, "Unauthorized", {
        message: "Redirecting to registration",
      });
    } else {
      // Users exist but not authenticated, redirect to login
      window.location.href = "/login";
      throw new ApiError(401, "Unauthorized", {
        message: "Redirecting to login",
      });
    }
  } catch (err) {
    // If checking setup status fails, default to login
    window.location.href = "/login";
    throw new ApiError(401, "Unauthorized", {
      message: "Redirecting to login",
    });
  }
}

// Make API request with credentials (cookies sent automatically)
async function apiRequest<T>(
  endpoint: string,
  options: ApiRequestOptions = {},
  skipAuthRedirect = false,
): Promise<T> {
  const headers = {
    "Content-Type": "application/json",
    ...options.headers,
  };

  let response: Response;
  let data: unknown;

  try {
    response = await fetch(`${API_BASE_URL}${endpoint}`, {
      ...options,
      headers,
      credentials: "include", // Important: send cookies with requests
    });

    // Try to parse JSON response
    try {
      data = await response.json();
    } catch (parseError) {
      // If JSON parsing fails, create error with status text
      if (!response.ok) {
        throw new ApiError(response.status, response.statusText);
      }
      throw new ApiError(500, "Invalid response format");
    }
  } catch (err) {
    // Network or fetch errors
    if (err instanceof ApiError) {
      throw err;
    }
    // Network error (e.g., no internet, CORS, etc.)
    throw new ApiError(0, "Network error", {
      message:
        err instanceof Error ? err.message : "Failed to connect to server",
    });
  }

  if (!response.ok) {
    // Handle authentication errors (skip for auth endpoints like login/register)
    if (response.status === 401 && !skipAuthRedirect) {
      await handleAuthError();
    }

    // Throw API error for other cases
    throw new ApiError(
      response.status,
      response.statusText,
      data as { error?: string; message?: string },
    );
  }

  return data as T;
}

// Auth API calls
export async function register(
  email: string,
  password: string,
  username?: string,
): Promise<RegisterResponse> {
  return apiRequest<RegisterResponse>(
    "/auth/register",
    {
      method: "POST",
      body: JSON.stringify({ email, password, username: username || "" }),
    },
    true,
  );
}

export async function login(
  email: string,
  password: string,
): Promise<LoginResponse> {
  return apiRequest<LoginResponse>(
    "/auth/login",
    {
      method: "POST",
      body: JSON.stringify({ email, password }),
    },
    true,
  );
}

export async function logout(): Promise<{ message: string }> {
  return apiRequest<{ message: string }>("/auth/logout", {
    method: "POST",
  });
}

export async function verifyTOTP(totpCode?: string, backupCode?: string): Promise<{ message: string }> {
  const body: Record<string, string> = {};
  if (totpCode) body.totp_code = totpCode;
  if (backupCode) body.backup_code = backupCode;
  return apiRequest<{ message: string }>(
    '/auth/verify-totp',
    { method: 'POST', body: JSON.stringify(body) },
    true,
  );
}

export async function setupTOTP(): Promise<TOTPSetupResponse> {
  return apiRequest<TOTPSetupResponse>('/2fa/setup', { method: 'POST' });
}

export async function enableTOTP(code: string): Promise<TOTPEnableResponse> {
  return apiRequest<TOTPEnableResponse>(
    '/2fa/enable',
    { method: 'POST', body: JSON.stringify({ code }) },
  );
}

export async function disableTOTP(totpCode?: string, backupCode?: string): Promise<{ message: string }> {
  const body: Record<string, string> = {};
  if (totpCode) body.totp_code = totpCode;
  if (backupCode) body.backup_code = backupCode;
  return apiRequest<{ message: string }>(
    '/2fa',
    { method: 'DELETE', body: JSON.stringify(body) },
  );
}

export async function regenerateBackupCodes(code: string): Promise<TOTPRegenerateResponse> {
  return apiRequest<TOTPRegenerateResponse>(
    '/2fa/backup-codes/regenerate',
    { method: 'POST', body: JSON.stringify({ code }) },
  );
}

export async function changePassword(
  currentPassword: string,
  newPassword: string,
): Promise<{ message: string }> {
  return apiRequest<{ message: string }>("/auth/change-password", {
    method: "PUT",
    body: JSON.stringify({
      current_password: currentPassword,
      new_password: newPassword,
    }),
  });
}

export async function changeEmail(
  newEmail: string,
): Promise<{ message: string }> {
  return apiRequest<{ message: string }>("/auth/change-email", {
    method: "PUT",
    body: JSON.stringify({
      new_email: newEmail,
    }),
  });
}

export async function changeUsername(
  username: string,
): Promise<{ message: string; user: User }> {
  return apiRequest<{ message: string; user: User }>("/auth/change-username", {
    method: "PUT",
    body: JSON.stringify({ username }),
  });
}

// Host API calls
export async function listHosts(params?: {
  page?: number;
  perPage?: number;
  status?: HostStatus;
  search?: string;
  signal?: AbortSignal;
}): Promise<ListHostsResponse> {
  const query = buildQueryString({
    page: params?.page,
    per_page: params?.perPage,
    status: params?.status,
    search: params?.search,
  });
  return apiRequest<ListHostsResponse>(`/hosts${query}`, {
    signal: params?.signal,
  });
}

export async function getHostStats(): Promise<HostStatsResponse> {
  return apiRequest<HostStatsResponse>('/hosts/stats');
}

export async function getHost(id: string): Promise<GetHostResponse> {
  return apiRequest<GetHostResponse>(`/hosts/${id}`);
}

export async function createHost(
  displayName: string,
  configuredIP?: string,
  allowAnyIP?: boolean,
): Promise<CreateHostResponse> {
  return apiRequest<CreateHostResponse>("/hosts", {
    method: "POST",
    body: JSON.stringify({
      display_name: displayName,
      configured_ip: configuredIP,
      allow_any_ip: allowAnyIP,
    }),
  });
}

export async function pauseHost(id: string): Promise<{ message: string }> {
  return apiRequest<{ message: string }>(`/hosts/${id}/pause`, {
    method: "PUT",
  });
}

export async function resumeHost(id: string): Promise<{ message: string }> {
  return apiRequest<{ message: string }>(`/hosts/${id}/resume`, {
    method: "PUT",
  });
}

export async function deleteHost(id: string): Promise<{ message: string }> {
  return apiRequest<{ message: string }>(`/hosts/${id}`, {
    method: "DELETE",
  });
}

export async function regenerateToken(
  id: string,
): Promise<RegenerateTokenResponse> {
  return apiRequest<RegenerateTokenResponse>(`/hosts/${id}/regenerate-token`, {
    method: "POST",
  });
}

export async function validateIP(
  id: string,
  selectedIP: string,
): Promise<{ message: string }> {
  return apiRequest<{ message: string }>(`/hosts/${id}/validate-ip`, {
    method: "PUT",
    body: JSON.stringify({ selected_ip: selectedIP }),
  });
}

export async function renameHost(
  id: string,
  newName: string,
): Promise<{ message: string }> {
  const trimmed = newName.trim();
  if (trimmed.length < 2) {
    throw new Error("Host name must be at least 2 characters");
  }
  if (trimmed.length > 255) {
    throw new Error("Host name must not exceed 255 characters");
  }
  return apiRequest<{ message: string }>(`/hosts/${id}/rename`, {
    method: "PUT",
    body: JSON.stringify({ new_name: trimmed }),
  });
}

export async function updateConfiguredIP(
  id: string,
  newIP: string,
): Promise<{ message: string }> {
  return apiRequest<{ message: string }>(`/hosts/${id}/change-ip`, {
    method: "PUT",
    body: JSON.stringify({ new_ip: newIP }),
  });
}

export async function ignoreIPMismatch(
  id: string,
): Promise<{ message: string }> {
  return apiRequest<{ message: string }>(`/hosts/${id}/ignore-ip-mismatch`, {
    method: "PUT",
  });
}

export async function dismissReactivation(
  id: string,
): Promise<{ message: string }> {
  return apiRequest<{ message: string }>(`/hosts/${id}/dismiss-reactivation`, {
    method: "PUT",
  });
}

// User preferences API calls
export async function getCurrentUser(): Promise<CurrentUserResponse> {
  return apiRequest<CurrentUserResponse>("/auth/user");
}

export interface UpdatePreferencesPayload {
  default_time_range?: string;
  theme?: string;
  time_format?: string;
  temperature_unit?: string;
  network_unit?: string;
  disk_unit?: string;
  gauge_warning_threshold?: number;
  gauge_critical_threshold?: number;
}

export async function updatePreferences(
  payload: UpdatePreferencesPayload,
): Promise<{ message: string; user: User }> {
  return apiRequest<{ message: string; user: User }>("/auth/preferences", {
    method: "PUT",
    body: JSON.stringify(payload),
  });
}

// Metrics API calls
export async function getHostMetrics(
  hostId: string,
  params: MetricsQueryParams = {},
): Promise<GetMetricsResponse> {
  const query = buildQueryString({
    time_range: params.time_range,
    limit: params.limit,
    offset: params.offset,
  });
  return apiRequest<GetMetricsResponse>(`/hosts/${hostId}/metrics${query}`);
}

// Get dropped metrics summary for the last 24 hours
export async function getDroppedMetrics(): Promise<GetDroppedMetricsResponse> {
  return apiRequest<GetDroppedMetricsResponse>("/hosts/dropped-metrics");
}

// Get per-sensor temperature readings for a specific host
export async function getSensorReadings(
  hostId: string,
  timeRange?: string,
): Promise<GetSensorReadingsResponse> {
  const query = buildQueryString({ time_range: timeRange });
  return apiRequest<GetSensorReadingsResponse>(
    `/hosts/${hostId}/sensor-readings${query}`,
  );
}

// Get container metrics for a specific host
export async function getContainerMetrics(
  hostId: string,
  timeRange?: string,
): Promise<GetContainerMetricsResponse> {
  const query = buildQueryString({ time_range: timeRange });
  return apiRequest<GetContainerMetricsResponse>(
    `/hosts/${hostId}/container-metrics${query}`,
  );
}

// Get aggregated metrics from all online hosts
export async function getAggregatedMetrics(
  timeRange?: string,
): Promise<GetAggregatedMetricsResponse> {
  const query = buildQueryString({ time_range: timeRange });
  return apiRequest<GetAggregatedMetricsResponse>(
    `/hosts/metrics/aggregated${query}`,
  );
}

// Package API calls
interface PackageQueryParams {
  limit?: number;
  offset?: number;
  q?: string;
  manager?: string[];
  status?: string[];
  sort_by?: string;
  sort_order?: "asc" | "desc";
}

export async function getHostPackages(
  hostId: string,
  params: PackageQueryParams = {},
): Promise<GetPackagesResponse> {
  const qs = new URLSearchParams();
  if (params.q) qs.set("q", params.q);
  if (params.limit !== undefined) qs.set("limit", String(params.limit));
  if (params.offset !== undefined) qs.set("offset", String(params.offset));
  if (params.sort_by) qs.set("sort_by", params.sort_by);
  if (params.sort_order) qs.set("sort_order", params.sort_order);
  for (const m of params.manager ?? []) qs.append("manager", m);
  for (const s of params.status ?? []) qs.append("status", s);
  const query = qs.toString() ? "?" + qs.toString() : "";
  return apiRequest<GetPackagesResponse>(`/hosts/${hostId}/packages${query}`);
}

export async function getPackageStats(
  hostId: string,
): Promise<GetPackageStatsResponse> {
  return apiRequest<GetPackageStatsResponse>(`/hosts/${hostId}/packages/stats`);
}

interface CollectionQueryParams {
  limit?: number;
  offset?: number;
}

export async function getPackageCollections(
  hostId: string,
  params: CollectionQueryParams = {},
): Promise<GetPackageCollectionsResponse> {
  const query = buildQueryString({
    limit: params.limit,
    offset: params.offset,
  });
  return apiRequest<GetPackageCollectionsResponse>(
    `/hosts/${hostId}/packages/collections${query}`,
  );
}

interface HistoryQueryParams extends CollectionQueryParams {
  exclude_initial?: boolean;
}

export async function getPackageHistory(
  hostId: string,
  params: HistoryQueryParams = {},
): Promise<GetPackageHistoryResponse> {
  const query = buildQueryString({
    limit: params.limit,
    offset: params.offset,
    exclude_initial: params.exclude_initial,
  });
  return apiRequest<GetPackageHistoryResponse>(
    `/hosts/${hostId}/packages/history${query}`,
  );
}

export async function listAllPackages(
  params: ListGlobalPackagesParams = {},
): Promise<ListGlobalPackagesResponse> {
  const qs = new URLSearchParams();
  if (params.q) qs.set("q", params.q);
  if (params.limit !== undefined) qs.set("limit", String(params.limit));
  if (params.offset !== undefined) qs.set("offset", String(params.offset));
  if (params.sort_by) qs.set("sort_by", params.sort_by);
  if (params.sort_order) qs.set("sort_order", params.sort_order);
  for (const s of params.status ?? []) qs.append("status", s);
  for (const m of params.manager ?? []) qs.append("manager", m);
  const query = qs.toString() ? "?" + qs.toString() : "";
  return apiRequest<ListGlobalPackagesResponse>(`/packages${query}`);
}

export async function getLatestAgentVersion(): Promise<{
  latest_version: string;
}> {
  return apiRequest<{ latest_version: string }>("/agent/latest-version");
}

export async function triggerPackageCollect(
  hostId: string,
): Promise<{ message: string; command_id: string }> {
  return apiRequest<{ message: string; command_id: string }>(
    `/hosts/${hostId}/packages/collect`,
    { method: "POST" },
  );
}

export async function triggerAgentUpdate(
  hostId: string,
): Promise<{ message: string; command_id: string }> {
  return apiRequest<{ message: string; command_id: string }>(
    `/hosts/${hostId}/agent/update`,
    { method: "POST" },
  );
}

// Settings API calls
export async function getSmtpSettings(): Promise<GetSMTPSettingsResponse> {
  return apiRequest<GetSMTPSettingsResponse>("/settings/smtp");
}

export async function updateSmtpSettings(
  data: UpdateSMTPSettingsRequest,
): Promise<{ message: string }> {
  return apiRequest<{ message: string }>("/settings/smtp", {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

export async function testSmtpConnection(
  recipient?: string,
): Promise<{ message: string }> {
  return apiRequest<{ message: string }>("/settings/smtp/test", {
    method: "POST",
    body: JSON.stringify({ recipient: recipient ?? "" }),
  });
}

// Alert Rules API calls
export async function getAlertRules(): Promise<GetAlertRulesResponse> {
  return apiRequest<GetAlertRulesResponse>("/settings/alerts");
}

export async function updateAlertRules(
  rules: Array<{
    metric_type: AlertMetricType;
    enabled: boolean;
    threshold: number;
    duration_minutes: number;
  }>,
): Promise<{ message: string }> {
  return apiRequest<{ message: string }>("/settings/alerts", {
    method: "PUT",
    body: JSON.stringify({ rules }),
  });
}

export async function getHostAlertRules(
  hostId: string,
): Promise<GetHostAlertRulesResponse> {
  return apiRequest<GetHostAlertRulesResponse>(`/hosts/${hostId}/alerts`);
}

export async function upsertHostAlertRule(
  hostId: string,
  metricType: AlertMetricType,
  data: { enabled: boolean; threshold: number; duration_minutes: number },
): Promise<{ message: string }> {
  return apiRequest<{ message: string }>(
    `/hosts/${hostId}/alerts/${metricType}`,
    {
      method: "PUT",
      body: JSON.stringify(data),
    },
  );
}

export async function deleteHostAlertRule(
  hostId: string,
  metricType: AlertMetricType,
): Promise<{ message: string }> {
  return apiRequest<{ message: string }>(
    `/hosts/${hostId}/alerts/${metricType}`,
    {
      method: "DELETE",
    },
  );
}

export async function getActiveIncidents(): Promise<GetActiveIncidentsResponse> {
  return apiRequest<GetActiveIncidentsResponse>("/settings/alerts/active");
}

export async function getAllIncidents(
  params: {
    status?: IncidentStatusFilter;
    limit?: number;
    offset?: number;
  } = {},
): Promise<GetAllIncidentsResponse> {
  const qs = buildQueryString(params);
  return apiRequest<GetAllIncidentsResponse>(`/settings/alerts/incidents${qs}`);
}

export async function getHostIncidents(
  hostId: string,
  params: {
    status?: IncidentStatusFilter;
    limit?: number;
    offset?: number;
  } = {},
): Promise<GetHostIncidentsResponse> {
  const qs = buildQueryString(params);
  return apiRequest<GetHostIncidentsResponse>(
    `/hosts/${hostId}/incidents${qs}`,
  );
}

// Notification channels (Shoutrrr-backed)
export async function listNotificationChannels(): Promise<ListNotificationChannelsResponse> {
  return apiRequest<ListNotificationChannelsResponse>('/notifications/channels');
}

export async function createNotificationChannel(
  input: CreateNotificationChannelInput,
): Promise<NotificationChannelResponse> {
  return apiRequest<NotificationChannelResponse>('/notifications/channels', {
    method: 'POST',
    body: JSON.stringify(input),
  });
}

export async function updateNotificationChannel(
  id: string,
  input: UpdateNotificationChannelInput,
): Promise<void> {
  return apiRequest<void>(`/notifications/channels/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  });
}

export async function deleteNotificationChannel(id: string): Promise<void> {
  return apiRequest<void>(`/notifications/channels/${id}`, { method: 'DELETE' });
}

export async function testNotificationChannel(id: string): Promise<{ message: string }> {
  return apiRequest<{ message: string }>(`/notifications/channels/${id}/test`, {
    method: 'POST',
  });
}

export async function testNotificationChannelDraft(
  url: string,
): Promise<{ message: string }> {
  return apiRequest<{ message: string }>('/notifications/channels/test', {
    method: 'POST',
    body: JSON.stringify({ url }),
  });
}
