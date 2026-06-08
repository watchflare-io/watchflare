<script lang="ts">
    import { onMount } from 'svelte';
    import { Trash2, Send, Loader, TriangleAlert, Check } from 'lucide-svelte';
    import Toggle from '$lib/components/ui/Toggle.svelte';
    import type { WebhookEndpoint } from '$lib/types';
    import {
        getWebhooks,
        addWebhook,
        deleteWebhook,
        setWebhookEnabled,
        testWebhook,
    } from '$lib/api';

    // ── State ──────────────────────────────────────────────────────────────

    let loaded = $state(false);
    let webhooks = $state<WebhookEndpoint[]>([]);
    // per-webhook: unknown_service warning
    let warnings = $state<Record<string, boolean>>({});
    // per-webhook: test result message
    let testMessages = $state<Record<string, { ok: boolean; text: string }>>({});
    // per-webhook: test in-progress
    let testingIds = $state<Set<string>>(new Set());
    // per-webhook: delete in-progress
    let deletingIds = $state<Set<string>>(new Set());

    let newUrl = $state('');
    let addError = $state('');
    let adding = $state(false);

    // ── Helpers ────────────────────────────────────────────────────────────

    function isValidUrl(url: string): boolean {
        return url.startsWith('http://') || url.startsWith('https://');
    }

    function truncateUrl(url: string, max = 50): string {
        return url.length > max ? url.slice(0, max) + '…' : url;
    }

    // ── Lifecycle ──────────────────────────────────────────────────────────

    onMount(async () => {
        try {
            const res = await getWebhooks();
            webhooks = res.webhooks;
        } catch {
            // non-fatal: show empty list
        }
        loaded = true;
    });

    // ── Handlers ───────────────────────────────────────────────────────────

    async function handleAdd() {
        addError = '';
        const url = newUrl.trim();
        if (!url) return;
        if (!isValidUrl(url)) {
            addError = 'URL must start with http:// or https://';
            return;
        }
        adding = true;
        try {
            const res = await addWebhook(url);
            webhooks = [...webhooks, res.webhook];
            if (res.warning === 'unknown_service') {
                warnings = { ...warnings, [res.webhook.id]: true };
            }
            newUrl = '';
        } catch (err) {
            addError = err instanceof Error ? err.message : 'Failed to add webhook.';
        } finally {
            adding = false;
        }
    }

    function handleKeydown(e: KeyboardEvent) {
        if (e.key === 'Enter') handleAdd();
    }

    async function handleToggle(webhook: WebhookEndpoint, value: boolean) {
        // bind:checked already updated webhook.enabled — just persist and rollback on error
        try {
            await setWebhookEnabled(webhook.id, value);
        } catch {
            webhook.enabled = !value;
        }
    }

    async function handleDelete(id: string) {
        deletingIds = new Set([...deletingIds, id]);
        try {
            await deleteWebhook(id);
            webhooks = webhooks.filter(w => w.id !== id);
            // clean up side-state
            const { [id]: _w, ...restW } = warnings;
            warnings = restW;
            const { [id]: _t, ...restT } = testMessages;
            testMessages = restT;
        } catch {
            // no-op: leave in list
        } finally {
            const next = new Set(deletingIds);
            next.delete(id);
            deletingIds = next;
        }
    }

    async function handleTest(id: string) {
        testMessages = { ...testMessages, [id]: { ok: false, text: '' } };
        testingIds = new Set([...testingIds, id]);
        try {
            const res = await testWebhook(id);
            testMessages = { ...testMessages, [id]: { ok: true, text: res.message } };
        } catch (err) {
            const text = err instanceof Error ? err.message : 'Test failed.';
            testMessages = { ...testMessages, [id]: { ok: false, text } };
        } finally {
            const next = new Set(testingIds);
            next.delete(id);
            testingIds = next;
        }
    }
</script>

<div
    class="rounded-lg border bg-card p-4 sm:p-6 mb-6 transition-opacity duration-200 {loaded ? 'opacity-100' : 'opacity-0'}"
