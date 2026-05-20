<script lang="ts">
    import { changePassword, changeEmail, changeUsername } from "$lib/api";
    import { changePasswordSchema, validateForm } from "$lib/validation";
    import { userStore } from "$lib/stores/user";
    import { Eye, EyeOff } from "lucide-svelte";

    // Username form state
    let usernameOverride = $state<string | null>(null);
    let usernameError = $state("");
    let usernameSuccess = $state("");
    let usernameLoading = $state(false);

    const editUsername = $derived(usernameOverride ?? ($userStore.user?.username || ""));
    const usernameDirty = $derived(
        usernameOverride !== null && usernameOverride !== ($userStore.user?.username || ""),
    );

    async function handleChangeUsername() {
        usernameError = "";
        usernameSuccess = "";
        usernameLoading = true;
        try {
            await changeUsername(editUsername);
            await userStore.load();
            usernameOverride = null;
            usernameSuccess = "Username updated successfully!";
        } catch (err: unknown) {
            usernameError = err instanceof Error ? err.message : "Failed to update username";
        } finally {
            usernameLoading = false;
        }
    }

    // Email form state
    let emailOverride = $state<string | null>(null);
    let emailError = $state("");
    let emailSuccess = $state("");
    let emailLoading = $state(false);

    const editEmail = $derived(emailOverride ?? ($userStore.user?.email || ""));
    const emailDirty = $derived(
        emailOverride !== null && emailOverride !== ($userStore.user?.email || ""),
    );

    // Password form state
    let currentPassword = $state("");
    let newPassword = $state("");
    let confirmPassword = $state("");
    let error = $state("");
    let fieldErrors: Record<string, string> = $state({});
    let success = $state("");
    let loading = $state(false);

    // Password visibility toggles
    let showCurrentPassword = $state(false);
    let showNewPassword = $state(false);
    let showConfirmPassword = $state(false);

    async function handleChangeEmail() {
        emailError = "";
        emailSuccess = "";

        if (!editEmail || !editEmail.includes("@")) {
            emailError = "Please enter a valid email address.";
            return;
        }

        emailLoading = true;

        try {
            await changeEmail(editEmail);
            emailOverride = null;
            await userStore.load();
            emailSuccess = "Email updated successfully!";
        } catch (err: unknown) {
            emailError =
                err instanceof Error
                    ? err.message
                    : "Failed to update email";
        } finally {
            emailLoading = false;
        }
    }

    async function handleChangePassword() {
        error = "";
        fieldErrors = {};
        success = "";

        const result = validateForm(changePasswordSchema, {
            currentPassword,
            newPassword,
            confirmPassword,
        });
        if (!result.success) {
            fieldErrors = result.errors;
            return;
        }

        loading = true;

        try {
            await changePassword(currentPassword, newPassword);
            success = "Password changed successfully!";
            currentPassword = "";
            newPassword = "";
            confirmPassword = "";
            showCurrentPassword = false;
            showNewPassword = false;
            showConfirmPassword = false;
        } catch (err: unknown) {
            const message =
                err instanceof Error
                    ? err.message
                    : "Failed to change password";
            error =
                message === "current password is incorrect"
                    ? "Current password is incorrect."
                    : message;
        } finally {
            loading = false;
        }
    }
</script>

<svelte:head>
    <title>Account - Watchflare</title>
</svelte:head>

<div class="max-w-2xl space-y-6">

<!-- Header -->
<div class="mb-6">
    <h1 class="text-xl sm:text-2xl font-semibold text-foreground">
        Account
    </h1>
    <p class="text-sm text-muted-foreground mt-1">
        Manage your profile and credentials
    </p>
