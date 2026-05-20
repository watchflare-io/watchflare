<script lang="ts">
    import * as Select from "$lib/components/ui/select";

    const {
        searchQuery,
        statusFilter,
        onSearchInput,
        onStatusChange,
    }: {
        searchQuery: string;
        statusFilter: string;
        onSearchInput: (e: Event) => void;
        onStatusChange: (value: string) => void;
    } = $props();

    const statusOptions = [
        { value: "", label: "All statuses" },
        { value: "online", label: "Online" },
        { value: "offline", label: "Offline" },
        { value: "paused", label: "Paused" },
        { value: "pending", label: "Pending" },
    ];

    const statusLabel = $derived(
        statusOptions.find((o) => o.value === statusFilter)?.label ||
            "All statuses",
    );
</script>

<div class="mb-4 flex items-center gap-2 flex-wrap">
    <input
        type="text"
        placeholder="Search hosts..."
        value={searchQuery}
        oninput={onSearchInput}
        class="flex-1 min-w-48 h-9 rounded-lg border bg-card px-3 text-sm text-foreground placeholder:text-sm placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
    />
    <Select.Root
        type="single"
        value={statusFilter}
        onValueChange={onStatusChange}
    >
        <Select.Trigger items={statusOptions.map((o) => o.label)}>
            <span>{statusLabel}</span>
        </Select.Trigger>
        <Select.Content>
            {#each statusOptions as option}
                <Select.Item value={option.value} label={option.label}>
                    {option.label}
                </Select.Item>
            {/each}
        </Select.Content>
    </Select.Root>
</div>
