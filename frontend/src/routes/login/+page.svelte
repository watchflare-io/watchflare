<script lang="ts">
    import { onMount } from "svelte";
    import { login, checkSetupRequired, updatePreferences, getCurrentUser } from "$lib/api";
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

    onMount(async () => {
        const setupRequired = await checkSetupRequired();
        if (setupRequired) {
            goto("/register");
        }
    });

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
            await login(email, password);
            const theme = get(authTheme);
            if (theme !== "light") {
                const { user } = await getCurrentUser();
                await updatePreferences({ default_time_range: user.default_time_range, theme });
            }
            goto("/");
        } catch (err) {
            if (err.message === "invalid credentials") {
                error = "Invalid credentials.";
            } else if (err.status === 503) {
                error = "Service unavailable.";
            } else {
                error = "An unexpected error occurred. Please try again.";
            }
        } finally {
            loading = false;
        }
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
            <h2 class="text-lg font-semibold text-foreground mb-6">
                Login to your account
            </h2>

            <form
                onsubmit={(e) => {
                    e.preventDefault();
                    handleLogin();
                }}
            >
                <!-- Email -->
                <div class="mb-4">
                    <label
                        for="email"
                        class="block text-sm font-medium text-foreground mb-2"
                    >
                        Email
                    </label>
                    <input
                        id="email"
                        type="email"
                        autocomplete="email"
                        bind:value={email}
                        placeholder="admin@watchflare.io"
                        disabled={loading}
                        aria-invalid={!!fieldErrors.email}
                        aria-describedby={fieldErrors.email
                            ? "email-error"
                            : undefined}
                        class="w-full rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50 {fieldErrors.email
                            ? 'border-destructive'
                            : ''}"
                    />
                    {#if fieldErrors.email}<p
                            id="email-error"
                            class="mt-1 text-xs text-destructive"
                        >
                            {fieldErrors.email}
                        </p>{/if}
                </div>

                <!-- Password -->
                <div class="mb-6">
                    <label
                        for="password"
                        class="block text-sm font-medium text-foreground mb-2"
                    >
                        Password
                    </label>
                    <input
                        id="password"
                        type="password"
                        autocomplete="current-password"
                        bind:value={password}
                        placeholder="••••••••"
                        disabled={loading}
                        aria-invalid={!!fieldErrors.password}
                        aria-describedby={fieldErrors.password
                            ? "password-error"
                            : undefined}
                        class="w-full rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50 {fieldErrors.password
                            ? 'border-destructive'
                            : ''}"
                    />
                    {#if fieldErrors.password}<p
                            id="password-error"
                            class="mt-1 text-xs text-destructive"
                        >
                            {fieldErrors.password}
                        </p>{/if}
                </div>

                <!-- Error Message -->
                {#if error}
                    <div
                        role="alert"
                        class="mb-4 rounded-lg border border-destructive bg-destructive/10 p-3"
                    >
                        <p class="text-sm text-destructive">{error}</p>
                    </div>
                {/if}

                <!-- Submit Button -->
                <button
                    type="submit"
                    disabled={loading}
                    class="w-full rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                    {loading ? "Logging in..." : "Login"}
                </button>
            </form>
        </div>

        <!-- Footer -->
        <p class="mt-6 text-center text-xs text-muted-foreground">
            Watchflare Host Monitoring
        </p>
    </div>
</div>
