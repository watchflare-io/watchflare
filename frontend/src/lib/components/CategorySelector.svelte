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
		<label
			for={`${idPrefix}-${cat.value}`}
			class="flex items-start gap-2.5 cursor-pointer select-none"
		>
			<input
				id={`${idPrefix}-${cat.value}`}
				type="checkbox"
				checked={categories.includes(cat.value)}
				onchange={() => onChange(toggleCategory(categories, cat.value))}
				class="mt-0.5 h-4 w-4 shrink-0 rounded border-border text-primary focus-visible:ring-2 focus-visible:ring-primary/50"
			/>
			<span class="min-w-0">
				<span class="block text-sm font-medium text-foreground">{cat.label}</span>
				<span class="block text-xs text-muted-foreground">{cat.hint}</span>
			</span>
		</label>
	{/each}
</div>
