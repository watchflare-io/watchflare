<script lang="ts">
    import { authActions } from "$lib/stores";
    import { userStore } from "$lib/stores/user";
    import { Settings, LogOut } from "lucide-svelte";
    import * as DropdownMenu from "$lib/components/ui/dropdown-menu";

    const {
        collapsed = false,
        textClass = "",
        onAction,
    }: {
        collapsed?: boolean;
        textClass?: string;
        onAction?: () => void;
    } = $props();

    const email = $derived($userStore.user?.email || "");
    const displayName = $derived($userStore.user?.username || email);
    const initials = $derived(
        displayName ? displayName.substring(0, 2).toUpperCase() : "??",
    );

    let open = $state(false);

    function handleLogout() {
        onAction?.();
        authActions.logout();
    }
</script>

<DropdownMenu.Root bind:open>
    <DropdownMenu.Trigger>
        {#snippet child({ props })}
            <button
                type="button"
                {...props}
                class="flex w-full items-center rounded-lg text-sm font-medium text-surface-foreground transition-[padding,background-color,color] duration-300 ease-in-out hover:bg-surface-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary {collapsed
                    ? 'p-1.75'
                    : 'p-3.25'}"
                title={displayName || "User menu"}
            >
                <span
                    class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary text-xs font-bold text-primary-foreground"
                >
                    {initials}
                </span>
                <span
                    class="whitespace-nowrap overflow-hidden text-left truncate ml-3 {textClass}"
                >
                    {displayName || "User"}
                </span>
            </button>
        {/snippet}
    </DropdownMenu.Trigger>

    <DropdownMenu.Content side="top" align="start" preventScroll={false} class="mb-1">
        <a
            href="/user"
            onclick={() => {
                open = false;
                onAction?.();
            }}
            class="flex w-full cursor-pointer select-none items-center gap-2 rounded-md px-3 py-2 text-sm text-foreground outline-none hover:bg-muted"
        >
            <Settings class="h-4 w-4" />
            Account
        </a>

        <DropdownMenu.Separator />

        <DropdownMenu.Item
            onclick={handleLogout}
            class="text-destructive data-highlighted:text-destructive"
        >
            <LogOut class="h-4 w-4" />
            Logout
        </DropdownMenu.Item>
    </DropdownMenu.Content>
</DropdownMenu.Root>
