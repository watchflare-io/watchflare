<script lang="ts">
    import { goto } from "$app/navigation";
    import { isAgentOutdated } from "$lib/utils";
    import HostStatusBadge from "$lib/components/HostStatusBadge.svelte";
    import type { Host } from "$lib/types";

    const {
        hosts,
        sortColumn,
        sortOrder,
        latestAgentVersion = null,
        onSort,
        onDelete,
        onDismissReactivation,
    }: {
        hosts: Host[];
        sortColumn: string;
        sortOrder: "asc" | "desc";
        latestAgentVersion?: string | null;
        onSort: (column: string) => void;
        onDelete: (host: Host, e: Event) => void;
        onDismissReactivation: (hostId: string) => void;
    } = $props();

    function hasIPMismatch(host: Host) {
        return (
            host.configured_ip &&
            host.ip_address_v4 &&
            host.configured_ip !== host.ip_address_v4 &&
            !host.ignore_ip_mismatch
        );
    }
</script>

{#snippet sortIcon(column)}
    {#if sortColumn === column}
        <svg class="h-3 w-3" viewBox="0 0 12 12" fill="currentColor">
            {#if sortOrder === "asc"}
                <path d="M6 2l4 5H2z" />
            {:else}
                <path d="M6 10l4-5H2z" />
            {/if}
        </svg>
    {:else}
        <svg
            class="h-3 w-3 opacity-40 group-hover:opacity-100 transition-opacity"
            viewBox="0 0 12 12"
            fill="currentColor"
        >
            <path d="M6 10l4-5H2z" />
        </svg>
    {/if}
{/snippet}

<table class="w-full min-w-160">
    <thead>
        <tr class="border-b bg-table-header whitespace-nowrap">
            <th scope="col" class="w-2/5 px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">
                <button
                    type="button"
                    onclick={() => onSort("name")}
                    class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn === 'name' ? 'bg-table-header-active text-foreground' : ''}"
                >
                    Name
                    {@render sortIcon("name")}
                </button>
            </th>
            <th scope="col" class="w-1/5 px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">
                <button
                    type="button"
                    onclick={() => onSort("status")}
                    class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn === 'status' ? 'bg-table-header-active text-foreground' : ''}"
                >
                    Status
                    {@render sortIcon("status")}
                </button>
            </th>
            <th scope="col" class="w-1/4 px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">
                <button
                    type="button"
                    onclick={() => onSort("ip")}
                    class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 select-none transition-colors hover:bg-table-header-active hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary {sortColumn === 'ip' ? 'bg-table-header-active text-foreground' : ''}"
                >
                    IP Address
                    {@render sortIcon("ip")}
                </button>
            </th>
            <th scope="col" class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground">
                Agent
            </th>
            <th scope="col" class="px-4 py-2.5 text-right text-sm font-semibold text-muted-foreground">
                Actions
            </th>
        </tr>
    </thead>
    <tbody class="divide-y divide-border">
        {#each hosts as host}
            <tr
                onclick={() => goto(`/hosts/${host.id}`)}
                class="hover:bg-muted/20 transition-colors cursor-pointer"
            >
                <td class="px-4 py-3">
                    <div class="flex flex-col">
                        <span class="font-medium text-foreground"
                            >{host.display_name}</span
                        >
                        {#if host.hostname}
                            <span class="text-xs text-muted-foreground"
                                >{host.hostname}</span
                            >
                        {/if}
                    </div>
                </td>
                <td class="px-4 py-3">
                    <div class="flex items-center gap-2">
                        <HostStatusBadge status={host.status} />
                        {#if hasIPMismatch(host)}
                            <span
                                class="inline-flex items-center text-warning"
                                title="IP mismatch detected"
                            >
                                <svg
                                    class="h-4 w-4"
                                    fill="currentColor"
                                    viewBox="0 0 20 20"
                                >
                                    <path
                                        fill-rule="evenodd"
                                        d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
                                        clip-rule="evenodd"
                                    />
                                </svg>
                            </span>
                        {/if}
                        {#if host.reactivated_at}
                            <span
                                class="inline-flex items-center gap-1 rounded-full border border-primary/20 bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary"
                                title="Agent was reactivated (same physical host via UUID)"
                            >
                                <svg
                                    class="h-3 w-3"
                                    fill="currentColor"
                                    viewBox="0 0 20 20"
                                >
                                    <path
                                        fill-rule="evenodd"
                                        d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z"
                                        clip-rule="evenodd"
                                    />
                                </svg>
                                Reactivated
                                <button
                                    type="button"
                                    onclick={(e) => {
                                        e.stopPropagation();
                                        onDismissReactivation(host.id);
                                    }}
                                    class="ml-0.5 text-primary hover:text-primary/80"
                                    aria-label="Dismiss reactivation notice"
                                >
                                    <svg
                                        class="h-3 w-3"
                                        fill="currentColor"
                                        viewBox="0 0 20 20"
                                    >
                                        <path
                                            fill-rule="evenodd"
                                            d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                                            clip-rule="evenodd"
                                        />
                                    </svg>
                                </button>
                            </span>
                        {/if}
                    </div>
                </td>
                <td class="px-4 py-3 text-sm text-foreground">
                    {host.ip_address_v4 || host.configured_ip || "-"}
                </td>
                <td
                    class="px-4 py-3 text-sm text-muted-foreground table-cell"
                >
                    <span class="inline-flex items-center gap-1.5">
                        {host.agent_version ? `v${host.agent_version}` : "—"}
                        {#if isAgentOutdated(host.agent_version, latestAgentVersion)}
                            <span
                                class="inline-flex items-center rounded-full border border-warning/20 bg-warning/10 px-1.5 py-0.5 text-xs font-medium text-warning"
                                title="v{latestAgentVersion} available"
                            >
                                ↑
                            </span>
                        {/if}
                    </span>
                </td>
                <td class="px-4 py-3 text-right">
                    <div class="flex items-center justify-end gap-3">
                        <button
                            type="button"
                            onclick={(e) => {
                                e.stopPropagation();
                                goto(`/hosts/${host.id}`);
                            }}
                            class="text-sm font-medium text-primary hover:text-primary/80 transition-colors"
                        >
                            View
                        </button>
                        <button
                            type="button"
                            onclick={(e) => onDelete(host, e)}
                            class="text-sm font-medium text-destructive hover:text-destructive/80 transition-colors"
                        >
                            Delete
                        </button>
                    </div>
                </td>
            </tr>
        {/each}
    </tbody>
</table>
