<script lang="ts">
    import { setupTOTP, enableTOTP } from "$lib/api";
    import { lockBodyScroll } from "$lib/actions/lockBodyScroll";
    import QRCode from "qrcode";
    import { Copy, Check } from "lucide-svelte";

    const {
        open = false,
        onClose,
        onEnabled,
    }: {
        open?: boolean;
        onClose: () => void;
        onEnabled: () => void;
    } = $props();

    let step: "qr" | "verify" | "backup" = $state("qr");
    let otpauthURL = $state("");
    let secret = $state("");
    let code = $state("");
    let backupCodes: string[] = $state([]);
    let backupSaved = $state(false);
    let error = $state("");
    let loading = $state(false);
    let copied = $state(false);
    let secretCopied = $state(false);
    let canvasEl: HTMLCanvasElement | undefined = $state(undefined);

    $effect(() => {
        if (open) {
            step = "qr";
            code = "";
            backupCodes = [];
            backupSaved = false;
            error = "";
            loadSetup();
        }
    });

    $effect(() => {
        if (step === "qr" && otpauthURL && canvasEl) {
            QRCode.toCanvas(canvasEl, otpauthURL, { width: 200, margin: 1 });
        }
    });

    async function loadSetup() {
        loading = true;
        try {
            const res = await setupTOTP();
            otpauthURL = res.otpauth_url;
            secret = res.secret;
        } catch {
            error = "Failed to start 2FA setup. Please try again.";
        } finally {
            loading = false;
        }
    }

    async function handleEnable() {
        error = "";
        loading = true;
        try {
            const res = await enableTOTP(code);
            backupCodes = res.backup_codes;
            step = "backup";
        } catch {
            error = "Invalid code. Check your authenticator app and try again.";
        } finally {
            loading = false;
        }
    }

    async function copySecret() {
        await navigator.clipboard.writeText(secret);
        secretCopied = true;
        setTimeout(() => { secretCopied = false; }, 2000);
    }

    async function copyAll() {
        await navigator.clipboard.writeText(backupCodes.join("\n"));
        copied = true;
        setTimeout(() => {
            copied = false;
        }, 2000);
    }

    function formatSecret(s: string): string {
        return s.replace(/(.{4})/g, "$1 ").trim();
    }
</script>

{#if open}
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div
        role="presentation"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
        onkeydown={(e) => {
            if (e.key === "Escape") onClose();
        }}
        onclick={() => { if (step !== "backup") onClose(); }}
        use:lockBodyScroll
    >
        <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
        <div
            class="w-full max-w-md rounded-lg border bg-card shadow-lg"
            role="dialog"
            aria-modal="true"
            aria-label="Set up two-factor authentication"
            tabindex="-1"
            onclick={(e) => e.stopPropagation()}
        >
            <div class="flex items-center justify-between border-b px-6 py-4">
                <h2 class="text-base font-semibold text-foreground">
                    Enable Two-Factor Authentication
                </h2>
                <button
                    onclick={onClose}
                    class="text-muted-foreground hover:text-foreground rounded focus-visible:ring-2 focus-visible:ring-primary"
                    aria-label="Close">✕</button
                >
            </div>

            <div class="px-6 py-5">
                {#if error && step !== "backup"}
                    <div
                        role="alert"
                        class="mb-4 rounded-lg border border-destructive bg-destructive/10 p-3"
                    >
                        <p class="text-sm text-destructive">{error}</p>
                    </div>
                {/if}

                {#if step === "qr"}
                    <p class="text-sm text-muted-foreground mb-4">
                        Scan this QR code with your authenticator app (Proton
                        Authenticator, Google Authenticator, 2FAS, etc.).
                    </p>
                    <div class="flex justify-center mb-4">
                        {#if loading}
                            <div
                                class="h-[200px] w-[200px] rounded bg-muted animate-pulse"
                            ></div>
                        {:else}
                            <canvas bind:this={canvasEl} class="rounded border"
                            ></canvas>
                        {/if}
                    </div>
                    <p class="text-xs text-muted-foreground text-center mb-1">
                        Or enter the code manually:
                    </p>
                    <div class="flex items-center gap-2 bg-muted rounded px-3 py-2 mb-6">
                        <p class="text-xs font-mono text-foreground tracking-wider select-all flex-1 text-center">
                            {formatSecret(secret)}
                        </p>
                        <button
                            onclick={copySecret}
                            class="shrink-0 text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary rounded"
                            aria-label="Copy secret key"
                        >
                            {#if secretCopied}
                                <Check class="h-3.5 w-3.5 text-green-600" />
                            {:else}
                                <Copy class="h-3.5 w-3.5" />
                            {/if}
                        </button>
                    </div>
                    <button
                        onclick={() => {
                            step = "verify";
                        }}
                        disabled={loading || !otpauthURL}
                        class="w-full rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        Next
                    </button>
                {:else if step === "verify"}
                    <p class="text-sm text-muted-foreground mb-4">
                        Enter the 6-digit code from your authenticator app to
                        confirm setup.
                    </p>
                    <form
                        onsubmit={(e) => {
                            e.preventDefault();
                            handleEnable();
                        }}
                    >
                        <div class="mb-6">
                            <label
                                for="confirm-code"
                                class="block text-sm font-medium text-foreground mb-2"
                            >
                                Verification code
                            </label>
                            <input
                                id="confirm-code"
                                type="text"
                                inputmode="numeric"
                                bind:value={code}
                                placeholder="000000"
                                maxlength={6}
                                autofocus
                                disabled={loading}
                                class="w-full rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50 tracking-widest text-center text-lg"
                            />
                        </div>
                        <div class="flex gap-2">
                            <button
                                type="button"
                                onclick={() => {
                                    step = "qr";
                                    error = "";
                                }}
                                class="flex-1 rounded-lg border px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-muted"
                            >
                                Back
                            </button>
                            <button
                                type="submit"
                                disabled={loading || code.length < 6}
                                class="flex-1 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
                            >
                                {loading ? "Verifying..." : "Enable 2FA"}
                            </button>
                        </div>
                    </form>
                {:else if step === "backup"}
                    <p class="text-sm text-muted-foreground mb-4">
                        Save these backup codes somewhere safe. Each code can
                        only be used once. You'll need them if you lose access
                        to your authenticator app.
                    </p>
                    <div class="rounded-lg border bg-muted/50 p-4 mb-4">
                        <div class="grid grid-cols-2 gap-2 mb-3">
                            {#each backupCodes as bc}
                                <code
                                    class="text-xs font-mono text-foreground tracking-wider"
                                    >{bc}</code
                                >
                            {/each}
                        </div>
                        <button
                            onclick={copyAll}
                            class="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary rounded"
                        >
                            {#if copied}
                                <Check class="h-3.5 w-3.5 text-green-600" />
                                <span class="text-green-600">Copied!</span>
                            {:else}
                                <Copy class="h-3.5 w-3.5" />
                                Copy all
                            {/if}
                        </button>
                    </div>
                    <label class="flex items-start gap-2 mb-6 cursor-pointer">
                        <input
                            type="checkbox"
                            bind:checked={backupSaved}
                            class="mt-0.5 rounded border-border"
                        />
                        <span class="text-sm text-foreground"
                            >I have saved my backup codes</span
                        >
                    </label>
                    <button
                        onclick={onEnabled}
                        disabled={!backupSaved}
                        class="w-full rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        Done
                    </button>
                {/if}
            </div>
        </div>
    </div>
{/if}
