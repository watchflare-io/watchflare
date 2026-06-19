<script lang="ts">
    import { onMount, flushSync } from "svelte";
    import { goto } from "$app/navigation";
    import { Search, Server, Package as PackageIcon } from "lucide-svelte";
    import * as api from "$lib/api.js";
    import type { Host as HostType, GlobalPackage } from "$lib/types";

    const QUICK_HOSTS_LIMIT = 10;
    const SEARCH_HOSTS_LIMIT = 10;
    const SEARCH_PACKAGES_LIMIT = 10;

    let open = $state(false);
    let query = $state("");
    let hostResults: HostType[] = $state([]);
    let packageResults: GlobalPackage[] = $state([]);
    let quickHosts: HostType[] = $state([]);
    let loading = $state(false);
    let selectedIndex = $state(-1);
    let searchTimeout: ReturnType<typeof setTimeout> | null = null;
    let inputRef = $state<HTMLInputElement | null>(null);
    let listRef = $state<HTMLDivElement | null>(null);

    const isIOS =
        typeof navigator !== "undefined" &&
        /iPad|iPhone|iPod/.test(navigator.userAgent) &&
        !(window as any).MSStream;

    function updateViewportHeight() {
        const vh = window.visualViewport
            ? window.visualViewport.height
            : window.innerHeight;
        document.documentElement.style.setProperty(
            "--sp-viewport-height",
            vh + "px",
        );
    }

    function preventTouchMove(e: TouchEvent) {
        const scrollable = (e.target as HTMLElement).closest<HTMLElement>(
            ".sp-scrollable",
        );
        if (scrollable && scrollable.scrollHeight > scrollable.clientHeight)
            return;
        e.preventDefault();
    }

    function lockScroll() {
        if (isIOS) {
            document.addEventListener("touchmove", preventTouchMove, {
                passive: false,
            });
            updateViewportHeight();
            window.visualViewport?.addEventListener(
                "resize",
                updateViewportHeight,
            );
        } else {
            document.body.style.overflow = "hidden";
        }
    }

    function unlockScroll() {
        if (isIOS) {
            document.removeEventListener("touchmove", preventTouchMove);
            window.visualViewport?.removeEventListener(
                "resize",
                updateViewportHeight,
            );
        } else {
            document.body.style.overflow = "";
        }
    }

    const hasResults = $derived(
        hostResults.length > 0 || packageResults.length > 0,
    );
    const resultCount = $derived(hostResults.length + packageResults.length);

    // Flat list of navigable items for keyboard nav
    type NavItem =
        | { type: "host"; id: string }
        | { type: "package"; name: string };

    const navItems = $derived<NavItem[]>(
        query.trim()
            ? [
                  ...hostResults.map((h) => ({
                      type: "host" as const,
                      id: h.id,
                  })),
                  ...packageResults.map((p) => ({
                      type: "package" as const,
                      name: p.name,
                  })),
              ]
            : quickHosts.map((h) => ({ type: "host" as const, id: h.id })),
    );

    // Reset selection when results change
    $effect(() => {
        // eslint-disable-next-line @typescript-eslint/no-unused-expressions
        navItems;
        selectedIndex = -1;
    });

    function scrollSelectedIntoView() {
        if (!listRef || selectedIndex < 0) return;
        const items = listRef.querySelectorAll<HTMLElement>("[data-sp-item]");
        items[selectedIndex]?.scrollIntoView({ block: "nearest" });
    }

    function handleKeydown(e: KeyboardEvent) {
        if (navItems.length === 0) return;
        if (e.key === "ArrowDown") {
            e.preventDefault();
            selectedIndex = (selectedIndex + 1) % navItems.length;
            scrollSelectedIntoView();
        } else if (e.key === "ArrowUp") {
            e.preventDefault();
            selectedIndex =
                selectedIndex <= 0 ? navItems.length - 1 : selectedIndex - 1;
            scrollSelectedIntoView();
        } else if (e.key === "Enter" && selectedIndex >= 0) {
            e.preventDefault();
            const item = navItems[selectedIndex];
            if (item.type === "host") handleSelectHost(item.id);
            else handleSelectPackage(item.name);
        }
    }

    function fmtCount(count: number, limit: number): string {
        return count >= limit ? `${count}+` : `${count}`;
    }

    function openPalette() {
        flushSync(() => {
            open = true;
        });
        lockScroll();
        inputRef?.focus();
        if (quickHosts.length === 0) {
            api.listHosts({ perPage: QUICK_HOSTS_LIMIT })
                .then((r) => {
                    quickHosts = r.hosts ?? [];
                })
                .catch(() => {});
        }
    }

    function closePalette() {
        open = false;
        unlockScroll();
        clearState();
    }

    onMount(() => {
        function handleGlobalKeydown(e: KeyboardEvent) {
            if (e.key === "Escape" && open) {
                e.preventDefault();
                closePalette();
            }
        }
        window.addEventListener("watchflare:open-search", openPalette);
        window.addEventListener("keydown", handleGlobalKeydown);
        return () => {
            window.removeEventListener("watchflare:open-search", openPalette);
            window.removeEventListener("keydown", handleGlobalKeydown);
        };
    });

    function handleInput() {
        if (searchTimeout) clearTimeout(searchTimeout);
        if (!query.trim()) {
            hostResults = [];
            packageResults = [];
            loading = false;
            return;
        }
        loading = true;
        searchTimeout = setTimeout(async () => {
            try {
                const [hostsResult, packagesResult] = await Promise.allSettled([
                    api.listHosts({
                        search: query,
                        perPage: SEARCH_HOSTS_LIMIT,
                    }),
                    api.listAllPackages({
                        q: query,
                        limit: SEARCH_PACKAGES_LIMIT,
                    }),
                ]);
                hostResults =
                    hostsResult.status === "fulfilled"
                        ? (hostsResult.value.hosts ?? [])
                        : [];
                packageResults =
                    packagesResult.status === "fulfilled"
                        ? (packagesResult.value.packages ?? [])
                        : [];
            } finally {
                loading = false;
            }
        }, 200);
    }

    function clearState() {
        query = "";
        hostResults = [];
        packageResults = [];
    }

    function handleSelectHost(hostId: string) {
        closePalette();
        goto(`/hosts/${hostId}`);
    }

    function handleSelectPackage(name: string) {
        closePalette();
        goto(`/packages?q=${encodeURIComponent(name)}`);
    }

    function getStatusDot(status: string): string {
        switch (status) {
            case "online":
                return "bg-success";
            case "offline":
                return "bg-muted-foreground";
            default:
                return "bg-warning";
        }
    }
