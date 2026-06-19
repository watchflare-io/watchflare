<script lang="ts">
	import { onMount } from 'svelte';
	import { Sun, Moon, Monitor } from 'lucide-svelte';
	import { authTheme, initAuthTheme, cycleAuthTheme } from '$lib/stores/auth-theme';

	onMount(() => initAuthTheme());
</script>

<button
	type="button"
	onclick={cycleAuthTheme}
	class="fixed top-4 right-4 flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
	aria-label="Toggle theme"
>
	{#if $authTheme === 'light'}
		<Sun class="h-4 w-4" />
		<span>Light</span>
	{:else if $authTheme === 'dark'}
		<Moon class="h-4 w-4" />
		<span>Dark</span>
	{:else}
		<Monitor class="h-4 w-4" />
		<span>System</span>
	{/if}
</button>
