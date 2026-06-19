<script lang="ts">
    import { changePassword, changeEmail, changeUsername, disableTOTP, regenerateBackupCodes } from "$lib/api";
    import { changePasswordSchema, validateForm } from "$lib/validation";
    import { userStore } from "$lib/stores/user";
    import { Eye, EyeOff, Copy, Check, Shield, ShieldCheck } from "lucide-svelte";
    import TwoFactorSetupModal from '$lib/components/TwoFactorSetupModal.svelte';
    import { lockBodyScroll } from "$lib/actions/lockBodyScroll";

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

    let showSetupModal = $state(false);
    let showDisableConfirm = $state(false);
    let showRegenModal = $state(false);
    let totpVerifyCode = $state('');
    let twoFAError = $state('');
    let twoFALoading = $state(false);
    let regenCodes: string[] = $state([]);
    let regenDone = $state(false);
    let regenCopied = $state(false);
    let regenSaved = $state(false);

    async function handleDisableTOTP() {
        twoFAError = '';
        twoFALoading = true;
        try {
            await disableTOTP(totpVerifyCode);
            await userStore.load();
            showDisableConfirm = false;
            totpVerifyCode = '';
        } catch {
            twoFAError = 'Invalid code. Please try again.';
        } finally {
            twoFALoading = false;
        }
    }

    async function handleRegenCodes() {
        twoFAError = '';
        twoFALoading = true;
        try {
            const res = await regenerateBackupCodes(totpVerifyCode);
            regenCodes = res.backup_codes;
            regenDone = true;
            totpVerifyCode = '';
        } catch {
            twoFAError = 'Invalid code. Please try again.';
        } finally {
            twoFALoading = false;
        }
    }

    function openDisableConfirm() {
        totpVerifyCode = '';
        twoFAError = '';
        showDisableConfirm = true;
    }

    async function copyRegenCodes() {
        await navigator.clipboard.writeText(regenCodes.join("\n"));
        regenCopied = true;
        setTimeout(() => { regenCopied = false; }, 2000);
    }

    function openRegenModal() {
        totpVerifyCode = '';
        twoFAError = '';
        regenCodes = [];
        regenDone = false;
        regenCopied = false;
        regenSaved = false;
        showRegenModal = true;
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

    <!-- 2FA Section -->
    <div class="rounded-lg border bg-card">
        <div class="border-b px-6 py-4">
            <h2 class="text-base font-semibold text-foreground">Two-Factor Authentication</h2>
            <p class="text-sm text-muted-foreground mt-0.5">
                Add an extra layer of security to your account.
            </p>
        </div>
        <div class="px-6 py-5 flex items-center justify-between gap-4 flex-wrap">
            {#if $userStore.user?.totp_enabled}
                <div class="flex items-center gap-2">
                    <ShieldCheck class="h-5 w-5 text-green-600" />
                    <span class="text-sm font-medium text-foreground">2FA is enabled</span>
                </div>
                <div class="flex items-center gap-2">
                    <button
                        onclick={openRegenModal}
                        class="rounded-lg border px-3 py-1.5 text-sm font-medium text-foreground transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-primary"
                    >
                        Regenerate backup codes
                    </button>
                    <button
                        onclick={openDisableConfirm}
                        class="rounded-lg border border-destructive px-3 py-1.5 text-sm font-medium text-destructive transition-colors hover:bg-destructive/10 focus-visible:ring-2 focus-visible:ring-destructive"
                    >
                        Disable 2FA
                    </button>
                </div>
            {:else}
                <div class="flex items-center gap-2">
                    <Shield class="h-5 w-5 text-muted-foreground" />
                    <span class="text-sm text-muted-foreground">2FA is disabled</span>
                </div>
                <button
                    onclick={() => { showSetupModal = true; }}
                    class="rounded-lg bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-primary"
                >
                    Enable 2FA
                </button>
            {/if}
        </div>
    </div>

</div>

<TwoFactorSetupModal
    open={showSetupModal}
    onClose={() => { showSetupModal = false; }}
    onEnabled={() => { showSetupModal = false; userStore.load(); }}
/>

{#if showDisableConfirm}
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div
        role="presentation"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
        onclick={() => { showDisableConfirm = false; twoFAError = ''; }}
        use:lockBodyScroll
    >
        <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
        <div
            role="dialog"
            aria-modal="true"
            tabindex="-1"
            onclick={(e) => e.stopPropagation()}
            class="w-full max-w-sm rounded-lg border bg-card p-6 shadow-lg"
        >
            <h3 class="text-base font-semibold text-foreground mb-2">Disable 2FA</h3>
            <p class="text-sm text-muted-foreground mb-4">Enter your authenticator code to confirm.</p>
            <input
                type="text"
                inputmode="numeric"
                bind:value={totpVerifyCode}
                placeholder="000000"
                maxlength={6}
                autofocus
                class="w-full rounded-lg border bg-background px-3 py-2 text-sm mb-3 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary text-center tracking-widest text-lg"
            />
            {#if twoFAError}
                <p class="text-xs text-destructive mb-3">{twoFAError}</p>
            {/if}
            <div class="flex gap-2">
                <button
                    onclick={() => { showDisableConfirm = false; twoFAError = ''; }}
                    class="flex-1 rounded-lg border px-4 py-2 text-sm font-medium hover:bg-muted"
                >Cancel</button>
                <button
                    onclick={handleDisableTOTP}
                    disabled={twoFALoading || totpVerifyCode.length < 6}
                    class="flex-1 rounded-lg bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground hover:bg-destructive/90 disabled:opacity-50"
                >
                    {twoFALoading ? 'Disabling...' : 'Disable 2FA'}
                </button>
            </div>
        </div>
    </div>
{/if}

{#if showRegenModal}
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div
        role="presentation"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
        onclick={() => { if (!regenDone) { showRegenModal = false; twoFAError = ''; } }}
        use:lockBodyScroll
    >
        <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
        <div
            role="dialog"
            aria-modal="true"
            tabindex="-1"
            onclick={(e) => e.stopPropagation()}
            class="w-full max-w-sm rounded-lg border bg-card p-6 shadow-lg"
        >
            <h3 class="text-base font-semibold text-foreground mb-2">Regenerate Backup Codes</h3>
            {#if !regenDone}
                <p class="text-sm text-muted-foreground mb-4">Enter your authenticator code to generate new backup codes. Old codes will be invalidated.</p>
                <input
                    type="text"
                    inputmode="numeric"
                    bind:value={totpVerifyCode}
                    placeholder="000000"
                    maxlength={6}
                    autofocus
                    class="w-full rounded-lg border bg-background px-3 py-2 text-sm mb-3 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary text-center tracking-widest text-lg"
                />
                {#if twoFAError}
                    <p class="text-xs text-destructive mb-3">{twoFAError}</p>
                {/if}
                <div class="flex gap-2">
                    <button onclick={() => { showRegenModal = false; twoFAError = ''; }} class="flex-1 rounded-lg border px-4 py-2 text-sm font-medium hover:bg-muted">Cancel</button>
                    <button
                        onclick={handleRegenCodes}
                        disabled={twoFALoading || totpVerifyCode.length < 6}
                        class="flex-1 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
                    >
                        {twoFALoading ? 'Generating...' : 'Regenerate'}
                    </button>
                </div>
            {:else}
                <p class="text-sm text-muted-foreground mb-4">Your new backup codes (save them now — they won't be shown again):</p>
                <div class="rounded-lg border bg-muted/50 p-4 mb-4">
                    <div class="grid grid-cols-2 gap-2 mb-3">
                        {#each regenCodes as rc}
                            <code class="text-xs font-mono text-foreground tracking-wider">{rc}</code>
                        {/each}
                    </div>
                    <button
                        onclick={copyRegenCodes}
                        class="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary rounded"
                    >
                        {#if regenCopied}
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
                        bind:checked={regenSaved}
                        class="mt-0.5 rounded border-border"
                    />
                    <span class="text-sm text-foreground">I have saved my backup codes</span>
                </label>
                <button
                    onclick={() => { showRegenModal = false; }}
                    disabled={!regenSaved}
                    class="w-full rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
                >Done</button>
            {/if}
        </div>
    </div>
{/if}
