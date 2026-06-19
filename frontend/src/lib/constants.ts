// Metrics retention limits (number of data points kept in memory)
export const MAX_METRICS_POINTS_DASHBOARD = 50;
export const MAX_METRICS_POINTS_DETAIL = 120;
export const MAX_AGGREGATED_POINTS = 200;

// Pagination
export const HOSTS_PER_PAGE = 20;
export const PACKAGES_PER_PAGE = 25;
export const COLLECTIONS_PER_PAGE = 10;

// Polling intervals (ms)
export const DROPPED_METRICS_POLL_INTERVAL = 3_600_000; // 1 hour
export const AGENT_STATUS_POLL_INTERVAL = 5_000; // 5 seconds

// Debounce (ms)
export const SEARCH_DEBOUNCE_MS = 300;

// Toast durations (ms)
export const TOAST_DEFAULT_DURATION = 5_000;
export const TOAST_LONG_DURATION = 8_000;
