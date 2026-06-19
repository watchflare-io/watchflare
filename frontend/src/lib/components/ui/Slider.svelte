<script lang="ts">
	let {
		value = $bindable(),
		min = 0,
		max = 100,
		step = 1,
		oninput
	}: {
		value: number;
		min?: number;
		max?: number;
		step?: number;
		oninput?: () => void;
	} = $props();

	const fillPct = $derived(Math.min(Math.max(((value - min) / (max - min)) * 100, 0), 100));
	const fillStyle = $derived(
		`background: linear-gradient(to right, var(--primary) ${fillPct}%, var(--border) ${fillPct}%)`
	);
</script>

<input
	type="range"
	{min}
	{max}
	{step}
	bind:value
	{oninput}
	style={fillStyle}
	class="slider w-full"
/>

<style>
	.slider {
		-webkit-appearance: none;
		appearance: none;
		height: 5px;
		border-radius: 9999px;
		outline: none;
		cursor: pointer;
	}

	.slider::-webkit-slider-thumb {
		-webkit-appearance: none;
		appearance: none;
		width: 16px;
		height: 16px;
		border-radius: 50%;
		background: var(--primary);
		cursor: pointer;
		box-shadow:
			0 0 0 2px var(--surface),
			0 1px 4px rgba(0, 0, 0, 0.2);
		transition:
			transform 0.15s ease,
			box-shadow 0.15s ease;
	}

	.slider::-webkit-slider-thumb:hover {
		transform: scale(1.2);
		box-shadow:
			0 0 0 2px var(--surface),
			0 0 0 4px color-mix(in oklch, var(--primary) 30%, transparent);
	}

	.slider:focus-visible::-webkit-slider-thumb {
		box-shadow:
			0 0 0 2px var(--surface),
			0 0 0 4px color-mix(in oklch, var(--primary) 40%, transparent);
	}

	.slider::-moz-range-thumb {
		border: 2px solid var(--surface);
		width: 14px;
		height: 14px;
		border-radius: 50%;
		background: var(--primary);
		cursor: pointer;
	}

	.slider::-moz-range-track {
		background: var(--border);
		border-radius: 9999px;
		height: 5px;
	}

	.slider::-moz-range-progress {
		background: var(--primary);
		border-radius: 9999px;
		height: 5px;
	}
</style>