>
    <h2 class="text-lg font-semibold text-foreground mb-1">Webhooks</h2>
    <p class="text-sm text-muted-foreground mb-5">
        Send alert notifications to external services.
    </p>

    <!-- Add webhook -->
    <div class="flex gap-2 items-center">
        <input
            type="url"
            aria-label="Webhook URL"
            placeholder="https://hooks.example.com/..."
            bind:value={newUrl}
            onkeydown={handleKeydown}
            disabled={adding}
            class="flex-1 rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 {addError ? 'border-destructive focus-visible:ring-destructive' : 'focus-visible:ring-primary'} disabled:opacity-50"
        />
        <button
            type="button"
            onclick={handleAdd}
            disabled={adding || !newUrl.trim()}
            class="flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed shrink-0"
        >
            {#if adding}
                <Loader class="h-4 w-4 animate-spin" />
            {/if}
            Add
        </button>
    </div>
    {#if addError}
        <p class="mt-1.5 text-xs text-destructive">{addError}</p>
    {/if}

    <!-- Webhook list -->
    {#if webhooks.length > 0}
        <div class="mt-5 space-y-3">
            {#each webhooks as webhook (webhook.id)}
                {@const isTesting = testingIds.has(webhook.id)}
                {@const isDeleting = deletingIds.has(webhook.id)}
                {@const testResult = testMessages[webhook.id]}
                {@const hasWarning = warnings[webhook.id] ?? false}

                <div class="rounded-lg border border-border bg-background px-4 py-3 transition-opacity duration-200 {webhook.enabled ? 'opacity-100' : 'opacity-50'}">
                    <!-- Row: badge + url + actions -->
                    <div class="flex items-center gap-3 flex-wrap sm:flex-nowrap">
                        <!-- Service badge -->
                        <span
                            class="shrink-0 rounded-md px-2 py-0.5 text-xs font-medium capitalize
                                {webhook.service_name === 'Generic'
                                    ? 'bg-muted text-muted-foreground'
                                    : 'bg-primary/10 text-primary'}"
                        >
                            {webhook.service_name}
                        </span>

                        <!-- URL -->
                        <span class="flex-1 min-w-0 text-sm text-foreground font-mono truncate" title={webhook.url}>
                            {truncateUrl(webhook.url)}
                        </span>

                        <!-- Actions -->
                        <div class="flex items-center gap-2 shrink-0 ml-auto">
                            <!-- Test button -->
                            <button
                                type="button"
                                onclick={() => handleTest(webhook.id)}
                                disabled={isTesting}
                                title="Send a test notification"
                                class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-xs font-medium transition-colors hover:bg-muted disabled:opacity-50 disabled:cursor-not-allowed"
                            >
                                {#if isTesting}
                                    <Loader class="h-3.5 w-3.5 animate-spin" />
                                {:else}
                                    <Send class="h-3.5 w-3.5" />
                                {/if}
                                Test
                            </button>

                            <!-- Enable/disable toggle -->
                            <Toggle
                                bind:checked={webhook.enabled}
                                size="sm"
                                aria-label="Enable {webhook.service_name} webhook"
                                onchange={(value) => handleToggle(webhook, value)}
                            />

                            <!-- Delete button -->
                            <button
                                type="button"
                                onclick={() => handleDelete(webhook.id)}
                                disabled={isDeleting}
                                title="Remove webhook"
                                class="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive disabled:opacity-50 disabled:cursor-not-allowed"
                                aria-label="Delete webhook"
                            >
                                {#if isDeleting}
                                    <Loader class="h-4 w-4 animate-spin" />
                                {:else}
                                    <Trash2 class="h-4 w-4" />
                                {/if}
                            </button>
                        </div>
                    </div>

                    <!-- Unknown service warning -->
                    {#if hasWarning}
                        <div class="mt-2 flex items-start gap-1.5 text-xs text-warning">
                            <TriangleAlert class="h-3.5 w-3.5 shrink-0 mt-0.5" />
                            <p>
                                Unknown service — notifications will be sent as generic JSON. Verify your service accepts arbitrary HTTP webhooks.
                            </p>
                        </div>
                    {/if}

                    <!-- Test result -->
                    {#if testResult && testResult.text}
                        <p class="mt-2 text-xs {testResult.ok ? 'text-success' : 'text-destructive'}">
                            {#if testResult.ok}
                                <Check class="h-3 w-3 inline mr-1" />
                            {/if}
                            {testResult.text}
                        </p>
                    {/if}
                </div>
            {/each}
        </div>
    {:else if loaded}
        <p class="mt-5 text-sm text-muted-foreground">No webhooks configured yet.</p>
    {/if}
</div>
