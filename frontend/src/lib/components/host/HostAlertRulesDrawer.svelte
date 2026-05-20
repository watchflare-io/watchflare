<script lang="ts">
    import { onDestroy } from "svelte";
    import { Check, X, RotateCcw } from "lucide-svelte";
    import { getAlertRules, getHostAlertRules, upsertHostAlertRule, deleteHostAlertRule } from "$lib/api";
    import type { AlertMetricType, AlertRule, EffectiveAlertRule } from "$lib/types";
    import { ALERT_METRIC_LABELS } from "$lib/types";
    import RightSidebar from "$lib/components/RightSidebar.svelte";
    import Toggle from "$lib/components/ui/Toggle.svelte";
    import Slider from "$lib/components/ui/Slider.svelte";

    const {
        hostId,
        open,
        onClose,
        onSave,
    }: {
        hostId: string;
        open: boolean;
        onClose: () => void;
        onSave?: (hasActiveRules: boolean) => void;
    } = $props();

    type EditableRule = {
        metric_type: AlertMetricType;
        enabled: boolean;
        threshold: number;
        duration_minutes: number;
        is_override: boolean;
        dirty: boolean;
    };

    let rules = $state<EditableRule[]>([]);
    let globalRules = $state<Record<string, AlertRule>>({});
    let loaded = $state(false);
    let loadError = $state(false);
    let saving = $state(false);
    let saveSuccess = $state(false);
    let saveError = $state("");
    let resettingRule = $state<string | null>(null);
    let debounceTimer: ReturnType<typeof setTimeout> | undefined;
    let successTimer: ReturnType<typeof setTimeout> | undefined;

    onDestroy(() => {
        clearTimeout(debounceTimer);
        clearTimeout(successTimer);
    });

    function handleClose() {
        clearTimeout(debounceTimer);
        if (rules.some(r => r.dirty)) {
            autoSave();
        }
        onClose();
    }

    let hasLoaded = false;
    $effect(() => {
        if (open && !hasLoaded) {
            hasLoaded = true;
            loadRules();
        }
    });

    async function loadRules() {
        loaded = false;
        loadError = false;
        try {
            const [hostRes, globalRes] = await Promise.all([
                getHostAlertRules(hostId),
                getAlertRules(),
            ]);
            const globalMap: Record<string, AlertRule> = {};
            for (const r of globalRes.rules) {
                globalMap[r.metric_type] = r;
            }
            globalRules = globalMap;
            rules = hostRes.rules.map((r: EffectiveAlertRule) => ({
                metric_type: r.metric_type,
                enabled: r.enabled,
                threshold: r.threshold,
                duration_minutes: r.duration_minutes,
                is_override: r.is_override,
                dirty: false,
            }));
        } catch {
            loadError = true;
        }
        loaded = true;
    }

    function isReallyDifferent(rule: EditableRule): boolean {
        if (!rule.is_override) return false;
        const g = globalRules[rule.metric_type];
        if (!g) return false;
        return rule.enabled !== g.enabled || rule.threshold !== g.threshold || rule.duration_minutes !== g.duration_minutes;
    }

    function markDirty(metricType: AlertMetricType) {
        const rule = rules.find((r) => r.metric_type === metricType);
        if (rule) {
            rule.dirty = true;
            rule.is_override = true;
        }
        clearTimeout(debounceTimer);
        debounceTimer = setTimeout(() => autoSave(), 800);
    }

    async function autoSave() {
        saveError = "";
        saveSuccess = false;
        clearTimeout(successTimer);

        const invalid = rules.find(
            (r) =>
                r.dirty &&
                r.enabled &&
                r.metric_type !== "host_down" &&
                (isNaN(r.threshold) || r.threshold === null),
        );
        if (invalid) {
            saveError = `Enter a threshold for ${ALERT_METRIC_LABELS[invalid.metric_type]}.`;
            return;
        }
        const invalidDuration = rules.find(
            (r) =>
                r.dirty &&
                r.enabled &&
                (isNaN(r.duration_minutes) || r.duration_minutes === null),
        );
        if (invalidDuration) {
            saveError = `Enter a duration for ${ALERT_METRIC_LABELS[invalidDuration.metric_type]}.`;
            return;
        }

        saving = true;
        try {
            const dirtyRules = rules.filter((r) => r.dirty);
            await Promise.all(
                dirtyRules.map((r) =>
                    upsertHostAlertRule(hostId, r.metric_type, {
                        enabled: r.enabled,
                        threshold: r.threshold,
                        duration_minutes: r.duration_minutes,
                    }),
                ),
            );
            for (const rule of rules) {
                if (rule.dirty) rule.dirty = false;
            }
            onSave?.(rules.some(r => r.enabled));
            saveSuccess = true;
            successTimer = setTimeout(() => { saveSuccess = false; }, 2000);
        } catch (err) {
            saveError = err instanceof Error ? err.message : "Failed to save.";
        } finally {
            saving = false;
        }
    }

    const DESCRIPTIONS: Record<AlertMetricType, string> = {
        host_down: "Alert when the host stops sending heartbeats.",
        cpu_usage: "Alert when CPU usage stays above the threshold.",
        memory_usage: "Alert when RAM usage stays above the threshold.",
        disk_usage: "Alert when disk usage stays above the threshold.",
        load_avg: "Alert when the 1-min load average exceeds the threshold.",
        load_avg_5: "Alert when the 5-min load average exceeds the threshold.",
        load_avg_15:
            "Alert when the 15-min load average exceeds the threshold.",
        temperature: "Alert when CPU temperature exceeds the threshold.",
    };

    const GAUGE_MAX: Partial<Record<AlertMetricType, number>> = {
        cpu_usage: 100,
        memory_usage: 100,
        disk_usage: 100,
        temperature: 120,
        load_avg: 16,
        load_avg_5: 16,
        load_avg_15: 16,
    };

    const GAUGE_STEP: Partial<Record<AlertMetricType, number>> = {
        load_avg: 0.1,
        load_avg_5: 0.1,
        load_avg_15: 0.1,
    };

    const DURATION_MAX = 60;

    const RECOMMENDED_THRESHOLDS: Partial<Record<AlertMetricType, number>> = {
        load_avg: 2.0,
        load_avg_5: 2.0,
        load_avg_15: 2.0,
    };

    function thresholdPlaceholder(metricType: AlertMetricType): string {
        const global = globalRules[metricType];
        const val = global?.threshold ?? RECOMMENDED_THRESHOLDS[metricType];
        return val !== undefined ? String(val) : '';
    }

    function durationPlaceholder(metricType: AlertMetricType): string {
        const val = globalRules[metricType]?.duration_minutes;
        return val !== undefined ? String(val) : '5';
    }

    function thresholdUnit(metricType: AlertMetricType): string {
        if (
            metricType === "cpu_usage" ||
            metricType === "memory_usage" ||
            metricType === "disk_usage"
        )
            return "%";
        if (metricType === "temperature") return "°C";
        return "";
    }

    async function handleReset(metricType: AlertMetricType) {
        resettingRule = metricType;
        try {
            await deleteHostAlertRule(hostId, metricType);
            const rule = rules.find(r => r.metric_type === metricType);
            const g = globalRules[metricType];
            if (rule && g) {
                rule.enabled = g.enabled;
                rule.threshold = g.threshold;
                rule.duration_minutes = g.duration_minutes;
                rule.is_override = false;
                rule.dirty = false;
            }
            onSave?.(rules.some(r => r.enabled));
        } catch {
            // non-critical
        } finally {
            resettingRule = null;
        }
    }

