<script lang="ts">
    import { onMount } from "svelte";
    import { APP_VERSION } from "$lib/version";
    import { userStore } from "$lib/stores/user";
    import { TIME_RANGES } from "$lib/utils";
    import { Sun, Moon, Monitor, Check, ShieldAlert } from "lucide-svelte";
    import type {
        Theme,
        TimeRange,
        TimeFormat,
        TemperatureUnit,
        NetworkUnit,
        DiskUnit,
    } from "$lib/types";
    import * as Select from "$lib/components/ui/select";
    import { getAppConfig } from "$lib/api";

    let cookieSecure = $state(true);

    onMount(async () => {
        const appConfig = await getAppConfig();
        cookieSecure = appConfig.cookie_secure;
    });

    const githubUrl = "https://github.com/watchflare-io/watchflare";
    const releaseUrl = `${githubUrl}/releases/tag/v${APP_VERSION}`;

    // Local state — mirrors user preferences, editable before save
    const user = $derived($userStore.user);

    let selectedTimeRange = $state<TimeRange>("1h");
    let selectedTheme = $state<Theme>("system");
    let selectedTimeFormat = $state<TimeFormat>("24h");
    let selectedTempUnit = $state<TemperatureUnit>("celsius");
    let selectedNetworkUnit = $state<NetworkUnit>("bytes");
    let selectedDiskUnit = $state<DiskUnit>("bytes");
    let gaugeWarning = $state(70);
    let gaugeCritical = $state(90);

    // Sync local state when user loads
    $effect(() => {
        if (user) {
            selectedTimeRange = user.default_time_range ?? "1h";
            selectedTheme = user.theme ?? "system";
            selectedTimeFormat = user.time_format ?? "24h";
            selectedTempUnit = user.temperature_unit ?? "celsius";
            selectedNetworkUnit = user.network_unit ?? "bytes";
            selectedDiskUnit = user.disk_unit ?? "bytes";
            gaugeWarning = user.gauge_warning_threshold ?? 70;
            gaugeCritical = user.gauge_critical_threshold ?? 90;
        }
    });

    let saving = $state(false);
    let saveError = $state("");
    let saveSuccess = $state(false);

    const selectedTimeRangeLabel = $derived(
        TIME_RANGES.find((r) => r.value === selectedTimeRange)?.label ??
            selectedTimeRange,
    );

    const themeOptions: { value: Theme; label: string; icon: typeof Sun }[] = [
        { value: "light", label: "Light", icon: Sun },
        { value: "dark", label: "Dark", icon: Moon },
        { value: "system", label: "System", icon: Monitor },
    ];

    async function handleSave() {
        saveError = "";
        saveSuccess = false;

        // Validate gauge thresholds
        if (gaugeWarning < 1 || gaugeWarning > 99) {
            saveError = "Warning threshold must be between 1 and 99.";
            return;
        }
        if (gaugeCritical < 1 || gaugeCritical > 100) {
            saveError = "Critical threshold must be between 1 and 100.";
            return;
        }
        if (gaugeWarning >= gaugeCritical) {
            saveError =
                "Warning threshold must be lower than critical threshold.";
            return;
        }

        saving = true;
        try {
            await userStore.updatePreferences({
                default_time_range: selectedTimeRange,
                time_format: selectedTimeFormat,
                temperature_unit: selectedTempUnit,
                network_unit: selectedNetworkUnit,
                disk_unit: selectedDiskUnit,
                gauge_warning_threshold: gaugeWarning,
                gauge_critical_threshold: gaugeCritical,
            });
            saveSuccess = true;
            setTimeout(() => {
                saveSuccess = false;
            }, 3000);
        } catch (err) {
            saveError =
                err instanceof Error
                    ? err.message
                    : "Failed to save preferences.";
        } finally {
            saving = false;
        }
    }
</script>

<svelte:head>
    <title>Settings - Watchflare</title>
</svelte:head>

