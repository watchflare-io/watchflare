<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import { page } from "$app/stores";
    import { goto } from "$app/navigation";
    import * as api from "$lib/api.js";
    import * as DropdownMenu from "$lib/components/ui/dropdown-menu";
    import { PACKAGES_PER_PAGE, SEARCH_DEBOUNCE_MS } from "$lib/constants";
    import type { GlobalPackage, GlobalPackageStatus } from "$lib/types";
    import Pagination from "$lib/components/Pagination.svelte";
    import PackageStatusBadge from "$lib/components/PackageStatusBadge.svelte";
    import { getManagerLabel, getManagerColor } from "$lib/utils";
    import {
        Filter,
        ChevronDown,
        Tag,
        X,
        Package as PackageIcon,
        ShieldAlert,
        ArrowUp,
        Users,
        Server,
    } from "lucide-svelte";

    const PAGE_SIZE_OPTIONS = [25, 50, 100, 200];

    type StatusFilter = GlobalPackageStatus;
    const STATUS_LABELS: Record<StatusFilter, string> = {
        security: "Security update",
        outdated: "Outdated",
        up_to_date: "Up to date",
        not_checked: "Not checked",
    };

    // Data
    let packages: GlobalPackage[] = $state([]);
    let totalCount = $state(0);
    let totalPages = $state(1);
    let totalPackages = $state(0);
    let outdatedCount = $state(0);
    let securityCount = $state(0);
    let outdatedHostsCount = $state(0);
    let availableManagers: string[] = $state([]);
    // initialLoading: first load — shows skeletons for cards and table loading state
    let initialLoading = $state(true);
    // tableLoading: filter/sort/page changes — keeps table visible, dims content
    let tableLoading = $state(false);
    let error = $state("");

    // Filters
    let searchTerm = $state("");
    let selectedStatuses: Set<StatusFilter> = $state(new Set());
    let selectedManagers: Set<string> = $state(new Set());

    // Sort
    let sortColumn = $state("name");
    let sortOrder: "asc" | "desc" = $state("asc");

    // Pagination
    let limit = $state(PACKAGES_PER_PAGE);
    let offset = $state(0);

    const currentPage = $derived(Math.floor(offset / limit) + 1);
    const isStatusFiltered = $derived(selectedStatuses.size > 0);
    const isManagerFiltered = $derived(selectedManagers.size > 0);
    const hasActiveFilters = $derived(
        searchTerm !== "" || isStatusFiltered || isManagerFiltered,
    );
    const statusFilterLabel = $derived(
        selectedStatuses.size === 0
            ? "All statuses"
            : selectedStatuses.size === 1
              ? STATUS_LABELS[[...selectedStatuses][0]]
              : `${selectedStatuses.size} statuses`,
    );
    const managerFilterLabel = $derived(
        !isManagerFiltered
            ? "All managers"
            : selectedManagers.size === 1
              ? getManagerLabel([...selectedManagers][0])
              : `${selectedManagers.size} managers`,
    );

    let searchDebounce: ReturnType<typeof setTimeout> | null = null;

    async function loadData(isInitial = false) {
        if (isInitial) {
            initialLoading = true;
        } else {
            tableLoading = true;
        }
        error = "";
        try {
            const resp = await api.listAllPackages({
                q: searchTerm || undefined,
                status: [...selectedStatuses] as GlobalPackageStatus[],
                manager: [...selectedManagers],
                limit,
                offset,
                sort_by: sortColumn,
                sort_order: sortOrder,
            });
            packages = resp.packages ?? [];
            totalCount = resp.pagination?.total ?? 0;
            totalPages = resp.pagination?.pages ?? 1;
            totalPackages = resp.total_packages ?? 0;
            outdatedCount = resp.outdated_count ?? 0;
            securityCount = resp.security_count ?? 0;
            outdatedHostsCount = resp.outdated_hosts_count ?? 0;
            availableManagers = resp.available_managers ?? [];
        } catch (err: unknown) {
            error =
                err instanceof Error ? err.message : "Failed to load packages";
        } finally {
            initialLoading = false;
            tableLoading = false;
        }
    }

    function updateURL() {
        const params = new URLSearchParams();
        if (searchTerm) params.set("q", searchTerm);
        for (const s of selectedStatuses) params.append("status", s);
        for (const m of selectedManagers) params.append("manager", m);
        if (limit !== PACKAGES_PER_PAGE) params.set("limit", String(limit));
        if (offset > 0) params.set("offset", String(offset));
        if (sortColumn !== "name") params.set("sort_by", sortColumn);
        if (sortOrder !== "asc") params.set("sort_order", sortOrder);
        const qs = params.toString();
        goto(qs ? `?${qs}` : "?", {
            replaceState: true,
            noScroll: true,
            keepFocus: true,
        });
    }

    function handleSearchInput() {
        offset = 0;
        if (searchDebounce) clearTimeout(searchDebounce);
        searchDebounce = setTimeout(() => {
            updateURL();
            loadData();
        }, SEARCH_DEBOUNCE_MS);
    }

    function toggleStatus(status: StatusFilter) {
        const next = new Set(selectedStatuses);
        if (next.has(status)) next.delete(status);
        else next.add(status);
        selectedStatuses = next;
        offset = 0;
        updateURL();
        loadData();
    }

    function toggleManager(manager: string) {
        const next = new Set(selectedManagers);
        if (next.has(manager)) next.delete(manager);
        else next.add(manager);
        selectedManagers = next;
        offset = 0;
        updateURL();
        loadData();
    }

    function clearAllFilters() {
        searchTerm = "";
        selectedStatuses = new Set();
        selectedManagers = new Set();
        offset = 0;
        updateURL();
        loadData();
    }

    function handleSort(column: string) {
        if (sortColumn === column) {
            sortOrder = sortOrder === "asc" ? "desc" : "asc";
        } else {
            sortColumn = column;
            sortOrder =
                column === "host_count" || column === "available_version"
                    ? "desc"
                    : "asc";
        }
        offset = 0;
        updateURL();
        loadData();
    }

    function handlePageChange(newPage: number) {
        offset = (newPage - 1) * limit;
        updateURL();
        loadData();
    }

    function handlePageSizeChange(size: number) {
        limit = size;
        offset = 0;
        updateURL();
        loadData();
    }

    onMount(() => {
        const params = $page.url.searchParams;
        searchTerm = params.get("q") ?? "";
        selectedStatuses = new Set(
            params.getAll("status").filter(Boolean) as GlobalPackageStatus[],
        );
        selectedManagers = new Set(params.getAll("manager").filter(Boolean));
        const urlOffset = Number(params.get("offset"));
        offset = isNaN(urlOffset) ? 0 : urlOffset;
        const urlLimit = Number(params.get("limit"));
        if (!isNaN(urlLimit) && urlLimit > 0) limit = urlLimit;
        sortColumn = params.get("sort_by") ?? "name";
        const urlOrder = params.get("sort_order");
        if (urlOrder === "desc") sortOrder = "desc";
        loadData(true);
    });

    onDestroy(() => {
        if (searchDebounce) clearTimeout(searchDebounce);
    });