</div>

    <!-- Profile Card -->
    <div class="rounded-lg border bg-card p-4 sm:p-6">
        <h2 class="text-lg font-semibold text-foreground mb-6">Profile</h2>

        <!-- Username row -->
        <div class="mb-5">
            <label for="username" class="block text-sm font-medium text-foreground mb-2">
                Username
            </label>
            <form
                onsubmit={(e) => { e.preventDefault(); handleChangeUsername(); }}
                class="flex flex-col sm:flex-row gap-2"
            >
                <input
                    id="username"
                    type="text"
                    value={editUsername}
                    oninput={(e) => { usernameOverride = (e.target as HTMLInputElement).value; }}
                    placeholder="Enter a username"
                    maxlength={50}
                    disabled={usernameLoading}
                    class="flex-1 rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50"
                />
                <button
                    type="submit"
                    disabled={usernameLoading || !usernameDirty}
                    class="self-start rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                    {usernameLoading ? "Saving..." : "Save"}
                </button>
            </form>
            {#if usernameError}
                <p class="mt-1.5 text-xs text-destructive">{usernameError}</p>
            {/if}
            {#if usernameSuccess}
                <p class="mt-1.5 text-xs text-success">{usernameSuccess}</p>
            {/if}
        </div>

        <!-- Email row -->
        <div>
            <label for="email" class="block text-sm font-medium text-foreground mb-2">
                Email address
            </label>
            <form
                onsubmit={(e) => { e.preventDefault(); handleChangeEmail(); }}
                class="flex flex-col sm:flex-row gap-2"
            >
                <input
                    id="email"
                    type="email"
                    value={editEmail}
                    oninput={(e) => { emailOverride = (e.target as HTMLInputElement).value; }}
                    placeholder="Enter email address"
                    disabled={emailLoading}
                    class="flex-1 rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50"
                />
                <button
                    type="submit"
                    disabled={emailLoading || !emailDirty}
                    class="self-start rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                    {emailLoading ? "Saving..." : "Save"}
                </button>
            </form>
            {#if emailError}
                <p class="mt-1.5 text-xs text-destructive">{emailError}</p>
            {/if}
            {#if emailSuccess}
                <p class="mt-1.5 text-xs text-success">{emailSuccess}</p>
            {/if}
        </div>
    </div>

    <!-- Password Card -->
    <div class="rounded-lg border bg-card p-4 sm:p-6">
        <h2 class="text-lg font-semibold text-foreground mb-6">
            Change Password
        </h2>

        <form
            onsubmit={(e) => {
                e.preventDefault();
                handleChangePassword();
            }}
        >
            <div class="mb-4">
                <label
                    for="current-password"
                    class="block text-sm font-medium text-foreground mb-2"
                >
                    Current Password
                </label>
                <div class="relative">
                    <input
                        id="current-password"
                        type={showCurrentPassword ? "text" : "password"}
                        bind:value={currentPassword}
                        placeholder="Enter current password"
                        disabled={loading}
                        aria-invalid={!!fieldErrors.currentPassword}
                        aria-describedby={fieldErrors.currentPassword
                            ? "currentPassword-error"
                            : undefined}
                        class="w-full rounded-lg border bg-background px-3 py-2 pr-10 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50 {fieldErrors.currentPassword
                            ? 'border-destructive focus-visible:ring-destructive'
                            : ''}"
                    />
                    <button
                        type="button"
                        onclick={() => (showCurrentPassword = !showCurrentPassword)}
                        class="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                        tabindex={-1}
                    >
                        {#if showCurrentPassword}
                            <EyeOff class="h-4 w-4" />
                        {:else}
                            <Eye class="h-4 w-4" />
                        {/if}
                    </button>
                </div>
                {#if fieldErrors.currentPassword}
                    <p id="currentPassword-error" class="mt-1 text-xs text-destructive">
                        {fieldErrors.currentPassword}
                    </p>
                {/if}
            </div>

            <div class="mb-4">
                <label
                    for="new-password"
                    class="block text-sm font-medium text-foreground mb-2"
                >
                    New Password
                </label>
                <div class="relative">
                    <input
                        id="new-password"
                        type={showNewPassword ? "text" : "password"}
                        bind:value={newPassword}
                        placeholder="Enter new password (min 12 characters)"
                        disabled={loading}
                        aria-invalid={!!fieldErrors.newPassword}
                        aria-describedby={fieldErrors.newPassword
                            ? "newPassword-error"
                            : undefined}
                        class="w-full rounded-lg border bg-background px-3 py-2 pr-10 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50 {fieldErrors.newPassword
                            ? 'border-destructive focus-visible:ring-destructive'
                            : ''}"
                    />
                    <button
                        type="button"
                        onclick={() => (showNewPassword = !showNewPassword)}
                        class="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                        tabindex={-1}
                    >
                        {#if showNewPassword}
                            <EyeOff class="h-4 w-4" />
                        {:else}
                            <Eye class="h-4 w-4" />
                        {/if}
                    </button>
                </div>
                {#if fieldErrors.newPassword}
                    <p id="newPassword-error" class="mt-1 text-xs text-destructive">
                        {fieldErrors.newPassword}
                    </p>
                {/if}
            </div>

            <div class="mb-4">
                <label
                    for="confirm-password"
                    class="block text-sm font-medium text-foreground mb-2"
                >
                    Confirm New Password
                </label>
                <div class="relative">
                    <input
                        id="confirm-password"
                        type={showConfirmPassword ? "text" : "password"}
                        bind:value={confirmPassword}
                        placeholder="Confirm new password"
                        disabled={loading}
                        aria-invalid={!!fieldErrors.confirmPassword}
                        aria-describedby={fieldErrors.confirmPassword
                            ? "confirmPassword-error"
                            : undefined}
                        class="w-full rounded-lg border bg-background px-3 py-2 pr-10 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:opacity-50 {fieldErrors.confirmPassword
                            ? 'border-destructive focus-visible:ring-destructive'
                            : ''}"
                    />
                    <button
                        type="button"
                        onclick={() => (showConfirmPassword = !showConfirmPassword)}
                        class="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                        tabindex={-1}
                    >
                        {#if showConfirmPassword}
                            <EyeOff class="h-4 w-4" />
                        {:else}
                            <Eye class="h-4 w-4" />
                        {/if}
                    </button>
                </div>
                {#if fieldErrors.confirmPassword}
                    <p id="confirmPassword-error" class="mt-1 text-xs text-destructive">
                        {fieldErrors.confirmPassword}
                    </p>
                {/if}
            </div>

            {#if error}
                <div class="mb-4 rounded-lg border border-destructive bg-destructive/10 p-3">
                    <p class="text-sm text-destructive">{error}</p>
                </div>
            {/if}

            {#if success}
                <div class="mb-4 rounded-lg border border-success bg-success/10 p-3">
                    <p class="text-sm text-success">{success}</p>
                </div>
            {/if}

            <button
                type="submit"
                disabled={loading}
                class="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
            >
                {loading ? "Changing Password..." : "Change Password"}
            </button>
        </form>
    </div>

</div>
