<script lang="ts">
    let {
        checked = $bindable(),
        size = 'md',
        class: className = '',
        onchange,
        'aria-labelledby': ariaLabelledBy,
    }: {
        checked: boolean;
        size?: 'sm' | 'md';
        class?: string;
        onchange?: (value: boolean) => void;
        'aria-labelledby'?: string;
    } = $props();

    const trackSize = $derived(size === 'sm' ? 'h-5 w-9' : 'h-6 w-11');
    const thumbSize = $derived(size === 'sm' ? 'h-3.5 w-3.5' : 'h-4 w-4');
    const thumbOn  = $derived(size === 'sm' ? 'translate-x-5' : 'translate-x-6');
    const thumbOff = $derived(size === 'sm' ? 'translate-x-0.5' : 'translate-x-1');
</script>

<button
    type="button"
    role="switch"
    aria-checked={checked}
    aria-labelledby={ariaLabelledBy}
    onclick={() => {
        checked = !checked;
        onchange?.(checked);
    }}
    class="relative inline-flex shrink-0 items-center rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary {trackSize} {checked ? 'bg-primary' : 'bg-muted border border-border'} {className}"
>
    <span class="inline-block transform rounded-full bg-primary-foreground shadow-sm transition-transform {thumbSize} {checked ? thumbOn : thumbOff}"></span>
</button>
