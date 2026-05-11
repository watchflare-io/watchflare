<script lang="ts">
    import { onMount, onDestroy, setContext } from "svelte";
    import { goto } from "$app/navigation";
    import { page } from "$app/stores";
    import * as api from "$lib/api.js";
    import { API_BASE_URL } from "$lib/api.js";
    import { SSEManager } from "$lib/sse/manager.js";
    import { handleSSEReactivation, formatOfflineDuration } from "$lib/utils";
    import type {
        Host,
        Metric,
        SSEEvent,
        HostUpdateEvent,
        MetricsUpdateEvent,
        Package,
        PackageStats,
        HostIncident,
        IncidentStatusFilter,
        TimeRange,
        ContainerMetric,
    } from "$lib/types";

    type SSECallback = (event: SSEEvent) => void;
    import { ChevronRight } from "lucide-svelte";
    import HostDetailHeader from "$lib/components/host/HostDetailHeader.svelte";
    import HostLiveStats from "$lib/components/host/HostLiveStats.svelte";
    import HostAlerts from "$lib/components/host/HostAlerts.svelte";
    import HostAlertRulesDrawer from "$lib/components/host/HostAlertRulesDrawer.svelte";
    import ConfirmDialog from "$lib/components/ConfirmDialog.svelte";
    import Modal from "$lib/components/Modal.svelte";
    import InstallInstructions from "$lib/components/InstallInstructions.svelte";

    const { children } = $props();

    const hostId = $derived($page.params.id);
    const currentPath = $derived($page.url.pathname);

    let host: Host | null = $state(null);
    let loading = $state(true);
    let error = $state("");
    let clockDesync = $state(false);
    let latestAgentVersion: string | null = $state(null);
    let latestMetric: Metric | null = $state(null);
    let hasActiveAlertRules = $state(false);
    let now = $state(Date.now());
    let nowTimer: ReturnType<typeof setInterval> | null = null;

    // Tab data caches — persist between tab switches for the duration of the host detail session
    let overviewCache: {
        metrics: Metric[];
        containerMetrics: ContainerMetric[];
        timeRange: TimeRange;
    } | null = $state(null);
    let packagesCache: {
        packages: Package[];
        totalCount: number;
        totalPages: number;
        stats: PackageStats | null;
        searchTerm: string;
        allManagerKeys: string[];
        selectedManagers: string[];
        selectedStatuses: string[];
        sortColumn: string;
        sortOrder: 'asc' | 'desc';
        offset: number;
        limit: number;
        visibleColumns: string[];
    } | null = $state(null);
    let incidentsCache: {
        incidents: HostIncident[];
        totalCount: number;
        offset: number;
        statusFilter: IncidentStatusFilter;
    } | null = $state(null);

    // Incremented each time a package_inventory_update SSE event arrives for this host
    let packageInventorySignal = $state(0);

    // Child pages can subscribe to per-host SSE events via context
    const ssePageCallbacks = new Set<SSECallback>();

    // Modals
    let showDeleteConfirm = $state(false);
    let showRegenerateConfirm = $state(false);
    let showChangeIP = $state(false);
    let showRename = $state(false);
    let showAlertRules = $state(false);
    let newHostName = $state("");
    let newIP = $state("");
    let regeneratedToken = $state("");
    let copiedToken = $state(false);
    let backendHost = $state("");

    setContext("hostDetail", {
        get host() {
            return host;
        },
        get loading() {
            return loading;
        },
        get latestMetric() {
            return latestMetric;
        },
        setLatestMetric: (m: Metric | null) => {
            latestMetric = m;
        },
        get overviewCache() {
            return overviewCache;
        },
        setOverviewCache: (data: typeof overviewCache) => {
            overviewCache = data;
        },
        get packagesCache() {
            return packagesCache;
        },
        setPackagesCache: (data: typeof packagesCache) => {
            packagesCache = data;
        },
        get packageInventorySignal() {
            return packageInventorySignal;
        },
        get incidentsCache() {
            return incidentsCache;
        },
        setIncidentsCache: (data: typeof incidentsCache) => {
            incidentsCache = data;
        },
        subscribeToSSE: (cb: SSECallback): (() => void) => {
            ssePageCallbacks.add(cb);
            return () => ssePageCallbacks.delete(cb);
        },
    });

    const showIPMismatchWarning = $derived(
        !!(
            host &&
            host.configured_ip &&
            host.ip_address_v4 &&
            host.configured_ip !== host.ip_address_v4 &&
            !host.ignore_ip_mismatch
        ),
    );

    function isActiveTab(tab: "overview" | "packages" | "incidents"): boolean {
        const base = `/hosts/${hostId}`;
        if (tab === "overview") return currentPath === base;
        return currentPath.startsWith(`${base}/${tab}`);
    }

    let hostSseManager: SSEManager | null = null;

    onMount(() => {
        hostSseManager = new SSEManager(`${API_BASE_URL}/hosts/${hostId}/events`);
        hostSseManager.onMessage(handleSSEMessage);
        hostSseManager.connect();
        loadHost();
        nowTimer = setInterval(() => { now = Date.now(); }, 30_000);
        const state = $page.state as { newHostToken?: string };
        if (state.newHostToken) {
            regeneratedToken = state.newHostToken;
            backendHost = window.location.host;
        }
    });

    onDestroy(() => {
        hostSseManager?.disconnect();
        hostSseManager = null;
        if (nowTimer) clearInterval(nowTimer);
        if (updateAgentMessageTimeout) clearTimeout(updateAgentMessageTimeout);
        if (copyErrorTimeout) clearTimeout(copyErrorTimeout);
    });

    function handleSSEMessage(event: SSEEvent) {
        handleSSEReactivation(event);
        ssePageCallbacks.forEach((cb) => cb(event));
        if (event.type === "host_update") {
            const update = event.data as HostUpdateEvent;
            if (host) {
                host = {
                    ...host,
                    status: update.status,
                    ip_address_v4: update.ip_address_v4,
                    ip_address_v6: update.ip_address_v6,
                    configured_ip: update.configured_ip,
                    ignore_ip_mismatch: update.ignore_ip_mismatch,
                    last_seen: update.last_seen,
                    agent_version: update.agent_version ?? host.agent_version,
                };
                clockDesync = update.clock_desync || false;
            }
        }
        if (event.type === "metrics_update") {
            if (host) {
                latestMetric = event.data as MetricsUpdateEvent;
            }
        }
        if (event.type === "package_inventory_update") {
            if (host) {
                packageInventorySignal++;
            }
        }
    }

    const METRIC_STALE_MS = 5 * 60 * 1000; // 5 minutes

    async function loadHost() {
        try {
            const [response] = await Promise.all([
                api.getHost(hostId),
                latestAgentVersion === null
                    ? api
                          .getLatestAgentVersion()
                          .then((r) => {
                              latestAgentVersion = r.latest_version || null;
                          })
                          .catch(() => {})
                    : Promise.resolve(),
            ]);
            host = response.host;
            clockDesync = response.clock_desync || false;
            if (response.latest_metrics) {
                const age = Date.now() - new Date(response.latest_metrics.timestamp).getTime();
                if (age <= METRIC_STALE_MS) {
                    latestMetric = response.latest_metrics;
                }
            }
        } catch (err) {
            error = err instanceof Error ? err.message : "Failed to load host";
        } finally {
            loading = false;
        }
        // Load alert rules for bell indicator (non-critical)
        try {
            const rulesData = await api.getHostAlertRules(hostId);
            hasActiveAlertRules = rulesData.rules.some((r) => r.enabled);
        } catch {
            // non-critical
        }
    }

    async function handleDelete() {
        try {
            await api.deleteHost(hostId);
            goto("/hosts");
        } catch (err) {
            error =
                err instanceof Error ? err.message : "Failed to delete host";
            showDeleteConfirm = false;
        }
    }

    async function handleRegenerateToken() {
        try {
            const response = await api.regenerateToken(hostId);
            regeneratedToken = response.token;
            backendHost = window.location.host;
            showRegenerateConfirm = false;
            await loadHost();
        } catch (err) {
            error =
                err instanceof Error
                    ? err.message
                    : "Failed to regenerate token";
            showRegenerateConfirm = false;
        }
    }

    async function handleRename() {
        try {
            await api.renameHost(hostId, newHostName);
            showRename = false;
            newHostName = "";
            await loadHost();
        } catch (err) {
            error =
                err instanceof Error ? err.message : "Failed to rename host";
        }
    }

    async function handleChangeIP() {
        try {
            await api.updateConfiguredIP(hostId, newIP);
            showChangeIP = false;
            newIP = "";
            await loadHost();
        } catch (err) {
            error = err instanceof Error ? err.message : "Failed to update IP";
        }
    }

    async function handleUpdateIP() {
        if (!host) return;
        try {
            await api.updateConfiguredIP(host.id, host.ip_address_v4);
            await loadHost();
        } catch (err) {
            error = err instanceof Error ? err.message : "Failed to update IP";
        }
    }

    async function handleIgnoreIP() {
        if (!host) return;
        try {
            await api.ignoreIPMismatch(host.id);
            await loadHost();
        } catch (err) {
            error =
                err instanceof Error
                    ? err.message
                    : "Failed to ignore IP mismatch";
        }
    }

    async function handleDismissReactivation() {
        if (!host) return;
        try {
            await api.dismissReactivation(host.id);
            await loadHost();
        } catch (err) {
            error =
                err instanceof Error
                    ? err.message
                    : "Failed to dismiss reactivation";
        }
    }

    async function handlePause() {
        if (!host) return;
        const previousStatus = host.status;
        try {
            await api.pauseHost(host.id);
            host = { ...host, status: "paused" };
        } catch (err) {
            host = { ...host, status: previousStatus };
            error = err instanceof Error ? err.message : "Failed to pause host";
        }
    }

    async function handleResume() {
        if (!host) return;
        const previousStatus = host.status;
        try {
            await api.resumeHost(host.id);
            host = { ...host, status: "online" };
        } catch (err) {
            host = { ...host, status: previousStatus };
            error =
                err instanceof Error ? err.message : "Failed to resume host";
        }
    }

    let updateAgentMessage = $state("");
    let updateAgentMessageTimeout: ReturnType<typeof setTimeout> | null = null;

    async function handleUpdateAgent() {
        if (!host) return;
        updateAgentMessage = "";
        try {
            await api.triggerAgentUpdate(host.id);
            updateAgentMessage = "Update requested";
        } catch (err: unknown) {
            updateAgentMessage =
                err instanceof Error ? err.message : "Failed to request update";
        }
        if (updateAgentMessageTimeout) clearTimeout(updateAgentMessageTimeout);
        updateAgentMessageTimeout = setTimeout(() => {
            updateAgentMessage = "";
        }, 4000);
    }

    let copyErrorTimeout: ReturnType<typeof setTimeout> | null = null;
    let copyError = $state(false);

    async function handleCopy(text: string) {
        try {
            await navigator.clipboard.writeText(text);
        } catch {
            copyError = true;
            if (copyErrorTimeout) clearTimeout(copyErrorTimeout);
            copyErrorTimeout = setTimeout(() => (copyError = false), 2000);
        }
    }

    function closeChangeIPModal() {
        showChangeIP = false;
        newIP = "";
    }