</script>

<svelte:head>
    <title>Packages - Watchflare</title>
</svelte:head>

<!-- Page header -->
<div class="mb-6">
    <h1 class="text-xl font-semibold text-foreground">Packages</h1>
    <p class="text-sm text-muted-foreground mt-0.5">
        Inventory of all installed packages
    </p>
</div>

<!-- Error -->
{#if error}
    <div
        role="alert"
        class="mb-6 rounded-lg border border-destructive bg-destructive/10 p-4"
    >
        <p class="text-sm text-destructive">{error}</p>
    </div>
{/if}

<!-- Stat cards -->
<div class="mb-6 grid grid-cols-2 gap-3 lg:grid-cols-4 lg:gap-4">
    {#if initialLoading}
        {#each Array(4) as _}
            <div
                class="flex items-center gap-2.5 rounded-lg border bg-card px-3 py-2.5 animate-pulse"
            >
                <div class="h-8 w-8 rounded-md bg-muted shrink-0"></div>
                <div class="min-w-0 flex-1">
                    <div class="h-3 w-16 rounded bg-muted mb-2"></div>
                    <div class="h-4 w-10 rounded bg-muted"></div>
                </div>
            </div>
        {/each}
    {:else}
        <!-- Total packages -->
        <div
            class="flex items-center gap-2.5 rounded-lg border bg-card px-3 py-2.5"
        >
            <div
                class="flex items-center justify-center rounded-md bg-primary/10 text-primary h-8 w-8 shrink-0"
            >
                <PackageIcon class="h-4 w-4" />
            </div>
            <div class="min-w-0">
                <p class="text-xs text-muted-foreground truncate">Packages</p>
                <p class="text-sm font-semibold text-foreground">
                    {totalPackages.toLocaleString()}
                </p>
            </div>
        </div>

        <!-- Outdated packages -->
        <button
            type="button"
            onclick={() => {
                selectedStatuses = new Set(["outdated"]);
                offset = 0;
                updateURL();
                loadData();
            }}
            class="flex items-center gap-2.5 rounded-lg border bg-card px-3 py-2.5 text-left transition-colors hover:bg-muted/30 focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:outline-none"
        >
            <div
                class="flex items-center justify-center rounded-md h-8 w-8 shrink-0
                {outdatedCount > 0
                    ? 'bg-warning/10 text-warning'
                    : 'bg-muted text-muted-foreground'}"
            >
                <ArrowUp class="h-4 w-4" />
            </div>
            <div class="min-w-0">
                <p class="text-xs text-muted-foreground truncate">
                    Outdated packages
                </p>
                <p
                    class="text-sm font-semibold {outdatedCount > 0
                        ? 'text-warning'
                        : 'text-foreground'}"
                >
                    {outdatedCount.toLocaleString()}
                </p>
            </div>
        </button>

        <!-- Security updates -->
        <button
            type="button"
            onclick={() => {
                selectedStatuses = new Set(["security"]);
                offset = 0;
                updateURL();
                loadData();
            }}
            class="flex items-center gap-2.5 rounded-lg border bg-card px-3 py-2.5 text-left transition-colors hover:bg-muted/30 focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:outline-none"
        >
            <div
                class="flex items-center justify-center rounded-md h-8 w-8 shrink-0
                {securityCount > 0
                    ? 'bg-destructive/10 text-destructive'
                    : 'bg-muted text-muted-foreground'}"
            >
                <ShieldAlert class="h-4 w-4" />
            </div>
            <div class="min-w-0">
                <p class="text-xs text-muted-foreground truncate">
                    Security updates
                </p>
                <p
                    class="text-sm font-semibold {securityCount > 0
                        ? 'text-destructive'
                        : 'text-foreground'}"
                >
                    {securityCount.toLocaleString()}
                </p>
            </div>
        </button>

        <!-- Outdated hosts -->
        <div
            class="flex items-center gap-2.5 rounded-lg border bg-card px-3 py-2.5"
        >
            <div
                class="flex items-center justify-center rounded-md h-8 w-8 shrink-0
                {outdatedHostsCount > 0
                    ? 'bg-warning/10 text-warning'
                    : 'bg-muted text-muted-foreground'}"
            >
                <Server class="h-4 w-4" />
            </div>
            <div class="min-w-0">
                <p class="text-xs text-muted-foreground truncate">
                    Outdated hosts
                </p>
                <p
                    class="text-sm font-semibold {outdatedHostsCount > 0
                        ? 'text-warning'
                        : 'text-foreground'}"
                >
                    {outdatedHostsCount.toLocaleString()}
                </p>
            </div>
        </div>
    {/if}
