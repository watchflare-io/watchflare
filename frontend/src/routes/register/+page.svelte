<script lang="ts">
    import { register, updatePreferences } from "$lib/api";
    import { goto } from "$app/navigation";
    import { registerSchema, validateForm } from "$lib/validation";
    import { authTheme } from "$lib/stores/auth-theme";
    import AuthThemeToggle from "$lib/components/AuthThemeToggle.svelte";
    import { get } from "svelte/store";

    let email = $state("");
    let username = $state("");
    let password = $state("");
    let confirmPassword = $state("");
    let error = $state("");
    let fieldErrors: Record<string, string> = $state({});
    let loading = $state(false);

    async function handleRegister() {
        error = "";
        fieldErrors = {};

        const result = validateForm(registerSchema, {
            email,
            password,
            confirmPassword,
        });
        if (!result.success) {
            fieldErrors = result.errors;
            return;
        }

        loading = true;

        try {
            await register(email, password, username);
            await updatePreferences({ theme: get(authTheme) });
            goto("/");
        } catch (err) {
            error = err.message;
        } finally {
            loading = false;
        }
    }
</script>

<svelte:head>
    <title>Setup - Watchflare</title>
</svelte:head>

<AuthThemeToggle />

<div class="flex min-h-dvh items-center justify-center bg-background p-4">
    <div class="w-full max-w-md">
        <!-- Logo/Title -->
        <div class="mb-8 text-center">
            <h1 class="text-3xl font-semibold text-foreground mb-2">
                Watchflare
            </h1>
            <p class="text-sm text-muted-foreground">
                Initial Setup - Create Admin Account
            </p>
        </div>

        <!-- Register Card -->
        <div class="rounded-lg border bg-card p-4 sm:p-8 shadow-sm">
            <h2 class="text-lg font-semibold text-foreground mb-6">
                Create your admin account
            </h2>

            <form
                onsubmit={(e) => {
                    e.preventDefault();
                    handleRegister();
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

                <!-- Username -->
                <div class="mb-4">
                    <label
                        for="username"
                        class="block text-sm font-medium text-foreground mb-2"
                    >
                        Username <span class="text-muted-foreground font-normal">(optional)</span>
                    </label>
                    <input
                        id="username"
                        type="text"
                        autocomplete="username"
                        bind:value={username}
                        placeholder="johndoe"
                        maxlength={50}
                        disabled={loading}
                        class="w-full rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50"
                    />
                    <p class="mt-1 text-xs text-muted-foreground">
                        Leave blank to use your email prefix
                    </p>
                </div>

                <!-- Password -->
                <div class="mb-4">
                    <label
                        for="password"
                        class="block text-sm font-medium text-foreground mb-2"
                    >
                        Password
                    </label>
                    <input
                        id="password"
                        type="password"
                        autocomplete="new-password"
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
                    <p class="mt-1 text-xs text-muted-foreground">
                        Minimum 12 characters
                    </p>
                </div>

                <!-- Confirm Password -->
                <div class="mb-6">
                    <label
                        for="confirmPassword"
                        class="block text-sm font-medium text-foreground mb-2"
                    >
                        Confirm Password
                    </label>
                    <input
                        id="confirmPassword"
                        type="password"
                        autocomplete="new-password"
                        bind:value={confirmPassword}
                        placeholder="••••••••"
                        disabled={loading}
                        aria-invalid={!!fieldErrors.confirmPassword}
                        aria-describedby={fieldErrors.confirmPassword
                            ? "confirmPassword-error"
                            : undefined}
                        class="w-full rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50 {fieldErrors.confirmPassword
                            ? 'border-destructive'
                            : ''}"
                    />
                    {#if fieldErrors.confirmPassword}<p
                            id="confirmPassword-error"
                            class="mt-1 text-xs text-destructive"
                        >
                            {fieldErrors.confirmPassword}
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
                    {loading ? "Creating Account..." : "Create Admin Account"}
                </button>
            </form>
        </div>

        <!-- Footer -->
        <p class="mt-6 text-center text-xs text-muted-foreground">
            Watchflare Host Monitoring
        </p>
    </div>
</div>