</script>

<svelte:head>
    <title>{host?.display_name || "Host"} - Watchflare</title>
</svelte:head>

{#if loading}
    <!-- Skeleton: breadcrumb -->
    <div class="flex items-center gap-1 mb-3 animate-pulse">
        <div class="h-3.5 w-10 rounded bg-muted"></div>
        <div class="h-3.5 w-3.5 rounded bg-muted"></div>
        <div class="h-3.5 w-24 rounded bg-muted"></div>
    </div>
    <!-- Skeleton: HostDetailHeader -->
    <div class="mb-4 rounded-xl border bg-card p-3 md:p-4 animate-pulse">
        <div class="flex items-start justify-between mb-3">
            <div class="flex items-center gap-3">
                <div class="h-7 w-48 rounded bg-muted"></div>
                <div class="h-5 w-16 rounded-full bg-muted"></div>
            </div>
            <div class="h-8 w-8 rounded-lg bg-muted"></div>
        </div>
        <div class="flex flex-wrap gap-2">
            {#each Array(4) as _}
                <div class="h-6 w-28 rounded-full bg-muted"></div>
            {/each}
        </div>
    </div>
    <!-- Skeleton: live metric pills -->
    <div class="flex gap-3 mb-6 animate-pulse overflow-x-auto">
        {#each Array(5) as _}
            <div
                class="shrink-0 rounded-lg border bg-card px-3 py-2 flex items-center gap-2"
            >
                <div class="h-3.5 w-3.5 rounded bg-muted"></div>
                <div class="h-3 w-10 rounded bg-muted"></div>
                <div class="h-5 w-12 rounded bg-muted"></div>
            </div>
        {/each}
    </div>
    <!-- Tabs (static, no skeleton needed) -->
    <div
        class="mb-6 flex gap-1 border-b overflow-x-auto overflow-y-clip no-scrollbar"
    >
        {#each [["overview", "Overview", `/hosts/${hostId}`], ["packages", "Packages", `/hosts/${hostId}/packages`], ["incidents", "Incidents", `/hosts/${hostId}/incidents`]] as [tab, label, href]}
            <a
                {href}
                class="shrink-0 px-4 py-2.5 text-sm font-medium transition-colors border-b-2 -mb-px {isActiveTab(
                    tab as 'overview' | 'packages' | 'incidents',
                )
                    ? 'border-primary text-foreground'
                    : 'border-transparent text-muted-foreground hover:text-foreground'}"
            >
                {label}
            </a>
        {/each}
    </div>
{:else if error}
    <div class="rounded-lg border border-destructive bg-destructive/10 p-4">
        <p class="text-sm text-destructive">{error}</p>
    </div>
{:else if host}
    <!-- Breadcrumb -->
    <nav aria-label="Breadcrumb" class="flex items-center gap-1 mb-3 text-sm">
        <a href="/hosts" class="text-muted-foreground hover:text-foreground transition-colors">Hosts</a>
        <ChevronRight class="h-3.5 w-3.5 text-muted-foreground/60 shrink-0" />
        <span class="text-foreground font-medium truncate">{host.display_name}</span>
    </nav>
    <HostDetailHeader
        {host}
        metric={latestMetric}
        {latestAgentVersion}
        {hasActiveAlertRules}
        onDelete={() => (showDeleteConfirm = true)}
        onRegenerateToken={() => (showRegenerateConfirm = true)}
        onChangeIP={() => (showChangeIP = true)}
        onRename={() => {
            newHostName = host?.display_name || "";
            showRename = true;
        }}
        onPause={handlePause}
        onResume={handleResume}
        onAlertRules={() => {
            showAlertRules = true;
        }}
        onUpdateAgent={handleUpdateAgent}
    />
    {#if updateAgentMessage}
        <p class="mb-3 text-xs text-muted-foreground">{updateAgentMessage}</p>
    {/if}

    {#if regeneratedToken}
        <div
            class="mb-6 rounded-lg border border-warning bg-warning/10 p-4 space-y-3"
        >
            <div class="flex items-center justify-between gap-4 flex-wrap">
                <p class="text-sm font-medium text-warning">
                    This token is valid for 24 hours and will not be displayed
                    again. Make sure to copy it or use it now.
                </p>
                <div class="flex items-center gap-2 shrink-0">
                    <button
                        onclick={() => {
                            handleCopy(regeneratedToken);
                            copiedToken = true;
                            setTimeout(() => (copiedToken = false), 2000);
                        }}
                        disabled={copiedToken}
                        class="rounded-lg border bg-background px-3 py-1.5 text-sm font-medium transition-colors hover:bg-muted disabled:opacity-60 {copyError
                            ? 'text-destructive border-destructive/40'
                            : 'text-foreground'}"
                    >
                        {copiedToken
                            ? "Copied!"
                            : copyError
                              ? "Copy failed"
                              : "Copy Token"}
                    </button>
                    <button
                        onclick={() => {
                            regeneratedToken = "";
                        }}
                        class="rounded-lg border bg-background px-3 py-1.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                    >
                        Dismiss
                    </button>
                </div>
            </div>
            <!-- svelte-ignore a11y_click_events_have_key_events -->
            <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
            <input
                readonly
                value={regeneratedToken}
                onclick={(e) => (e.currentTarget as HTMLInputElement).select()}
                class="w-full font-mono text-xs bg-background border rounded-lg px-3 py-2 text-foreground select-all cursor-text focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
            />
        </div>
        <InstallInstructions {host} token={regeneratedToken} {backendHost} />
    {/if}

    <HostAlerts
        {host}
        {showIPMismatchWarning}
        {clockDesync}
        onUpdateIP={handleUpdateIP}
        onIgnoreIP={handleIgnoreIP}
        onDismissReactivation={handleDismissReactivation}
    />

    <HostLiveStats metric={latestMetric} />
    {#if host.status !== "online" && host.last_seen}
        <p class="text-xs text-muted-foreground mb-6">
            {formatOfflineDuration(host.last_seen, now)}
        </p>
    {/if}

    <!-- Tab Navigation -->
    <div
        class="mb-6 flex gap-1 border-b overflow-x-auto overflow-y-clip no-scrollbar"
    >
        <a
            href="/hosts/{hostId}"
            class="shrink-0 px-4 py-2.5 text-sm font-medium transition-colors border-b-2 -mb-px {isActiveTab(
                'overview',
            )
                ? 'border-primary text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground'}"
        >
            Overview
        </a>
        <a
            href="/hosts/{hostId}/packages"
            class="shrink-0 px-4 py-2.5 text-sm font-medium transition-colors border-b-2 -mb-px {isActiveTab(
                'packages',
            )
                ? 'border-primary text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground'}"
        >
            Packages
        </a>
        <a
            href="/hosts/{hostId}/incidents"
            class="shrink-0 px-4 py-2.5 text-sm font-medium transition-colors border-b-2 -mb-px {isActiveTab(
                'incidents',
            )
                ? 'border-primary text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground'}"
        >
            Incidents
        </a>
    </div>

    {@render children()}

    <HostAlertRulesDrawer
        {hostId}
        open={showAlertRules}
        onClose={() => {
            showAlertRules = false;
        }}
        onSave={(hasActive) => {
            hasActiveAlertRules = hasActive;
        }}
    />
{/if}

