<script lang="ts">
	import { NOTIFICATION_CATEGORIES } from '$lib/types';
	import type { NotificationCategory } from '$lib/types';
	import { toggleCategory } from '$lib/utils';

	const {
		categories,
		onChange,
		idPrefix = 'cat'
	}: {
		categories: NotificationCategory[];
		onChange: (next: NotificationCategory[]) => void;
		idPrefix?: string;
	} = $props();
</script>

<div class="flex flex-col gap-2">
	{#each NOTIFICATION_CATEGORIES as cat (cat.value)}
		{@const checked = categories.includes(cat.value)}
		<button
			id={`${idPrefix}-${cat.value}`}
			type="button"
			role="checkbox"
			aria-checked={checked}
			onclick={() => onChange(toggleCategory(categories, cat.value))}
			class="flex items-start gap-2.5 rounded text-left select-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
		>
			<span
				class="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded border transition-colors {checked
					? 'border-primary bg-primary'
					: 'border-muted-foreground/40'}"
			>
				{#if checked}
					<svg
						class="h-3 w-3 text-primary-foreground"
						fill="none"
						stroke="currentColor"
						viewBox="0 0 24 24"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="3"
							d="M5 13l4 4L19 7"
						/>
					</svg>
				{/if}
			</span>
			<span class="min-w-0">
				<span class="block text-sm font-medium text-foreground">{cat.label}</span>
				<span class="block text-xs text-muted-foreground">{cat.hint}</span>
			</span>
		</button>
	{/each}
</div>