{#if !cookieSecure}
    <div
        role="alert"
        class="mb-6 flex items-start gap-3 rounded-lg border border-warning/40 bg-warning/10 px-4 py-3 text-sm text-warning"
    >
        <ShieldAlert class="h-4 w-4 shrink-0 mt-0.5" />
        <div>
            <p class="font-medium">Cookies are not marked Secure</p>
            <p class="mt-0.5 text-warning/80">
                Serve Watchflare over HTTPS to enable secure cookies. Either
                configure direct TLS via <code
                    class="font-mono text-xs bg-warning/10 px-1 rounded"
                    >TLS_CERT_FILE</code
                >
                /
                <code class="font-mono text-xs bg-warning/10 px-1 rounded"
                    >TLS_KEY_FILE</code
                >, or use a reverse proxy (Nginx, Caddy, Traefik) and add its IP
                to the
                <code class="font-mono text-xs bg-warning/10 px-1 rounded"
                    >TRUSTED_PROXIES</code
                > environment variable.
            </p>
        </div>
    </div>
{/if}

<!-- General Preferences Card -->
<div class="rounded-lg border bg-card p-4 sm:p-6 mb-6">
    <h2 class="text-lg font-semibold text-foreground mb-6">General</h2>

    <!-- Theme -->
    <div class="mb-6">
        <p class="block text-sm font-medium text-foreground mb-1">Theme</p>
        <p class="text-xs text-muted-foreground mb-3">Interface color scheme</p>
        <div class="flex gap-2">
            {#each themeOptions as option}
                {@const Icon = option.icon}
                <button
                    onclick={() => userStore.updateTheme(option.value)}
                    class="flex items-center gap-2 rounded-lg border px-4 py-2.5 text-sm font-medium transition-colors {selectedTheme ===
                    option.value
                        ? 'border-primary bg-primary/10 text-primary'
                        : 'border-border text-muted-foreground hover:bg-muted hover:text-foreground'}"
                >
                    <Icon class="h-4 w-4" />
                    {option.label}
                </button>
            {/each}
        </div>
    </div>

    <!-- Default Time Range -->
    <div class="mb-6">
        <p class="block text-sm font-medium text-foreground mb-1">
            Default Time Range
        </p>
        <p class="text-xs text-muted-foreground mb-3">
            Default range for dashboard and host metrics charts
        </p>
        <div class="w-48">
            <Select.Root
                type="single"
                value={selectedTimeRange}
                onValueChange={(v) => {
                    if (v) selectedTimeRange = v as TimeRange;
                }}
            >
                <Select.Trigger items={TIME_RANGES.map((r) => r.label)}>
                    <span>{selectedTimeRangeLabel}</span>
                </Select.Trigger>
                <Select.Content>
                    {#each TIME_RANGES as range}
                        <Select.Item value={range.value} label={range.label}>
                            {range.label}
                        </Select.Item>
                    {/each}
                </Select.Content>
            </Select.Root>
        </div>
    </div>

    <!-- Time Format -->
    <div class="mb-6">
        <p class="block text-sm font-medium text-foreground mb-1">
            Time Format
        </p>
        <p class="text-xs text-muted-foreground mb-3">
            Clock format used throughout the interface
        </p>
        <div class="flex gap-2">
            {#each [{ value: "24h", label: "24-hour" }, { value: "12h", label: "12-hour (AM/PM)" }] as opt}
                <button
                    onclick={() => {
                        selectedTimeFormat = opt.value as TimeFormat;
                    }}
                    class="flex items-center gap-2 rounded-lg border px-4 py-2.5 text-sm font-medium transition-colors {selectedTimeFormat ===
                    opt.value
                        ? 'border-primary bg-primary/10 text-primary'
                        : 'border-border text-muted-foreground hover:bg-muted hover:text-foreground'}"
                >
                    {opt.label}
                </button>
            {/each}
        </div>
    </div>

    <!-- Temperature Unit -->
    <div class="mb-6">
        <p class="block text-sm font-medium text-foreground mb-1">
            Temperature Unit
        </p>
        <p class="text-xs text-muted-foreground mb-3">
            Unit for CPU and sensor temperature readings
        </p>
        <div class="flex gap-2">
            {#each [{ value: "celsius", label: "°C — Celsius" }, { value: "fahrenheit", label: "°F — Fahrenheit" }] as opt}
                <button
                    onclick={() => {
                        selectedTempUnit = opt.value as TemperatureUnit;
                    }}
                    class="flex items-center gap-2 rounded-lg border px-4 py-2.5 text-sm font-medium transition-colors {selectedTempUnit ===
                    opt.value
                        ? 'border-primary bg-primary/10 text-primary'
                        : 'border-border text-muted-foreground hover:bg-muted hover:text-foreground'}"
                >
                    {opt.label}
                </button>
            {/each}
        </div>
    </div>

    <!-- Network Unit -->
    <div class="mb-6">
        <p class="block text-sm font-medium text-foreground mb-1">
            Network Throughput Unit
        </p>
        <p class="text-xs text-muted-foreground mb-3">
            Unit for network RX/TX rates
        </p>
        <div class="flex gap-2">
            {#each [{ value: "bytes", label: "Bytes/s (MB/s)" }, { value: "bits", label: "Bits/s (Mbps)" }] as opt}
                <button
                    onclick={() => {
                        selectedNetworkUnit = opt.value as NetworkUnit;
                    }}
                    class="flex items-center gap-2 rounded-lg border px-4 py-2.5 text-sm font-medium transition-colors {selectedNetworkUnit ===
                    opt.value
                        ? 'border-primary bg-primary/10 text-primary'
                        : 'border-border text-muted-foreground hover:bg-muted hover:text-foreground'}"
                >
                    {opt.label}
                </button>
            {/each}
        </div>
    </div>

    <!-- Disk Unit -->
    <div class="mb-6">
        <p class="block text-sm font-medium text-foreground mb-1">
            Disk I/O Unit
        </p>
        <p class="text-xs text-muted-foreground mb-3">
            Unit for disk read/write rates
        </p>
        <div class="flex gap-2">
            {#each [{ value: "bytes", label: "Bytes/s (MB/s)" }, { value: "bits", label: "Bits/s (Mbps)" }] as opt}
                <button
                    onclick={() => {
                        selectedDiskUnit = opt.value as DiskUnit;
                    }}
                    class="flex items-center gap-2 rounded-lg border px-4 py-2.5 text-sm font-medium transition-colors {selectedDiskUnit ===
                    opt.value
                        ? 'border-primary bg-primary/10 text-primary'
                        : 'border-border text-muted-foreground hover:bg-muted hover:text-foreground'}"
                >
                    {opt.label}
                </button>
            {/each}
        </div>
    </div>

    <!-- Gauge Thresholds -->
    <div class="mb-6">
        <p class="block text-sm font-medium text-foreground mb-1">
            Gauge Color Thresholds
        </p>
        <p class="text-xs text-muted-foreground mb-3">
            Percentage values that trigger warning and critical colors in CPU,
            memory and disk gauges
        </p>
        <div class="flex items-center gap-6">
            <div
                class="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-3"
            >
                <span class="text-xs font-medium text-warning sm:w-16"
                    >Warning</span
                >
                <div class="flex items-center gap-2">
                    <input
                        type="number"
                        min="1"
                        max="99"
                        bind:value={gaugeWarning}
                        class="w-20 rounded-lg border bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                    />
                    <span class="text-xs text-muted-foreground">%</span>
                </div>
            </div>
            <div
                class="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-3"
            >
                <span class="text-xs font-medium text-danger sm:w-16"
                    >Critical</span
                >
                <div class="flex items-center gap-2">
                    <input
                        type="number"
                        min="1"
                        max="100"
                        bind:value={gaugeCritical}
                        class="w-20 rounded-lg border bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
                    />
                    <span class="text-xs text-muted-foreground">%</span>
                </div>
            </div>
        </div>
    </div>

    <!-- Save button -->
    {#if saveError}
        <div
            class="mb-4 rounded-lg border border-destructive bg-destructive/10 p-3"
        >
            <p class="text-sm text-destructive">{saveError}</p>
        </div>
    {/if}

    <button
        onclick={handleSave}
        disabled={saving}
        class="flex items-center gap-2 rounded-lg bg-primary px-5 py-2.5 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
    >
        {#if saveSuccess}
            <Check class="h-4 w-4" />
            Saved
        {:else}
            {saving ? "Saving..." : "Save preferences"}
        {/if}
    </button>
</div>

<!-- About -->
<div
    class="mt-6 flex items-center justify-center gap-3 text-xs text-muted-foreground"
>
    <a
        href={githubUrl}
        target="_blank"
        rel="noopener noreferrer"
        class="flex items-center gap-1.5 hover:text-foreground transition-colors"
    >
        <svg
            class="h-3.5 w-3.5"
            viewBox="0 0 24 24"
            fill="currentColor"
            aria-hidden="true"
        >
            <path
                d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"
            />
        </svg>
        GitHub
    </a>
    <span class="opacity-40">|</span>
    <a
        href={releaseUrl}
        target="_blank"
        rel="noopener noreferrer"
        class="hover:text-foreground transition-colors"
    >
        Watchflare v{APP_VERSION}
    </a>
</div>
