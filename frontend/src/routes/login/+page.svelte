<script lang="ts">
    import { onMount } from "svelte";
    import { login, checkSetupRequired, updatePreferences, getCurrentUser, verifyTOTP } from "$lib/api";
    import { goto } from "$app/navigation";
    import { loginSchema, validateForm } from "$lib/validation";
    import { authTheme } from "$lib/stores/auth-theme";
    import AuthThemeToggle from "$lib/components/AuthThemeToggle.svelte";
    import Logo from "$lib/components/Logo.svelte";
    import { get } from "svelte/store";

    let email = "";
    let password = "";
    let error = "";
    let fieldErrors: Record<string, string> = {};
    let loading = false;

    let step: 'credentials' | 'totp' = 'credentials';
    let totpCode = '';
    let useBackupCode = false;

    onMount(async () => {
        const setupRequired = await checkSetupRequired();
        if (setupRequired) {
            goto("/register");
        }
    });

    async function redirectAfterLogin() {
        const theme = get(authTheme);
        if (theme !== "light") {
            const { user } = await getCurrentUser();
            await updatePreferences({ default_time_range: user.default_time_range, theme });
        }
        goto("/");
    }

    async function handleLogin() {
        error = "";
        fieldErrors = {};

        const result = validateForm(loginSchema, { email, password });
        if (!result.success) {
            fieldErrors = result.errors;
            return;
        }

        loading = true;
        try {
            const response = await login(email, password);
            if (response.totp_required) {
                step = 'totp';
                return;
            }
            await redirectAfterLogin();
        } catch (err: unknown) {
            if (err instanceof Error) {
                if (err.message === "invalid credentials") {
                    error = "Invalid credentials.";
                } else if ((err as { status?: number }).status === 503) {
                    error = "Service unavailable.";
                } else {
                    error = "An unexpected error occurred. Please try again.";
                }
            }
        } finally {
            loading = false;
        }
    }

    async function handleVerifyTOTP() {
        error = "";
        if (!totpCode.trim()) return;

        loading = true;
        try {
            if (useBackupCode) {
                await verifyTOTP(undefined, totpCode.trim());
            } else {
                await verifyTOTP(totpCode.trim(), undefined);
            }
            await redirectAfterLogin();
        } catch {
            error = "Invalid code. Please try again.";
        } finally {
            loading = false;
        }
    }

    function resetToCredentials() {
        step = 'credentials';
        totpCode = '';
        useBackupCode = false;
        error = '';
    }
</script>

<svelte:head>
    <title>Login - Watchflare</title>
</svelte:head>

<AuthThemeToggle />

<div class="flex min-h-dvh items-center justify-center bg-background p-4">
    <div class="w-full max-w-md">
        <!-- Logo/Title -->
        <div class="mb-8 text-center">
            <div class="flex justify-center mb-4">
                <Logo class="h-16 w-16" />
            </div>
            <h1 class="text-3xl font-semibold text-foreground mb-2">
                Watchflare
            </h1>
            <p class="text-sm text-muted-foreground">
                Host Monitoring Dashboard
            </p>
        </div>

        <!-- Login Card -->
        <div class="rounded-lg border bg-card p-4 sm:p-8 shadow-sm">
            {#if step === 'credentials'}
                <h2 class="text-lg font-semibold text-foreground mb-6">
                    Login to your account
                </h2>
                <form onsubmit={(e) => { e.preventDefault(); handleLogin(); }}>
                    <!-- Email -->
                    <div class="mb-4">
                        <label for="email" class="block text-sm font-medium text-foreground mb-2">Email</label>
                        <input
                            id="email"
                            type="email"
                            autocomplete="email"
                            bind:value={email}
                            placeholder="admin@watchflare.io"
                            disabled={loading}
                            aria-invalid={!!fieldErrors.email}
                            aria-describedby={fieldErrors.email ? "email-error" : undefined}
                            class="w-full rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50 {fieldErrors.email ? 'border-destructive' : ''}"
                        />
                        {#if fieldErrors.email}<p id="email-error" class="mt-1 text-xs text-destructive">{fieldErrors.email}</p>{/if}
                    </div>
                    <!-- Password -->
                    <div class="mb-6">
                        <label for="password" class="block text-sm font-medium text-foreground mb-2">Password</label>
                        <input
                            id="password"
                            type="password"
                            autocomplete="current-password"
                            bind:value={password}
                            placeholder="••••••••"
                            disabled={loading}
                            aria-invalid={!!fieldErrors.password}
                            aria-describedby={fieldErrors.password ? "password-error" : undefined}
                            class="w-full rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50 {fieldErrors.password ? 'border-destructive' : ''}"
                        />
                        {#if fieldErrors.password}<p id="password-error" class="mt-1 text-xs text-destructive">{fieldErrors.password}</p>{/if}
                    </div>
                    {#if error}
                        <div role="alert" class="mb-4 rounded-lg border border-destructive bg-destructive/10 p-3">
                            <p class="text-sm text-destructive">{error}</p>
                        </div>
                    {/if}
                    <button
                        type="submit"
                        disabled={loading}
                        class="w-full rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                        {loading ? "Logging in..." : "Login"}
                    </button>
                </form>
            {:else}
                <h2 class="text-lg font-semibold text-foreground mb-2">Two-factor authentication</h2>
                <p class="text-sm text-muted-foreground mb-6">
                    {useBackupCode ? "Enter one of your backup codes." : "Enter the 6-digit code from your authenticator app."}
                </p>
                <form onsubmit={(e) => { e.preventDefault(); handleVerifyTOTP(); }}>
                    <div class="mb-6">
                        <label for="totp-code" class="block text-sm font-medium text-foreground mb-2">
                            {useBackupCode ? "Backup code" : "Authenticator code"}
                        </label>
                        <input
                            id="totp-code"
                            type="text"
                            inputmode="numeric"
                            bind:value={totpCode}
                            placeholder={useBackupCode ? "Enter backup code" : "000000"}
                            disabled={loading}
                            autofocus
                            maxlength={useBackupCode ? 10 : 6}
                            oninput={() => { if (!useBackupCode && totpCode.length === 6) handleVerifyTOTP(); }}
                            class="w-full rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50 tracking-widest text-center text-lg"
                        />
                    </div>
                    {#if error}
                        <div role="alert" class="mb-4 rounded-lg border border-destructive bg-destructive/10 p-3">
                            <p class="text-sm text-destructive">{error}</p>
                        </div>
                    {/if}
                    <button
                        type="submit"
                        disabled={loading || !totpCode.trim()}
                        class="w-full rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed mb-3"
                    >
                        {loading ? "Verifying..." : "Verify"}
                    </button>
                    <div class="flex items-center justify-between text-sm">
                        <button
                            type="button"
                            onclick={() => { useBackupCode = !useBackupCode; totpCode = ''; error = ''; }}
                            class="text-primary hover:underline focus-visible:ring-2 focus-visible:ring-primary rounded"
                        >
                            {useBackupCode ? "Use authenticator app" : "Use a backup code"}
                        </button>
                        <button
                            type="button"
                            onclick={resetToCredentials}
                            class="text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary rounded"
                        >
                            ← Back
                        </button>
                    </div>
                </form>
            {/if}
        </div>

        <!-- Footer -->
        <p class="mt-6 text-center text-xs text-muted-foreground">
            Watchflare Host Monitoring
        </p>
    </div>
</div>
