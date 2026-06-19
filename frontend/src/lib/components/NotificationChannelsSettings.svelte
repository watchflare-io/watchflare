<script lang="ts">
	import { onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import { Plus, Pencil, Trash2, Send, Loader, TriangleAlert, Check, X } from 'lucide-svelte';
	import Toggle from '$lib/components/ui/Toggle.svelte';
	import RightSidebar from '$lib/components/RightSidebar.svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import type { NotificationChannel } from '$lib/types';
	import {
		ApiError,
		listNotificationChannels,
		createNotificationChannel,
		updateNotificationChannel,
		deleteNotificationChannel,
		testNotificationChannel,
		testNotificationChannelDraft
	} from '$lib/api';

	let loaded = $state(false);
	let channels = $state<NotificationChannel[]>([]);

	let drawerOpen = $state(false);
	let editingChannel = $state<NotificationChannel | null>(null);
	let formName = $state('');
	let formUrl = $state('');
	let formEnabled = $state(true);
	let formNameError = $state('');
	let formUrlError = $state('');
	let saving = $state(false);

	let drawerTesting = $state(false);
	let drawerTestMessage = $state<{ ok: boolean; text: string } | null>(null);

	let testingIds = $state<Set<string>>(new Set());
	let testMessages = $state<Record<string, { ok: boolean; text: string }>>({});
	let deletingIds = $state<Set<string>>(new Set());

	let pendingDelete = $state<NotificationChannel | null>(null);
	let toast = $state<{ text: string; ok: boolean } | null>(null);
	let toastTimer: ReturnType<typeof setTimeout> | undefined;

	// Must stay in sync with testCooldownPeriod in backend/handlers/notification_channels.go.
	const COOLDOWN_MS = 5_000;
	let cooldownEndsAt = $state<Record<string, number>>({});
	let now = $state(Date.now());

	$effect(() => {
		const active = Object.values(cooldownEndsAt).some((t) => t > now);
		if (!active) return;
		const id = setInterval(() => {
			now = Date.now();
		}, 250);
		return () => clearInterval(id);
	});

	function cooldownSecondsLeft(key: string): number {
		const ends = cooldownEndsAt[key];
		if (!ends) return 0;
		return Math.max(0, Math.ceil((ends - now) / 1000));
	}

	function startCooldown(key: string, seconds = COOLDOWN_MS / 1000) {
		// Refresh `now` so the first render after startCooldown uses a fresh
		// clock; the interval tick only catches up after 250ms.
		now = Date.now();
		cooldownEndsAt = {
			...cooldownEndsAt,
			[key]: now + seconds * 1000
		};
	}

	function extractRetryAfter(err: unknown): number | null {
		if (err instanceof ApiError) {
			const v = (err.data as { retry_after_seconds?: number } | undefined)?.retry_after_seconds;
			if (typeof v === 'number') return v;
		}
		return null;
	}

	// Mirrors the backend cooldown granularity: per-URL for draft tests, per-id
	// for saved channels. Empty when neither applies (button is disabled then).
	function drawerTestKey(): string {
		const url = formUrl.trim();
		if (url) return `draft:${url}`;
		if (editingChannel) return editingChannel.id;
		return '';
	}

	const drawerCooldownLeft = $derived(drawerTestKey() ? cooldownSecondsLeft(drawerTestKey()) : 0);

	const testDisabled = $derived(
		drawerTesting || drawerCooldownLeft > 0 || (formUrl.trim().length === 0 && !editingChannel)
	);

	const SERVICE_LABELS: Record<string, string> = {
		discord: 'Discord',
		slack: 'Slack',
		telegram: 'Telegram',
		smtp: 'Email',
		smtps: 'Email',
		matrix: 'Matrix',
		ntfy: 'Ntfy',
		gotify: 'Gotify',
		pushover: 'Pushover',
		pushbullet: 'Pushbullet',
		teams: 'Teams',
		mattermost: 'Mattermost',
		rocketchat: 'Rocket.Chat',
		zulip: 'Zulip',
		generic: 'Generic webhook'
	};

	const URL_FORMAT_HINTS = [
		'discord://TOKEN@WEBHOOK_ID',
		'slack://hook:T_A-T_B-T_C@webhook',
		'telegram://BOT_TOKEN@telegram?chats=@CHANNEL',
		'smtp://USER:PASS@HOST:PORT/?fromAddress=from@x.com&toAddresses=to@x.com'
	];

	function serviceFromMaskedUrl(masked: string): string {
		const scheme = masked.split('://')[0]?.toLowerCase() ?? '';
		const label = SERVICE_LABELS[scheme];
		if (label) return label;
		return scheme ? scheme.toUpperCase() : 'Unknown';
	}

	function showToast(text: string, ok = false) {
		toast = { text, ok };
		clearTimeout(toastTimer);
		toastTimer = setTimeout(() => {
			toast = null;
		}, 5000);
	}

	function dismissToast() {
		clearTimeout(toastTimer);
		toast = null;
	}

	function resetForm() {
		formName = '';
		formUrl = '';
		formEnabled = true;
		formNameError = '';
		formUrlError = '';
		drawerTestMessage = null;
		editingChannel = null;
	}

	function openAddDrawer() {
		resetForm();
		drawerOpen = true;
	}

	function openEditDrawer(channel: NotificationChannel) {
		resetForm();
		editingChannel = channel;
		formName = channel.name;
		formUrl = '';
		formEnabled = channel.enabled;
		drawerOpen = true;
	}

	function closeDrawer() {
		drawerOpen = false;
	}

	onMount(async () => {
		try {
			const res = await listNotificationChannels();
			channels = res.channels;
		} catch {
			// Non-fatal: leave list empty so the user can still add channels.
		}
		loaded = true;
	});

	function validateForm(): boolean {
		formNameError = '';
		formUrlError = '';
		const name = formName.trim();
		const url = formUrl.trim();
		if (!name) {
			formNameError = 'Name is required';
		} else if (name.length > 100) {
			formNameError = 'Maximum 100 characters';
		}
		if (!editingChannel && !url) {
			formUrlError = 'URL is required';
		}
		return !formNameError && !formUrlError;
	}

	async function handleSave() {
		if (!validateForm()) return;
		saving = true;
		try {
			if (editingChannel) {
				await updateNotificationChannel(editingChannel.id, {
					name: formName.trim(),
					url: formUrl.trim(),
					enabled: formEnabled
				});
				const fresh = await listNotificationChannels();
				channels = fresh.channels;
			} else {
				const res = await createNotificationChannel({
					name: formName.trim(),
					url: formUrl.trim(),
					enabled: formEnabled
				});
				channels = [...channels, res.channel];
			}
			closeDrawer();
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Save failed.';
			showToast(msg);
		} finally {
			saving = false;
		}
	}

	async function handleDrawerTest() {
		const draftUrl = formUrl.trim();
		if (!draftUrl && !editingChannel) {
			drawerTestMessage = {
				ok: false,
				text: 'Enter a URL to test.'
			};
			return;
		}
		const key = drawerTestKey();
		drawerTestMessage = null;
		drawerTesting = true;
		try {
			const res = draftUrl
				? await testNotificationChannelDraft(draftUrl)
				: await testNotificationChannel(editingChannel!.id);
			drawerTestMessage = { ok: true, text: res.message };
			startCooldown(key);
		} catch (err) {
			const text = err instanceof Error ? err.message : 'Test failed.';
			drawerTestMessage = { ok: false, text };
			const retry = extractRetryAfter(err);
			if (retry !== null) startCooldown(key, retry);
		} finally {
			drawerTesting = false;
		}
	}

	async function handleRowTest(channel: NotificationChannel) {
		testingIds = new Set([...testingIds, channel.id]);
		testMessages = { ...testMessages, [channel.id]: { ok: false, text: '' } };
		try {
			const res = await testNotificationChannel(channel.id);
			testMessages = {
				...testMessages,
				[channel.id]: { ok: true, text: res.message }
			};
			startCooldown(channel.id);
		} catch (err) {
			const text = err instanceof Error ? err.message : 'Test failed.';
			testMessages = {
				...testMessages,
				[channel.id]: { ok: false, text }
			};
			const retry = extractRetryAfter(err);
			if (retry !== null) startCooldown(channel.id, retry);
		} finally {
			const next = new Set(testingIds);
			next.delete(channel.id);
			testingIds = next;
		}
	}

	async function handleToggle(channel: NotificationChannel, value: boolean) {
		try {
			await updateNotificationChannel(channel.id, { enabled: value });
		} catch {
			channel.enabled = !value;
			showToast('Failed to update channel.');
		}
	}

	function askDelete(channel: NotificationChannel) {
		pendingDelete = channel;
	}

	async function confirmDelete() {
		if (!pendingDelete) return;
		const id = pendingDelete.id;
		pendingDelete = null;
		deletingIds = new Set([...deletingIds, id]);
		try {
			await deleteNotificationChannel(id);
			channels = channels.filter((c) => c.id !== id);
			const { [id]: _drop, ...rest } = testMessages;
			testMessages = rest;
		} catch (err) {
			const msg = err instanceof Error ? err.message : 'Delete failed.';
			showToast(msg);
		} finally {
			const next = new Set(deletingIds);
			next.delete(id);
			deletingIds = next;
		}
	}
</script>

<div
	class="rounded-lg border bg-card p-4 sm:p-6 mb-6 transition-opacity duration-200 {loaded
		? 'opacity-100'
		: 'opacity-0'}"
