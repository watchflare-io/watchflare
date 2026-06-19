<script lang="ts">
    import type { Snippet } from "svelte";

    const {
        open,
        onClose,
        children,
        size = "default",
    }: {
        open: boolean;
        onClose: () => void;
        children: Snippet;
        size?: "default" | "wide";
    } = $props();

    $effect(() => {
        document.body.classList.toggle("overflow-hidden", open);
        return () => document.body.classList.remove("overflow-hidden");
    });
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && open && onClose()} />

<!-- Backdrop -->
<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div
    style="transition: opacity 300ms, visibility 0ms {open ? '0ms' : '300ms'}"
    class="fixed inset-0 z-40 bg-black/50 m-0 max-w-dvw {open
        ? 'opacity-100 visible'
        : 'opacity-0 invisible pointer-events-none'}"
    role="presentation"
    onclick={onClose}
></div>

<!-- Panel -->
<div
    role="dialog"
    aria-modal="true"
    aria-hidden={!open}
    inert={!open}
    tabindex="-1"
    class="fixed right-0 top-0 z-50 h-svh w-full sm:py-4 sm:pr-4 bg-transparent transition-transform duration-300
        {size === 'wide'
        ? 'sm:w-156 sm:max-w-[calc(100vw-1rem)]'
        : 'sm:w-80 sm:max-w-[85vw]'}
        {open ? 'translate-x-0' : 'translate-x-[calc(100%+1.5rem)]'}"
>
    <div
        class="flex h-full flex-col bg-surface sm:rounded-2xl border shadow-lg"
    >
        {@render children()}
    </div>
</div>