</script>

<RightSidebar {open} onClose={handleClose} size="wide">
    <!-- Header -->
    <div class="flex items-center justify-between border-b px-6 py-4 shrink-0">
        <div class="flex items-center gap-3">
            <h2 class="text-base font-semibold text-foreground">Alert Rules</h2>
            {#if saving}
                <span class="text-xs text-muted-foreground">Saving…</span>
            {:else if saveSuccess}
                <span class="flex items-center gap-1 text-xs text-success">
                    <Check class="h-3 w-3" />Saved
                </span>
            {:else if saveError}
                <span class="text-xs text-destructive">{saveError}</span>
            {/if}
        </div>
        <button
            type="button"
            onclick={onClose}
            class="flex h-8 w-8 items-center justify-center rounded-lg text-muted-foreground hover:text-foreground transition-colors"
            aria-label="Close"
        >
            <X class="h-4 w-4" />
        </button>
    </div>

    <!-- Content -->
    <div class="flex-1 overflow-y-auto p-6">
        {#if !loaded}
            <div class="flex items-center justify-center py-16">
                <p class="text-sm text-muted-foreground">Loading…</p>
            </div>
        {:else if loadError}
            <div class="flex items-center justify-center py-16">
                <p class="text-sm text-destructive">Failed to load alert rules.</p>
            </div>
        {:else}
            <div>
                {#each rules as rule (rule.metric_type)}
                    <div class="py-4 border-b border-border/50 last:border-0">
                        <!-- Title + toggle -->
                        <div class="flex items-center justify-between gap-4">
                            <div class="flex items-center gap-2 min-w-0">
                                <span class="text-sm font-medium text-foreground"
                                    >{ALERT_METRIC_LABELS[rule.metric_type]}</span
                                >
                                {#if isReallyDifferent(rule)}
                                    <span class="shrink-0 text-[10px] font-medium text-primary bg-primary/10 rounded-full px-1.5 py-0.5">Custom</span>
                                {/if}
                            </div>
                            <div class="flex items-center gap-2 shrink-0">
                                {#if isReallyDifferent(rule)}
                                    <button
                                        type="button"
                                        onclick={() => handleReset(rule.metric_type)}
                                        disabled={resettingRule === rule.metric_type}
                                        title="Reset to global default"
                                        class="rounded p-1 text-muted-foreground transition-colors hover:text-foreground disabled:opacity-40"
                                    >
                                        <RotateCcw class="h-3.5 w-3.5 {resettingRule === rule.metric_type ? 'animate-spin' : ''}" />
                                    </button>
                                {/if}
                                <Toggle
                                    bind:checked={rule.enabled}
                                    onchange={() => markDirty(rule.metric_type)}
                                />
                            </div>
                        </div>

                        <!-- Description -->
                        <p class="text-xs text-muted-foreground mt-1">
                            {DESCRIPTIONS[rule.metric_type]}
                        </p>

                        <!-- Controls -->
                        {#if rule.enabled}
                            <div
                                class="mt-4 rounded-xl bg-muted/40 px-4 py-3 space-y-4"
                            >
                                {#if rule.metric_type !== "host_down"}
                                    <!-- Threshold -->
                                    <div>
                                        <p
                                            class="text-[11px] font-medium text-muted-foreground uppercase tracking-wide mb-2"
                                        >
                                            Threshold
                                        </p>
                                        <div class="flex items-center gap-3">
                                            <Slider
                                                bind:value={rule.threshold}
                                                min={0}
                                                max={GAUGE_MAX[rule.metric_type]}
                                                step={GAUGE_STEP[rule.metric_type] ?? 1}
                                                oninput={() => markDirty(rule.metric_type)}
                                            />
                                            <div class="flex items-center gap-1 shrink-0">
                                                <input
                                                    type="number"
                                                    min="0"
                                                    step={GAUGE_STEP[rule.metric_type] ?? 1}
                                                    placeholder={thresholdPlaceholder(rule.metric_type)}
                                                    bind:value={rule.threshold}
                                                    oninput={() => markDirty(rule.metric_type)}
                                                    class="w-14 rounded-lg border bg-background px-2 py-1 text-xs text-foreground text-right focus:outline-none focus-visible:ring-2 focus-visible:ring-primary [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                                                />
                                                {#if thresholdUnit(rule.metric_type)}
                                                    <span class="text-xs text-muted-foreground w-5">{thresholdUnit(rule.metric_type)}</span>
                                                {/if}
                                            </div>
                                        </div>
                                    </div>
                                {/if}

                                <!-- Duration -->
                                <div>
                                    <p
                                        class="text-[11px] font-medium text-muted-foreground uppercase tracking-wide mb-2"
                                    >
                                        Duration
                                    </p>
                                    <div class="flex items-center gap-3">
                                        <Slider
                                            bind:value={rule.duration_minutes}
                                            min={1}
                                            max={DURATION_MAX}
                                            step={1}
                                            oninput={() => markDirty(rule.metric_type)}
                                        />
                                        <div
                                            class="flex items-center gap-1 shrink-0"
                                        >
                                            <input
                                                type="number"
                                                min="1"
                                                max={DURATION_MAX}
                                                step="1"
                                                placeholder={durationPlaceholder(rule.metric_type)}
                                                bind:value={rule.duration_minutes}
                                                oninput={() => markDirty(rule.metric_type)}
                                                class="w-14 rounded-lg border bg-background px-2 py-1 text-xs text-foreground text-right focus:outline-none focus-visible:ring-2 focus-visible:ring-primary [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                                            />
                                            <span
                                                class="text-xs text-muted-foreground w-5"
                                                >min</span
                                            >
                                        </div>
                                    </div>
                                </div>
                            </div>
                        {/if}
                    </div>
                {/each}
            </div>
        {/if}
    </div>

</RightSidebar>
