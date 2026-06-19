<script lang="ts">
	import { TIME_RANGES, logger } from '$lib/utils';
	import { updatePreferences } from '$lib/api';
	import * as Select from '$lib/components/ui/select';
	import type { TimeRange } from '$lib/types';

	let {
		value = $bindable<TimeRange>('24h'),
		onValueChange,
		class: className = ''
	}: {
		value?: TimeRange;
		onValueChange?: (value: TimeRange) => void;
		class?: string;
	} = $props();

	let selectedLabel = $derived(TIME_RANGES.find((r) => r.value === value)?.label || value);

	async function handleChange(newValue: TimeRange) {
		value = newValue;

		// Save to user preferences
		try {
			await updatePreferences({ default_time_range: newValue });
		} catch (err) {
			logger.error('Failed to save time range preference:', err);
		}

		// Trigger callback if provided
		if (onValueChange) {
			onValueChange(newValue);
		}
	}
</script>

<Select.Root type="single" {value} onValueChange={handleChange}>
	<Select.Trigger class={className} items={TIME_RANGES.map((r) => r.label)}>
		<span>{selectedLabel}</span>
	</Select.Trigger>
	<Select.Content>
		{#each TIME_RANGES as range}
			<Select.Item value={range.value} label={range.label}>
				{range.label}
			</Select.Item>
		{/each}
	</Select.Content>
</Select.Root>
