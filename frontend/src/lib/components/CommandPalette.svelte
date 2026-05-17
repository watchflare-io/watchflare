<script lang="ts">
    import { goto } from "$app/navigation";
    import { Command, Dialog } from "bits-ui";
    import { Search, Server } from "lucide-svelte";
    import * as api from "$lib/api.js";
    import type { Host as HostType } from "$lib/types";

    let { open = $bindable(false) } = $props();

    let query = $state("");
    let results: HostType[] = $state([]);
    let loading = $state(false);
    let searchTimeout: ReturnType<typeof setTimeout> | null = $state(null);

    function handleInputChange(value: string) {
        query = value;
        if (searchTimeout) clearTimeout(searchTimeout);
        if (!value.trim()) {
            results = [];
            loading = false;
            return;
        }
        loading = true;
        searchTimeout = setTimeout(async () => {
            try {
                const response = await api.listHosts({ search: value, perPage: 10 });
                results = response.hosts || [];
            } catch {
                results = [];
            } finally {
                loading = false;
            }
        }, 200);
    }

    function handleSelect(hostId: string) {
        open = false;
        query = "";
        results = [];
        goto(`/hosts/${hostId}`);
    }

    function getStatusDot(status: string): string {
        switch (status) {
            case "online": return "bg-success";
            case "offline": return "bg-muted-foreground";
            default: return "bg-warning";
        }
    }
</script>

<Dialog.Root bind:open onOpenChange={(v) => { if (!v) { query = ""; results = []; } }}>
    <Dialog.Portal>
        <Dialog.Overlay
            class="fixed inset-0 z-50 bg-black/50 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=closed]:animate-out data-[state=closed]:fade-out-0"
        />
        <Dialog.Content
            class="fixed left-1/2 top-1/2 z-50 w-full max-w-lg -translate-x-1/2 -translate-y-1/2 rounded-xl border bg-surface shadow-xl data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95"
        >
            <Command.Root shouldFilter={false} class="flex flex-col">
                <div class="flex items-center border-b px-4">
                    <Search class="h-4 w-4 shrink-0 text-muted-foreground" />
                    <Command.Input
                        placeholder="Search hosts..."
                        class="flex-1 bg-transparent px-3 py-3 text-sm text-foreground outline-none placeholder:text-muted-foreground"
                        value={query}
                        onValueChange={handleInputChange}
                    />
                    <kbd class="hidden sm:inline-flex items-center rounded border bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">
                        Esc
                    </kbd>
                </div>
                <Command.List class="max-h-72 overflow-y-auto p-2">
                    {#if !query.trim()}
                        <div class="py-8 text-center text-sm text-muted-foreground">
                            Type to search hosts...
                        </div>
                    {:else if loading}
                        <div class="py-8 text-center text-sm text-muted-foreground">
                            Searching...
                        </div>
                    {:else if results.length === 0}
                        <Command.Empty class="py-8 text-center text-sm text-muted-foreground">
                            No hosts found
                        </Command.Empty>
                    {:else}
                        {#each results as host}
                            <Command.Item
                                value={host.id}
                                onSelect={() => handleSelect(host.id)}
                                class="flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm cursor-pointer outline-none data-highlighted:bg-muted transition-colors"
                            >
                                <Server class="h-4 w-4 shrink-0 text-muted-foreground" />
                                <div class="flex-1 min-w-0">
                                    <div class="flex items-center gap-2">
                                        <span class="font-medium text-foreground truncate">{host.display_name}</span>
                                        <span class="h-1.5 w-1.5 shrink-0 rounded-full {getStatusDot(host.status)}"></span>
                                    </div>
                                    {#if host.hostname}
                                        <p class="text-xs text-muted-foreground truncate">{host.hostname}{#if host.ip_address_v4} · {host.ip_address_v4}{/if}</p>
                                    {/if}
                                </div>
                            </Command.Item>
                        {/each}
                    {/if}
                </Command.List>
            </Command.Root>
        </Dialog.Content>
    </Dialog.Portal>
</Dialog.Root>