</script>

<!-- Always in DOM — display:none hides it while keeping input accessible for synchronous focus() -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
    role="dialog"
    aria-modal="true"
    aria-label="Search"
    style:display={open ? "flex" : "none"}
    class="fixed inset-0 z-50 items-start justify-center px-3 pt-4 sm:pt-20 touch-none"
>
    <!-- Backdrop -->
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <div
        class="absolute inset-0 bg-black/60 backdrop-blur-sm"
        role="presentation"
        onclick={closePalette}
    ></div>

    <!-- Panel -->
    <div
        class="sp-panel relative flex w-full max-w-140 flex-col overflow-hidden rounded-xl border bg-surface shadow-xl sm:max-h-[calc(100dvh-140px)]"
    >
        <!-- Input row -->
        <div
            class="flex shrink-0 items-center gap-2.5 border-b px-4 py-3.5 transition-colors focus-within:border-primary"
        >
            <Search class="h-4 w-4 shrink-0 text-muted-foreground" />
            <input
                bind:this={inputRef}
                bind:value={query}
                oninput={handleInput}
                onkeydown={handleKeydown}
                type="text"
                placeholder="Search hosts, packages..."
                autocomplete="off"
                autocorrect="off"
                spellcheck={false}
                role="combobox"
                aria-expanded={open}
                aria-controls="command-palette-results"
                aria-autocomplete="list"
                class="flex-1 bg-transparent text-base text-foreground outline-none placeholder:text-muted-foreground sm:text-[15px]"
            />
            <!-- Desktop: Esc kbd — Mobile: Cancel text -->
            <button
                type="button"
                onclick={closePalette}
                class="hidden shrink-0 items-center rounded border bg-background px-1.5 py-0.5 font-mono text-[11px] leading-relaxed text-muted-foreground sm:inline-flex"
            >
                Esc
            </button>
            <button
                type="button"
                onclick={closePalette}
                class="shrink-0 px-1 text-sm font-medium text-primary sm:hidden"
            >
                Cancel
            </button>
        </div>

        <!-- Results -->
        <div
            id="command-palette-results"
            bind:this={listRef}
            class="sp-scrollable min-h-0 flex-1 overflow-y-auto p-1.5 touch-pan-y"
        >
            {#if !query.trim()}
                {#if quickHosts.length > 0}
                    <div>
                        <p
                            class="px-2.5 py-1.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground/70"
                        >
                            Quick access
                        </p>
                        {#each quickHosts as host, i (host.id)}
                            <button
                                type="button"
                                data-sp-item
                                onclick={() => handleSelectHost(host.id)}
                                onmouseenter={() => (selectedIndex = i)}
                                class="flex w-full cursor-pointer items-center gap-3 rounded-lg px-2.5 py-2.5 text-sm transition-colors {selectedIndex ===
                                i
                                    ? 'bg-muted'
                                    : ''}"
                            >
                                <Server
                                    class="h-3.5 w-3.5 shrink-0 text-muted-foreground"
                                />
                                <div class="min-w-0 flex-1 text-left">
                                    <div class="flex items-center gap-2">
                                        <span
                                            class="truncate font-medium text-foreground"
                                            >{host.display_name}</span
                                        >
                                        <span
                                            class="h-1.5 w-1.5 shrink-0 rounded-full {getStatusDot(
                                                host.status,
                                            )}"
                                        ></span>
                                    </div>
                                    {#if host.hostname}
                                        <p
                                            class="truncate text-xs text-muted-foreground"
                                        >
                                            {host.hostname}{#if host.ip_address_v4}
                                                · {host.ip_address_v4}{/if}
                                        </p>
                                    {/if}
                                </div>
                            </button>
                        {/each}
                    </div>
                {:else}
                    <div
                        class="py-10 text-center text-sm text-muted-foreground"
                    >
                        Search hosts and packages...
                    </div>
                {/if}
            {:else if !hasResults && !loading}
                <div class="py-10 text-center text-sm text-muted-foreground">
                    No results for "{query}"
                </div>
            {:else}
                {#if hostResults.length > 0}
                    <div>
                        <p
                            class="px-2.5 py-1.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground/70"
                        >
                            Hosts
                        </p>
                        {#each hostResults as host, i (host.id)}
                            <button
                                type="button"
                                data-sp-item
                                onclick={() => handleSelectHost(host.id)}
                                onmouseenter={() => (selectedIndex = i)}
                                class="flex w-full cursor-pointer items-center gap-3 rounded-lg px-2.5 py-2.5 text-sm transition-colors {selectedIndex ===
                                i
                                    ? 'bg-muted'
                                    : ''}"
                            >
                                <Server
                                    class="h-3.5 w-3.5 shrink-0 text-muted-foreground"
                                />
                                <div class="min-w-0 flex-1 text-left">
                                    <div class="flex items-center gap-2">
                                        <span
                                            class="truncate font-medium text-foreground"
                                            >{host.display_name}</span
                                        >
                                        <span
                                            class="h-1.5 w-1.5 shrink-0 rounded-full {getStatusDot(
                                                host.status,
                                            )}"
                                        ></span>
                                    </div>
                                    {#if host.hostname}
                                        <p
                                            class="truncate text-xs text-muted-foreground"
                                        >
                                            {host.hostname}{#if host.ip_address_v4}
                                                · {host.ip_address_v4}{/if}
                                        </p>
                                    {/if}
                                </div>
                            </button>
                        {/each}
                    </div>
                {/if}
                {#if hostResults.length > 0 && packageResults.length > 0}
                    <div class="my-1 h-px bg-border"></div>
                {/if}
                {#if packageResults.length > 0}
                    <div>
                        <p
                            class="px-2.5 py-1.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground/70"
                        >
                            Packages
                        </p>
                        {#each packageResults as pkg, i (`${pkg.name}-${pkg.package_manager}`)}
                            <button
                                type="button"
                                data-sp-item
                                onclick={() => handleSelectPackage(pkg.name)}
                                onmouseenter={() =>
                                    (selectedIndex = hostResults.length + i)}
                                class="flex w-full cursor-pointer items-center gap-3 rounded-lg px-2.5 py-2.5 text-sm transition-colors {selectedIndex ===
                                hostResults.length + i
                                    ? 'bg-muted'
                                    : ''}"
                            >
                                <PackageIcon
                                    class="h-3.5 w-3.5 shrink-0 text-muted-foreground"
                                />
                                <div class="min-w-0 flex-1 text-left">
                                    <div
                                        class="flex min-w-0 items-center gap-2"
                                    >
                                        <span
                                            class="truncate font-medium text-foreground"
                                            >{pkg.name}</span
                                        >
                                        <span
                                            class="shrink-0 text-xs text-muted-foreground"
                                            >{pkg.package_manager}</span
                                        >
                                    </div>
                                    <p class="text-xs text-muted-foreground">
                                        {pkg.host_count} host{pkg.host_count !==
                                        1
                                            ? "s"
                                            : ""}
                                        {#if pkg.has_security_update}
                                            · <span class="text-destructive"
                                                >security update</span
                                            >
                                        {:else if pkg.available_version}
                                            · <span class="text-warning"
                                                >update available</span
                                            >
                                        {/if}
                                    </p>
                                </div>
                            </button>
                        {/each}
                    </div>
                {/if}
            {/if}
        </div>

        <!-- Footer -->
        {#if query.trim()}
            <div class="flex items-center gap-4 border-t px-3.5 py-2">
                {#if hasResults}
                    <span
                        class="hidden sm:inline-flex items-center gap-1.5 text-[11px] text-muted-foreground"
                    >
                        <kbd
                            class="rounded border bg-background px-1 py-px font-mono text-[10px] leading-relaxed"
                            >↑↓</kbd
                        >
                        Navigate
                    </span>
                    <span
                        class="hidden sm:inline-flex items-center gap-1.5 text-[11px] text-muted-foreground"
                    >
                        <kbd
                            class="rounded border bg-background px-1 py-px font-mono text-[10px] leading-relaxed"
                            >↵</kbd
                        >
                        Open
                    </span>
                {/if}
                <span
                    class="hidden sm:inline-flex items-center gap-1.5 text-[11px] text-muted-foreground"
                >
                    <kbd
                        class="rounded border bg-background px-1 py-px font-mono text-[10px] leading-relaxed"
                        >Esc</kbd
                    >
                    Close
                </span>
                {#if resultCount > 0}
                    <span
                        class="ml-auto font-mono text-[11px] text-muted-foreground/60"
                    >
                        {#if hostResults.length > 0 && packageResults.length > 0}
                            {fmtCount(hostResults.length, SEARCH_HOSTS_LIMIT)} host{hostResults.length !==
                            1
                                ? "s"
                                : ""} · {fmtCount(
                                packageResults.length,
                                SEARCH_PACKAGES_LIMIT,
                            )} package{packageResults.length !== 1 ? "s" : ""}
                        {:else if hostResults.length > 0}
                            {fmtCount(hostResults.length, SEARCH_HOSTS_LIMIT)} host{hostResults.length !==
                            1
                                ? "s"
                                : ""}
                        {:else}
                            {fmtCount(
                                packageResults.length,
                                SEARCH_PACKAGES_LIMIT,
                            )} package{packageResults.length !== 1 ? "s" : ""}
                        {/if}
                    </span>
                {/if}
            </div>
        {/if}
    </div>
</div>

<style>
    .sp-panel {
        animation: sp-panel-in 0.15s ease-out both;
        max-height: calc(var(--sp-viewport-height, 100dvh) - 2rem);
    }

    @media (min-width: 640px) {
        .sp-panel {
            max-height: calc(100dvh - 140px);
        }
    }

    @keyframes sp-panel-in {
        from {
            opacity: 0;
            transform: translateY(-6px) scale(0.98);
        }
        to {
            opacity: 1;
            transform: translateY(0) scale(1);
        }
    }
</style>
