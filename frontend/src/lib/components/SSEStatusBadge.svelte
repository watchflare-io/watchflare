<script lang="ts">
    import { sseConnectionState } from "$lib/stores/sse";
    import { pageSseState } from "$lib/stores/pageSse";

    const { textClass = "" }: { textClass?: string } = $props();

    const connectionState = $derived($pageSseState ?? $sseConnectionState);
    const isReconnecting = $derived(connectionState === "reconnecting");

    function getStateColor(state: string): string {
        switch (state) {
            case "connected":
                return "bg-success";
            case "connecting":
                return "bg-warning";
            case "reconnecting":
                return "bg-warning";
            case "error":
                return "bg-destructive";
            default:
                return "bg-muted-foreground";
        }
    }

    function getStateLabel(state: string): string {
        switch (state) {
            case "connected":
                return "SSE Connected";
            case "connecting":
                return "SSE Connecting...";
            case "reconnecting":
                return "SSE Reconnecting...";
            case "error":
                return "SSE Error";
            default:
                return "SSE Disconnected";
        }
    }
</script>

<div
    class="flex items-center rounded-lg px-4.75 py-2 text-xs"
    title={getStateLabel(connectionState)}
>
    <span class="relative flex h-2 w-2 shrink-0">
        <span
            class="absolute inline-flex h-full w-full rounded-full opacity-75 {isReconnecting
                ? 'animate-ping'
                : ''} {getStateColor(connectionState)}"
        ></span>
        <span
            class="relative inline-flex h-2 w-2 rounded-full {getStateColor(
                connectionState,
            )}"
        >
        </span>
    </span>
    <span
        class="ml-3 text-muted-foreground whitespace-nowrap overflow-hidden {textClass}"
        >{getStateLabel(connectionState)}</span
    >
</div>