</div>

<!-- Search & Filters -->
<div class="mb-4 flex items-center gap-2 flex-wrap">
    <input
        type="text"
        bind:value={searchTerm}
        oninput={handleSearchInput}
        onkeydown={(e) => {
            if (e.key === "Enter") {
                if (searchDebounce) clearTimeout(searchDebounce);
                updateURL();
                loadData();
            }
        }}
        placeholder="Search packages..."
        class="flex-1 min-w-48 h-9 rounded-lg border bg-card px-3 text-sm text-foreground placeholder:text-sm placeholder:text-muted-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
    />
        <!-- Status filter -->
        <DropdownMenu.Root>
            <DropdownMenu.Trigger>
                {#snippet child({ props })}
                    <button
                        type="button"
                        {...props}
                        class="inline-flex items-center gap-1.5 h-9 rounded-lg border px-3 text-sm font-medium transition-colors whitespace-nowrap
                        {isStatusFiltered
                            ? 'border-primary/40 bg-primary/5 text-primary hover:bg-primary/10'
                            : 'bg-card text-muted-foreground hover:bg-muted hover:text-foreground'}"
                    >
                        <Tag class="h-3.5 w-3.5 shrink-0" />
                        <span class="hidden sm:inline">{statusFilterLabel}</span
                        >
                        {#if isStatusFiltered}
                            <span
                                class="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-primary/15 px-1 text-xs font-medium text-primary"
                            >
                                {selectedStatuses.size}
                            </span>
                        {/if}
                        <ChevronDown
                            class="hidden sm:inline-block h-3 w-3 opacity-40"
                        />
                    </button>
                {/snippet}
            </DropdownMenu.Trigger>
            <DropdownMenu.Content align="start">
                {#each [{ value: "security" as StatusFilter, label: "Security update" }, { value: "outdated" as StatusFilter, label: "Outdated" }, { value: "up_to_date" as StatusFilter, label: "Up to date" }, { value: "not_checked" as StatusFilter, label: "Not checked" }] as status}
                    <DropdownMenu.Item
                        closeOnSelect={false}
                        onclick={() => toggleStatus(status.value)}
                    >
                        <div
                            class="flex h-4 w-4 shrink-0 items-center justify-center rounded border
                            {selectedStatuses.has(status.value)
                                ? 'border-primary bg-primary'
                                : 'border-muted-foreground/40'}"
                        >
                            {#if selectedStatuses.has(status.value)}
                                <svg
                                    class="h-3 w-3 text-primary-foreground"
                                    fill="none"
                                    stroke="currentColor"
                                    viewBox="0 0 24 24"
                                >
                                    <path
                                        stroke-linecap="round"
                                        stroke-linejoin="round"
                                        stroke-width="3"
                                        d="M5 13l4 4L19 7"
                                    />
                                </svg>
                            {/if}
                        </div>
                        <span class="flex-1">{status.label}</span>
                    </DropdownMenu.Item>
                {/each}
                {#if isStatusFiltered}
                    <DropdownMenu.Separator />
                    <DropdownMenu.Item
                        onclick={() => {
                            selectedStatuses = new Set();
                            offset = 0;
                            updateURL();
                            loadData();
                        }}
                        class="text-muted-foreground"
                    >
                        Clear filter
                    </DropdownMenu.Item>
                {/if}
            </DropdownMenu.Content>
        </DropdownMenu.Root>

        <!-- Manager filter -->
        <DropdownMenu.Root>
            <DropdownMenu.Trigger>
                {#snippet child({ props })}
                    <button
                        type="button"
                        {...props}
                        class="inline-flex items-center gap-1.5 h-9 rounded-lg border px-3 text-sm font-medium transition-colors whitespace-nowrap
                        {isManagerFiltered
                            ? 'border-primary/40 bg-primary/5 text-primary hover:bg-primary/10'
                            : 'bg-card text-muted-foreground hover:bg-muted hover:text-foreground'}"
                    >
                        <Filter class="h-3.5 w-3.5 shrink-0" />
                        <span class="hidden sm:inline"
                            >{managerFilterLabel}</span
                        >
                        {#if isManagerFiltered}
                            <span
                                class="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-primary/15 px-1 text-xs font-medium text-primary"
                            >
                                {selectedManagers.size}
                            </span>
                        {/if}
                        <ChevronDown
                            class="hidden sm:inline-block h-3 w-3 opacity-40"
                        />
                    </button>
                {/snippet}
            </DropdownMenu.Trigger>
            <DropdownMenu.Content align="start">
                <div class="max-h-48 overflow-y-auto">
                    {#each availableManagers as manager}
                        <DropdownMenu.Item
                            closeOnSelect={false}
                            onclick={() => toggleManager(manager)}
                        >
                            <div
                                class="flex h-4 w-4 shrink-0 items-center justify-center rounded border
                                {selectedManagers.has(manager)
                                    ? 'border-primary bg-primary'
                                    : 'border-muted-foreground/40'}"
                            >
                                {#if selectedManagers.has(manager)}
                                    <svg
                                        class="h-3 w-3 text-primary-foreground"
                                        fill="none"
                                        stroke="currentColor"
                                        viewBox="0 0 24 24"
                                    >
                                        <path
                                            stroke-linecap="round"
                                            stroke-linejoin="round"
                                            stroke-width="3"
                                            d="M5 13l4 4L19 7"
                                        />
                                    </svg>
                                {/if}
                            </div>
                            <span class="flex-1"
                                >{getManagerLabel(manager)}</span
                            >
                        </DropdownMenu.Item>
                    {/each}
                    {#if availableManagers.length === 0}
                        <DropdownMenu.Item disabled>
                            <span class="text-muted-foreground text-xs"
                                >No packages yet</span
                            >
                        </DropdownMenu.Item>
                    {/if}
                </div>
                {#if isManagerFiltered}
                    <DropdownMenu.Separator />
                    <DropdownMenu.Item
                        onclick={() => {
                            selectedManagers = new Set();
                            offset = 0;
                            updateURL();
                            loadData();
                        }}
                        class="text-muted-foreground"
                    >
                        Clear filter
                    </DropdownMenu.Item>
                {/if}
            </DropdownMenu.Content>
        </DropdownMenu.Root>

        {#if hasActiveFilters}
            <button
                type="button"
                onclick={clearAllFilters}
                class="inline-flex items-center gap-1.5 h-9 rounded-lg border px-3 text-sm font-medium transition-colors whitespace-nowrap bg-card text-muted-foreground hover:bg-muted hover:text-foreground"
                aria-label="Clear all filters"
            >
                <X class="h-3.5 w-3.5 shrink-0" />
                <span class="hidden sm:inline">Clear filters</span>
            </button>
        {/if}
</div>

{#snippet sortIcon(column: string)}
    {#if sortColumn === column}
        <svg class="h-3 w-3 shrink-0" viewBox="0 0 12 12" fill="currentColor">
            {#if sortOrder === "asc"}
                <path d="M6 2l4 5H2z" />
            {:else}
                <path d="M6 10l4-5H2z" />
            {/if}
        </svg>
    {:else}
        <svg
            class="h-3 w-3 shrink-0 opacity-40 group-hover:opacity-100 transition-opacity"
            viewBox="0 0 12 12"
            fill="currentColor"
        >
            <path d="M6 10l4-5H2z" />
        </svg>
    {/if}
{/snippet}

<!-- Packages Table/Cards -->
<div class="rounded-xl border bg-card overflow-hidden mb-6">
    {#if initialLoading}
        <div class="flex items-center justify-center py-20">
            <p class="text-muted-foreground">Loading packages...</p>
        </div>
    {:else}
        <!-- Mobile: cards -->
        <div
            class="md:hidden p-3 flex flex-col gap-2 {tableLoading
                ? 'opacity-50 pointer-events-none'
                : ''}"
        >
            {#each packages as pkg}
                <div class="rounded-lg border bg-card">
                    <!-- Header: name + status badge -->
                    <div
                        class="rounded-t-lg bg-table-header px-4 py-2.5 border-b border-border flex items-center justify-between gap-2"
                    >
                        <span class="flex items-center gap-2 min-w-0">
                            <PackageIcon
                                class="h-3.5 w-3.5 shrink-0 text-muted-foreground"
                            />
                            <span
                                class="text-sm font-medium text-foreground break-all"
                                >{pkg.name}</span
                            >
                        </span>
                        <PackageStatusBadge hasSecurityUpdate={pkg.has_security_update} availableVersion={pkg.available_version} updateChecked={pkg.update_checked} />
                    </div>
                    <!-- Body: host count + manager + latest version -->
                    <div class="px-4 py-3 flex items-center gap-2 flex-wrap">
                        <span
                            class="inline-flex items-center gap-1 text-xs text-muted-foreground"
                        >
                            <Users class="h-3 w-3 shrink-0" />
                            {pkg.host_count}
                            {pkg.host_count === 1 ? "host" : "hosts"}
                        </span>
                        <span
                            class="inline-flex rounded-full border px-2 py-0.5 text-xs font-medium {getManagerColor(
                                pkg.package_manager,
                            )}">{getManagerLabel(pkg.package_manager)}</span
                        >
                        <span class="w-full text-xs text-muted-foreground">
                            Latest: <span class="font-mono"
                                >{pkg.available_version || pkg.current_version || "—"}</span
                            >
                        </span>
                    </div>
                </div>
            {:else}
                <div class="py-16 text-center">
                    <svg
                        class="mx-auto h-10 w-10 text-muted-foreground/40 mb-3"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                    >
                        <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="1.5"
                            d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"
                        />
                    </svg>
                    <p class="text-sm text-muted-foreground">
                        No packages found
                    </p>
                </div>
            {/each}
        </div>

        <!-- Desktop: table -->
        <div class="hidden md:block overflow-auto max-h-[65vh]">
            <table class="w-full min-w-120">
                <thead>
                    <tr
                        class="bg-table-header sticky top-0 z-10 [box-shadow:0_1px_0_var(--border)] whitespace-nowrap"
                    >
                        <th
                            scope="col"
                            class="px-4 py-2.5 text-left text-sm font-semibold text-muted-foreground"
                        >
                            <button
                                type="button"
                                onclick={() => handleSort("name")}
                                class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 cursor-pointer select-none transition-colors hover:bg-table-header-active hover:text-foreground {sortColumn ===
                                'name'
                                    ? 'bg-table-header-active text-foreground'
                                    : ''}"
                                >Package {@render sortIcon("name")}</button
                            >
                        </th>
                        <th
                            scope="col"
                            class="px-2 py-2.5 text-center text-sm font-semibold text-muted-foreground w-px whitespace-nowrap"
                        >
                            <button
                                type="button"
                                onclick={() => handleSort("host_count")}
                                class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 cursor-pointer select-none transition-colors hover:bg-table-header-active hover:text-foreground {sortColumn ===
                                'host_count'
                                    ? 'bg-table-header-active text-foreground'
                                    : ''}"
                                >Hosts {@render sortIcon("host_count")}</button
                            >
                        </th>
                        <th
                            scope="col"
                            class="px-2 py-2.5 text-left text-sm font-semibold text-muted-foreground w-px whitespace-nowrap"
                        >
                            <button
                                type="button"
                                onclick={() => handleSort("status")}
                                class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 cursor-pointer select-none transition-colors hover:bg-table-header-active hover:text-foreground {sortColumn ===
                                'status'
                                    ? 'bg-table-header-active text-foreground'
                                    : ''}"
                                >Status {@render sortIcon("status")}</button
                            >
                        </th>
                        <th
                            scope="col"
                            class="px-2 py-2.5 text-left text-sm font-semibold text-muted-foreground w-px whitespace-nowrap"
                        >
                            <button
                                type="button"
                                onclick={() => handleSort("manager")}
                                class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 cursor-pointer select-none transition-colors hover:bg-table-header-active hover:text-foreground {sortColumn ===
                                'manager'
                                    ? 'bg-table-header-active text-foreground'
                                    : ''}"
                                >Manager {@render sortIcon("manager")}</button
                            >
                        </th>
                        <th
                            scope="col"
                            class="px-2 py-2.5 text-left text-sm font-semibold text-muted-foreground whitespace-nowrap"
                        >
                            <button
                                type="button"
                                onclick={() => handleSort("available_version")}
                                class="group inline-flex items-center gap-1 h-8 rounded-md px-2.5 cursor-pointer select-none transition-colors hover:bg-table-header-active hover:text-foreground {sortColumn ===
                                'available_version'
                                    ? 'bg-table-header-active text-foreground'
                                    : ''}"
                                >Latest Version {@render sortIcon(
                                    "available_version",
                                )}</button
                            >
                        </th>
                    </tr>
                </thead>
                <tbody
                    class="divide-y divide-border {tableLoading
                        ? 'opacity-50 pointer-events-none'
                        : ''}"
                >
                    {#each packages as pkg}
                        <tr class="hover:bg-muted/20 transition-colors">
                            <td class="px-4 py-3">
                                <span class="flex items-center gap-2">
                                    <PackageIcon
                                        class="h-3.5 w-3.5 shrink-0 text-muted-foreground"
                                    />
                                    <span
                                        class="text-sm font-medium text-foreground"
                                        >{pkg.name}</span
                                    >
                                </span>
                            </td>
                            <td
                                class="px-2 py-3 w-px whitespace-nowrap text-center text-sm text-muted-foreground tabular-nums"
                            >
                                {pkg.host_count}
                            </td>
                            <td class="px-2 py-3 w-px whitespace-nowrap">
                                <PackageStatusBadge hasSecurityUpdate={pkg.has_security_update} availableVersion={pkg.available_version} updateChecked={pkg.update_checked} />
                            </td>
                            <td class="px-2 py-3 w-px whitespace-nowrap">
                                <span
                                    class="inline-flex rounded-full border px-2 py-0.5 text-xs font-medium {getManagerColor(
                                        pkg.package_manager,
                                    )}"
                                    >{getManagerLabel(
                                        pkg.package_manager,
                                    )}</span
                                >
                            </td>
                            <td
                                class="px-2 py-3 text-sm font-mono whitespace-nowrap"
                            >
                                {#if pkg.available_version}
                                    <span
                                        class="inline-flex items-center gap-1 font-medium {pkg.has_security_update
                                            ? 'text-destructive'
                                            : 'text-warning'}"
                                    >
                                        {#if pkg.has_security_update}
                                            <ShieldAlert
                                                class="h-3.5 w-3.5 shrink-0"
                                            />
                                        {:else}
                                            <ArrowUp
                                                class="h-3.5 w-3.5 shrink-0"
                                            />
                                        {/if}
                                        {pkg.available_version}
                                    </span>
                                {:else if pkg.current_version}
                                    <span class="text-muted-foreground">{pkg.current_version}</span>
                                {:else}
                                    <span class="text-muted-foreground">—</span>
                                {/if}
                            </td>
                        </tr>
                    {:else}
                        <tr>
                            <td colspan="5" class="py-16 text-center">
                                <svg
                                    class="mx-auto h-10 w-10 text-muted-foreground/40 mb-3"
                                    fill="none"
                                    stroke="currentColor"
                                    viewBox="0 0 24 24"
                                >
                                    <path
                                        stroke-linecap="round"
                                        stroke-linejoin="round"
                                        stroke-width="1.5"
                                        d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"
                                    />
                                </svg>
                                <p class="text-sm text-muted-foreground">
                                    No packages found
                                </p>
                            </td>
                        </tr>
                    {/each}
                </tbody>
            </table>
        </div>
    {/if}

    <Pagination
        {currentPage}
        {totalPages}
        totalItems={totalCount}
        pageSize={limit}
        itemLabel="packages"
        onPageChange={handlePageChange}
        onPageSizeChange={handlePageSizeChange}
        pageSizeOptions={PAGE_SIZE_OPTIONS}
    />
</div>