>
	<div class="flex items-start justify-between gap-4 mb-1">
		<div>
			<h2 class="text-lg font-semibold text-foreground mb-1">Notification channels</h2>
			<p class="text-sm text-muted-foreground">
				Send alerts to Discord, Slack, Telegram, email and many more via Shoutrrr.
			</p>
		</div>
		<button
			type="button"
			onclick={openAddDrawer}
			class="flex items-center gap-1.5 rounded-lg bg-primary px-3 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 shrink-0"
		>
			<Plus class="h-4 w-4" />
			Add channel
		</button>
	</div>

	{#if channels.length > 0}
		<div class="mt-5 overflow-x-auto">
			<table class="w-full text-sm">
				<thead class="bg-table-header text-xs uppercase tracking-wide text-muted-foreground">
					<tr>
						<th class="px-3 py-2 text-left font-medium whitespace-nowrap">Name</th>
						<th class="px-3 py-2 text-left font-medium whitespace-nowrap">Service</th>
						<th class="px-3 py-2 text-left font-medium whitespace-nowrap">Status</th>
						<th class="px-3 py-2 text-right font-medium whitespace-nowrap">Actions</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-border">
					{#each channels as channel (channel.id)}
						{@const isTesting = testingIds.has(channel.id)}
						{@const isDeleting = deletingIds.has(channel.id)}
						{@const result = testMessages[channel.id]}
						{@const cooldown = cooldownSecondsLeft(channel.id)}
						<tr
							class="transition-opacity duration-200 {channel.enabled
								? 'opacity-100'
								: 'opacity-60'}"
						>
							<td class="px-3 py-2 whitespace-nowrap font-medium text-foreground">{channel.name}</td
							>
							<td class="px-3 py-2 whitespace-nowrap">
								<span class="rounded-md bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">
									{serviceFromMaskedUrl(channel.url_masked)}
								</span>
							</td>
							<td class="px-3 py-2 whitespace-nowrap">
								<Toggle
									bind:checked={channel.enabled}
									size="sm"
									aria-label="Enable {channel.name}"
									onchange={(value) => handleToggle(channel, value)}
								/>
							</td>
							<td class="px-3 py-2 whitespace-nowrap text-right">
								<div class="inline-flex items-center gap-1">
									<button
										type="button"
										onclick={() => handleRowTest(channel)}
										disabled={isTesting || cooldown > 0}
										title={cooldown > 0
											? `Cooldown active, retry in ${cooldown}s`
											: 'Send a test notification'}
										aria-label="Test {channel.name}"
										class="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:opacity-50 disabled:cursor-not-allowed"
									>
										{#if isTesting}
											<Loader class="h-4 w-4 animate-spin" />
										{:else}
											<Send class="h-4 w-4" />
										{/if}
									</button>
									<button
										type="button"
										onclick={() => openEditDrawer(channel)}
										title="Edit channel"
										aria-label="Edit {channel.name}"
										class="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
									>
										<Pencil class="h-4 w-4" />
									</button>
									<button
										type="button"
										onclick={() => askDelete(channel)}
										disabled={isDeleting}
										title="Delete channel"
										aria-label="Delete {channel.name}"
										class="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive disabled:opacity-50 disabled:cursor-not-allowed"
									>
										{#if isDeleting}
											<Loader class="h-4 w-4 animate-spin" />
										{:else}
											<Trash2 class="h-4 w-4" />
										{/if}
									</button>
								</div>
							</td>
						</tr>
						{#if result?.text}
							<tr>
								<td
									colspan="4"
									class="px-3 pb-2 text-xs {result.ok ? 'text-success' : 'text-destructive'}"
								>
									{#if result.ok}
										<Check class="h-3 w-3 inline mr-1" />
									{/if}
									{result.text}
								</td>
							</tr>
						{/if}
					{/each}
				</tbody>
			</table>
		</div>
	{:else if loaded}
		<p class="mt-5 text-sm text-muted-foreground">
			No notification channels yet. Click <span class="font-medium text-foreground"
				>Add channel</span
			> to create one.
		</p>
	{/if}
</div>

<RightSidebar open={drawerOpen} onClose={closeDrawer} size="wide">
	<div class="flex items-center justify-between px-5 py-4 border-b border-border">
		<h3 class="text-base font-semibold text-foreground">
			{editingChannel ? 'Edit channel' : 'Add channel'}
		</h3>
		<button
			type="button"
			onclick={closeDrawer}
			aria-label="Close"
			class="rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
		>
			<X class="h-4 w-4" />
		</button>
	</div>

	<div class="flex-1 overflow-y-auto px-5 py-4">
		<div class="mb-4">
			<label for="channel-name" class="block text-sm font-medium text-foreground mb-1">
				Name<span class="text-destructive ml-0.5">*</span>
			</label>
			<input
				id="channel-name"
				type="text"
				placeholder="Ops Discord"
				bind:value={formName}
				disabled={saving}
				class="w-full rounded-lg border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 disabled:opacity-50 {formNameError
					? 'border-destructive focus-visible:ring-destructive'
					: 'focus-visible:ring-primary'}"
			/>
			{#if formNameError}
				<p class="mt-1 text-xs text-destructive">{formNameError}</p>
			{/if}
		</div>

		<div class="mb-4">
			<label for="channel-url" class="block text-sm font-medium text-foreground mb-1">
				Shoutrrr URL{#if !editingChannel}<span class="text-destructive ml-0.5">*</span>{/if}
			</label>
			{#if editingChannel}
				<p class="mb-1.5 text-xs text-muted-foreground">
					Current: <span class="font-mono text-foreground">{editingChannel.url_masked}</span>
				</p>
			{/if}
			<textarea
				id="channel-url"
				rows="3"
				placeholder={editingChannel
					? 'Leave empty to keep the current URL'
					: 'discord://TOKEN@WEBHOOK_ID'}
				bind:value={formUrl}
				disabled={saving}
				class="w-full rounded-lg border bg-background px-3 py-2 text-sm text-foreground font-mono placeholder:text-muted-foreground placeholder:font-mono focus:outline-none focus-visible:ring-2 disabled:opacity-50 {formUrlError
					? 'border-destructive focus-visible:ring-destructive'
					: 'focus-visible:ring-primary'}"></textarea>
			{#if formUrlError}
				<p class="mt-1 text-xs text-destructive">{formUrlError}</p>
			{:else}
				<details class="mt-1.5">
					<summary
						class="text-xs text-muted-foreground cursor-pointer hover:text-foreground transition-colors"
					>
						See URL format examples
					</summary>
					<ul class="mt-2 space-y-1 text-xs text-muted-foreground font-mono">
						{#each URL_FORMAT_HINTS as hint}
							<li class="truncate" title={hint}>{hint}</li>
						{/each}
					</ul>
					<p class="mt-2 text-xs text-muted-foreground">
						Full list: <a
							href="https://shoutrrr.nickfedor.com/services/overview/"
							target="_blank"
							rel="noopener noreferrer"
							class="text-primary underline">Shoutrrr documentation</a
						>
					</p>
				</details>
			{/if}
		</div>

		<div class="mb-4 flex items-center gap-3">
			<Toggle bind:checked={formEnabled} aria-labelledby="channel-enabled-label" />
			<div>
				<p id="channel-enabled-label" class="text-sm font-medium text-foreground">Enabled</p>
				<p class="text-xs text-muted-foreground">
					Disable to keep the channel without firing notifications.
				</p>
			</div>
		</div>

		{#if drawerTestMessage}
			<div
				class="mt-4 rounded-lg border px-3 py-2 text-sm {drawerTestMessage.ok
					? 'border-success/40 bg-success/10 text-success'
					: 'border-destructive/40 bg-destructive/10 text-destructive'}"
			>
				{#if drawerTestMessage.ok}
					<Check class="h-4 w-4 inline mr-1" />
				{/if}
				{drawerTestMessage.text}
			</div>
		{/if}
	</div>

	<div class="border-t border-border px-5 py-3 flex items-center justify-between gap-2">
		<button
			type="button"
			onclick={handleDrawerTest}
			disabled={testDisabled}
			title={drawerCooldownLeft > 0
				? `Cooldown active, retry in ${drawerCooldownLeft}s`
				: testDisabled
					? 'Enter a URL to test'
					: 'Send a test notification'}
			class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-2 text-sm font-medium transition-colors hover:bg-muted disabled:opacity-50 disabled:cursor-not-allowed"
		>
			{#if drawerTesting}
				<Loader class="h-4 w-4 animate-spin" />
			{:else}
				<Send class="h-4 w-4" />
			{/if}
			{drawerCooldownLeft > 0 ? `Test (${drawerCooldownLeft}s)` : 'Test'}
		</button>
		<div class="flex items-center gap-2">
			<button
				type="button"
				onclick={closeDrawer}
				disabled={saving}
				class="rounded-lg border border-border px-3 py-2 text-sm font-medium transition-colors hover:bg-muted disabled:opacity-50 disabled:cursor-not-allowed"
			>
				Cancel
			</button>
			<button
				type="button"
				onclick={handleSave}
				disabled={saving}
				class="flex items-center gap-1.5 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
			>
				{#if saving}
					<Loader class="h-4 w-4 animate-spin" />
				{/if}
				{editingChannel ? 'Save' : 'Create'}
			</button>
		</div>
	</div>
</RightSidebar>

<ConfirmDialog
	open={!!pendingDelete}
	title="Delete notification channel?"
	confirmLabel="Delete"
	confirmVariant="destructive"
	onConfirm={confirmDelete}
	onClose={() => {
		pendingDelete = null;
	}}
>
	<p class="text-sm text-muted-foreground">
		{#if pendingDelete}
			<span class="font-medium text-foreground">{pendingDelete.name}</span> will be removed. Future alerts
			will no longer be sent there.
		{/if}
	</p>
</ConfirmDialog>

{#if toast}
	<div
		transition:fly={{ y: 16, duration: 200 }}
		role="alert"
		class="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 flex items-center gap-3 rounded-lg border bg-background px-4 py-3 text-sm shadow-lg w-max max-w-[calc(100vw-2rem)] {toast.ok
			? 'border-success/40 text-success'
			: 'border-destructive/40 text-destructive'}"
	>
		{#if toast.ok}
			<Check class="h-4 w-4 shrink-0" />
		{:else}
			<TriangleAlert class="h-4 w-4 shrink-0" />
		{/if}
		<span>{toast.text}</span>
		<button
			type="button"
			onclick={dismissToast}
			class="ml-1 rounded p-0.5 hover:bg-muted transition-colors"
			aria-label="Dismiss"
		>
			<X class="h-3.5 w-3.5" />
		</button>
	</div>
{/if}