<!-- Modals -->
<ConfirmDialog
    open={showDeleteConfirm}
    title="Confirm Delete"
    onConfirm={handleDelete}
    onClose={() => (showDeleteConfirm = false)}
    confirmLabel="Delete Host"
    confirmVariant="destructive"
>
    <p class="text-sm text-muted-foreground mb-4">
        Are you sure you want to delete "{host?.display_name}"?
    </p>
    <p class="text-sm font-medium text-destructive">
        This action cannot be undone.
    </p>
</ConfirmDialog>

<ConfirmDialog
    open={showRegenerateConfirm}
    title="Regenerate Token"
    onConfirm={handleRegenerateToken}
    onClose={() => (showRegenerateConfirm = false)}
    confirmLabel="Regenerate"
>
    <p class="text-sm text-muted-foreground">
        This will generate a new registration token and set the host to pending
        until the agent re-registers. Use the new token to run <code
            class="font-mono">watchflare-agent register</code
        >
        on the host.
    </p>
</ConfirmDialog>

<Modal
    open={showRename}
    onClose={() => {
        showRename = false;
        newHostName = "";
    }}
>
    <h3 class="text-lg font-semibold text-foreground mb-3">Rename Host</h3>
    <div class="mb-4">
        <label
            for="newname"
            class="block text-sm font-medium text-foreground mb-2"
            >New Name</label
        >
        <input
            id="newname"
            type="text"
            bind:value={newHostName}
            placeholder="e.g., production-web-01"
            class="w-full rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
        />
    </div>
    <div class="flex gap-3 justify-end">
        <button
            onclick={() => {
                showRename = false;
                newHostName = "";
            }}
            class="rounded-lg border bg-background px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-muted"
        >
            Cancel
        </button>
        <button
            onclick={handleRename}
            disabled={newHostName.length < 2}
            class="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
        >
            Rename
        </button>
    </div>
</Modal>

<Modal open={showChangeIP} onClose={closeChangeIPModal}>
    <h3 class="text-lg font-semibold text-foreground mb-3">
        Change Configured IP
    </h3>
    <div class="mb-4">
        <label
            for="newip"
            class="block text-sm font-medium text-foreground mb-2"
            >New IP Address</label
        >
        <input
            id="newip"
            type="text"
            bind:value={newIP}
            placeholder="e.g., 192.168.1.100"
            class="w-full rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
        />
    </div>
    <div class="flex gap-3 justify-end">
        <button
            onclick={closeChangeIPModal}
            class="rounded-lg border bg-background px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-muted"
        >
            Cancel
        </button>
        <button
            onclick={handleChangeIP}
            disabled={!newIP.trim()}
            class="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
        >
            Update IP
        </button>
    </div>
</Modal>
