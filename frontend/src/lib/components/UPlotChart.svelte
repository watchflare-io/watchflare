<script lang="ts">
    import { onMount, onDestroy } from "svelte";
    import uPlot from "uplot";
    import "uplot/dist/uPlot.min.css";
    import { formatTooltipDate } from "$lib/chart-utils";
    import { userStore } from "$lib/stores/user";
    import type { TimeRange } from "$lib/types";

    const TIME_RANGE_SECONDS: Record<string, number> = {
        "1h": 3600,
        "12h": 43200,
        "24h": 86400,
        "7d": 604800,
        "30d": 2592000,
    };

    // How often to tick the x-axis forward (ms)
    const TICK_INTERVALS: Record<string, number> = {
        "1h": 30_000,
        "12h": 600_000,
        "24h": 1_200_000,
        "7d": 7_200_000,
        "30d": 36_000_000,
    };

    // Max gap before breaking the line (per time range)
    // Set to 1.5× the bucket interval so a single missing point creates a gap
    const GAP_THRESHOLDS: Record<string, number> = {
        "1h": 45, // 1.5 × 30s
        "12h": 900, // 1.5 × 10min
        "24h": 1350, // 1.5 × 15min
        "7d": 10800, // 1.5 × 2h
        "30d": 43200, // 1.5 × 8h
    };

    let {
        data,
        series,
        axes,
        scales,
        timeRange,
    }: {
        data: uPlot.AlignedData;
        series: uPlot.Series[];
        axes?: uPlot.Axis[];
        scales?: uPlot.Scales;
        timeRange?: TimeRange;
    } = $props();

    const timeFormat = $derived(
        ($userStore.user?.time_format ?? "24h") as "12h" | "24h",
    );

    let container: HTMLDivElement | undefined = $state(undefined);
    let chart: uPlot | null = null;
    let chartReady = $state(false);
    let mounted = $state(false);
    let rawMouseTop = 0;
    let resizeObserver: ResizeObserver | null = null;

    // Module-level color cache shared across all UPlotChart instances
    // Invalidated on theme change (class attribute mutation on <html>)
    const colorCache = new Map<string, string>();
    let cacheObserver: MutationObserver | null = null;

    if (typeof window !== "undefined" && !cacheObserver) {
        cacheObserver = new MutationObserver(() => colorCache.clear());
        cacheObserver.observe(document.documentElement, {
            attributes: true,
            attributeFilter: ["class"],
        });
    }

    // oklch(L C H) → '#rrggbb' via oklab → linear-sRGB → sRGB.
    // Used when the canvas can't resolve oklch (iOS Safari canvas lags CSS support).
    function oklchToHex(L: number, C: number, H: number): string {
        const h = (H * Math.PI) / 180;
        const a = C * Math.cos(h);
        const b = C * Math.sin(h);
        const l_ = L + 0.3963377774 * a + 0.2158037573 * b;
        const m_ = L - 0.1055613458 * a - 0.0638541728 * b;
        const s_ = L - 0.0894841775 * a - 1.2914855480 * b;
        const rl = 4.0767416621 * l_ ** 3 - 3.3077115913 * m_ ** 3 + 0.2309699292 * s_ ** 3;
        const gl = -1.2684380046 * l_ ** 3 + 2.6097574011 * m_ ** 3 - 0.3413193965 * s_ ** 3;
        const bl = -0.0041960863 * l_ ** 3 - 0.7034186147 * m_ ** 3 + 1.7076147010 * s_ ** 3;
        const gamma = (c: number) => {
            const abs = Math.abs(c);
            return abs <= 0.0031308 ? c * 12.92 : Math.sign(c) * (1.055 * abs ** (1 / 2.4) - 0.055);
        };
        const toInt = (c: number) => Math.max(0, Math.min(255, Math.round(gamma(c) * 255)));
        return "#" + [toInt(rl), toInt(gl), toInt(bl)].map(n => n.toString(16).padStart(2, "0")).join("");
    }

    // Resolve a CSS color or variable to '#rrggbb' (cached).
    // 1. getComputedStyle rgb/rgba  → parse directly (Safari/iOS serialises oklch→rgb)
    // 2. getComputedStyle oklch     → manual math   (iOS canvas can't parse oklch)
    // 3. anything else              → canvas readback (Chrome returns #hex from oklch)
    function resolveColor(color: string): string {
        const cached = colorCache.get(color);
        if (cached) return cached;
        const el = document.createElement("div");
        el.style.color = color;
        document.body.appendChild(el);
        const computed = getComputedStyle(el).color;
        document.body.removeChild(el);

        function rgbToHex(m: RegExpMatchArray): string {
            return "#" + [m[1], m[2], m[3]].map(n => parseInt(n).toString(16).padStart(2, "0")).join("");
        }

        let resolved: string;

        const rgbMatch = computed.match(/^rgba?\((\d+),\s*(\d+),\s*(\d+)/);
        if (rgbMatch) {
            // Safari/iOS: getComputedStyle converts oklch → rgb
            resolved = rgbToHex(rgbMatch);
        } else {
            const oklchMatch = computed.match(/^oklch\(([\d.e+-]+)\s+([\d.e+-]+)\s+([\d.e+-]+)/i);
            if (oklchMatch) {
                // Chrome/modern browsers keep oklch in getComputedStyle;
                // avoid canvas since iOS canvas may not support oklch fillStyle
                resolved = oklchToHex(parseFloat(oklchMatch[1]), parseFloat(oklchMatch[2]), parseFloat(oklchMatch[3]));
            } else {
                // Fallback: canvas (handles named colors, hsl, etc.)
                const ctx = document.createElement("canvas").getContext("2d")!;
                ctx.fillStyle = computed;
                const fb = ctx.fillStyle;
                const fbMatch = fb.match(/^rgba?\((\d+),\s*(\d+),\s*(\d+)/);
                resolved = fbMatch ? rgbToHex(fbMatch) : (fb.startsWith("#") ? fb : "#000000");
            }
        }

        colorCache.set(color, resolved);
        return resolved;
    }

    function tooltipPlugin(): uPlot.Plugin {
        let tooltip: HTMLDivElement;

        function init(u: uPlot) {
            tooltip = document.createElement("div");
            tooltip.style.cssText = `
				display: none;
				position: absolute;
				z-index: 50;
				pointer-events: none;
				background: var(--color-popover);
				color: var(--color-popover-foreground);
				border: 1px solid var(--color-border);
				border-radius: 0.5rem;
				padding: 0.5rem 0.75rem;
				font-size: 0.75rem;
				line-height: 1.25rem;
				box-shadow: 0 4px 6px -1px rgb(0 0 0 / 0.1);
				white-space: nowrap;
			`;
            u.over.parentElement!.appendChild(tooltip);

            // Track mouse Y for tooltip vertical positioning
            const over = u.over;
            over.addEventListener("mousemove", (e) => {
                rawMouseTop = e.offsetY;
            });
            let touchStartX = 0;
            let touchStartY = 0;
            let isHorizontal: boolean | null = null;

            over.addEventListener(
                "touchstart",
                (e) => {
                    const t = e.touches[0];
                    touchStartX = t.clientX;
                    touchStartY = t.clientY;
                    isHorizontal = null;
                },
                { passive: true },
            );

            over.addEventListener(
                "touchmove",
                (e) => {
                    const t = e.touches[0];
                    if (isHorizontal === null) {
                        const dx = Math.abs(t.clientX - touchStartX);
                        const dy = Math.abs(t.clientY - touchStartY);
                        if (dx < 5 && dy < 5) return;
                        isHorizontal = dx > dy;
                    }
                    if (!isHorizontal) return;
                    e.preventDefault();
                    const rect = over.getBoundingClientRect();
                    const left = t.clientX - rect.left;
                    const top = t.clientY - rect.top;
                    u.setCursor({ left, top });
                },
                { passive: false },
            );

            over.addEventListener(
                "touchend",
                () => {
                    isHorizontal = null;
                    u.setCursor({ left: -10, top: -10 });
                },
                { passive: true },
            );
        }

        function setCursor(u: uPlot) {
            const idx = u.cursor.idx;
            if (idx == null || idx < 0) {
                tooltip.style.display = "none";
                return;
            }

            const ts = u.data[0][idx];
            let html = `<div style="margin-bottom:4px;font-weight:500;">${formatTooltipDate(new Date(ts * 1000), timeFormat)}</div>`;

            let hasValue = false;
            for (let i = 1; i < u.series.length; i++) {
                const s = u.series[i];
                if (!s.show) continue;
                const val = u.data[i][idx];
                if (val == null) continue;
                hasValue = true;
                const color = (s as any)._stroke ?? s.stroke ?? "#888";
                const formatted = s.value
                    ? (
                          s.value as (
                              u: uPlot,
                              v: number | null,
                              si: number,
                              i: number | null,
                          ) => string
                      )(u, val, i, idx)
                    : String(val);
                html += `<div style="display:flex;align-items:center;gap:6px;">
					<span style="width:8px;height:8px;border-radius:2px;background:${color};display:inline-block;"></span>
					<span style="color:var(--color-muted-foreground);">${s.label}:</span>
					<span style="font-weight:500;">${formatted}</span>
				</div>`;
            }

            if (!hasValue) {
                tooltip.style.display = "none";
                return;
            }

            tooltip.innerHTML = html;
            tooltip.style.display = "block";

            const left = u.cursor.left ?? 0;
            const plotLeft = u.bbox.left / devicePixelRatio;
            const plotWidth = u.bbox.width / devicePixelRatio;

            let tipX = left + plotLeft + 12;
            let tipY = rawMouseTop - tooltip.offsetHeight / 2;

            if (tipX + tooltip.offsetWidth > plotLeft + plotWidth) {
                tipX = left + plotLeft - tooltip.offsetWidth - 12;
            }

            tooltip.style.left = tipX + "px";
            tooltip.style.top = Math.max(0, tipY) + "px";
        }

        return {
            hooks: {
                init: [init],
                setCursor: [setCursor],
            },
        };
    }

    // Plugin: draw dots for isolated points (both neighbors are gaps or absent)
    function isolatedPointsPlugin(): uPlot.Plugin {
        return {
            hooks: {
                draw: [
                    (u: uPlot) => {
                        const threshold = timeRange
                            ? GAP_THRESHOLDS[timeRange]
                            : null;
                        if (!threshold) return;

                        const ctx = u.ctx;
                        const dpr = devicePixelRatio;
                        const radius = 1.5 * dpr;
                        const timestamps = u.data[0];

                        for (let si = 1; si < u.series.length; si++) {
                            const s = u.series[si];
                            if (!s.show) continue;
                            const vals = u.data[si];
                            const color =
                                (s as any)._stroke ??
                                (s.stroke as string) ??
                                "#888";

                            ctx.save();
                            ctx.fillStyle = color;
                            ctx.beginPath();

                            for (let i = 0; i < vals.length; i++) {
                                if (vals[i] == null) continue;
                                // Check if previous real value has a gap
                                const prevGap =
                                    i === 0 ||
                                    vals[i - 1] == null ||
                                    timestamps[i] - timestamps[i - 1] >
                                        threshold;
                                const nextGap =
                                    i === vals.length - 1 ||
                                    vals[i + 1] == null ||
                                    timestamps[i + 1] - timestamps[i] >
                                        threshold;
                                if (!prevGap || !nextGap) continue;

                                const cx = Math.round(
                                    u.valToPos(timestamps[i], "x", true),
                                );
                                const cy = Math.round(
                                    u.valToPos(
                                        vals[i] as number,
                                        s.scale!,
                                        true,
                                    ),
                                );
                                ctx.moveTo(cx + radius, cy);
                                ctx.arc(cx, cy, radius, 0, Math.PI * 2);
                            }

                            ctx.fill();
                            ctx.restore();
                        }
                    },
                ],
            },
        };
    }

    // Plugin: custom cursor line + hover points (all positioned with same valToPos)
    function cursorOverlayPlugin(): uPlot.Plugin {
        let line: HTMLDivElement;
        let dots: HTMLDivElement[] = [];

        return {
            hooks: {
                init: [
                    (u: uPlot) => {
                        line = document.createElement("div");
                        line.style.cssText = `
						position: absolute;
						top: 0;
						left: 0;
						bottom: 0;
						width: 1px;
						border-left: 1px dashed var(--color-muted-foreground);
						background: none;
						pointer-events: none;
						display: none;
					`;
                        u.over.appendChild(line);

                        // Create a dot for each data series
                        for (let si = 1; si < u.series.length; si++) {
                            const dot = document.createElement("div");
                            dot.style.cssText = `
							position: absolute;
							width: 9px;
							height: 9px;
							border-radius: 50%;
							pointer-events: none;
							display: none;
							transform: translate(-4.5px, -4.5px);
						`;
                            u.over.appendChild(dot);
                            dots.push(dot);
                        }
                    },
                ],
                setCursor: [
                    (u: uPlot) => {
                        const idx = u.cursor.idx;
                        if (idx == null || idx < 0) {
                            line.style.display = "none";
                            for (const dot of dots) dot.style.display = "none";
                            return;
                        }

                        const x = u.valToPos(u.data[0][idx], "x");
                        line.style.display = "";
                        line.style.transform = `translateX(${x - 0.5}px)`;

                        for (let si = 1; si < u.series.length; si++) {
                            const s = u.series[si];
                            const dot = dots[si - 1];
                            const val = u.data[si][idx];
                            if (!s.show || val == null) {
                                dot.style.display = "none";
                                continue;
                            }
                            const y = u.valToPos(val as number, s.scale!);
                            const color =
                                (s as any)._stroke ??
                                (s.stroke as string) ??
                                "#888";
                            dot.style.display = "";
                            dot.style.background = color;
                            dot.style.left = x + "px";
                            dot.style.top = y + "px";
                        }
                    },
                ],
            },
        };
    }

    // Native uPlot gaps function: reads timeRange reactively at each call
    const gapsFn = (
        u: uPlot,
        seriesIdx: number,
        idx0: number,
        idx1: number,
        nullGaps: [number, number][],
    ): [number, number][] => {
        const threshold = timeRange ? GAP_THRESHOLDS[timeRange] : null;
        if (!threshold) return nullGaps;
        const gaps: [number, number][] = [...nullGaps];
        const timestamps = u.data[0];
        const vals = u.data[seriesIdx];

        for (let i = idx0 + 1; i <= idx1; i++) {
            if (vals[i] == null || vals[i - 1] == null) continue;
            if (timestamps[i] - timestamps[i - 1] > threshold) {
                const leftPx = Math.round(
                    u.valToPos(timestamps[i - 1], "x", true),
                );
                const rightPx = Math.round(
                    u.valToPos(timestamps[i], "x", true),
                );
                gaps.push([leftPx, rightPx]);
            }
        }

        return gaps;
    };

    function buildOpts(width: number, chartHeight: number): uPlot.Options {
        const resolvedSeries: uPlot.Series[] = series.map((s) => {
            const resolved: uPlot.Series = { ...s };
            if (s.stroke) resolved.stroke = resolveColor(s.stroke as string);
            if (s.fill) {
                const hex = resolveColor(s.fill as string);
                const r = parseInt(hex.slice(1, 3), 16);
                const g = parseInt(hex.slice(3, 5), 16);
                const b = parseInt(hex.slice(5, 7), 16);
                resolved.fill = `rgba(${r},${g},${b},0.2)`;
            }
            resolved.gaps = gapsFn;
            resolved.points = { show: false };
            return resolved;
        });

        const gridStroke = resolveColor("var(--border)");
        const textColor = resolveColor("var(--muted-foreground)");

        const defaultAxes: uPlot.Axis[] = [
            {
                stroke: textColor,
                grid: { stroke: gridStroke, width: 1 },
                ticks: { stroke: gridStroke, width: 1 },
                font: "11px system-ui",
                space: timeFormat === "12h" ? 70 : 50,
                values: (_u: uPlot, ticks: number[]) =>
                    ticks.map((t) => {
                        const d = new Date(t * 1000);
                        if (timeRange === "7d" || timeRange === "30d") {
                            return d.toLocaleDateString("en-US", {
                                day: "numeric",
                                month: "short",
                            });
                        }
                        if (timeFormat === "12h") {
                            let h = d.getHours();
                            const ampm = h >= 12 ? "PM" : "AM";
                            h = h % 12 || 12;
                            const mm = String(d.getMinutes()).padStart(2, "0");
                            return `${h}:${mm} ${ampm}`;
                        }
                        const hh = String(d.getHours()).padStart(2, "0");
                        const mm = String(d.getMinutes()).padStart(2, "0");
                        return `${hh}:${mm}`;
                    }),
            },
            {
                stroke: textColor,
                grid: { stroke: gridStroke, width: 1 },
                ticks: { stroke: gridStroke, width: 1 },
                font: "11px system-ui",
                splits: (u: uPlot) => {
                    const min = (u.scales.y as any).min ?? 0;
                    const max = (u.scales.y as any).max ?? 100;
                    if (max <= min) return [min];
                    const step = (max - min) / 4;
                    return [
                        min,
                        min + step,
                        min + step * 2,
                        min + step * 3,
                        max,
                    ];
                },
            },
        ];

        const mergedAxes = axes
            ? defaultAxes.map((def, i) =>
                  axes[i] ? { ...def, ...axes[i] } : def,
              )
            : defaultAxes;

        const defaultYScale: uPlot.Scales = {
            y: {
                range: (_u: uPlot, _min: number, max: number) => {
                    if (max <= 0) return [0, 1] as uPlot.Range.MinMax;
                    const units = [1, 1024, 1024 ** 2, 1024 ** 3];
                    let unit = 1;
                    for (const u of units) {
                        if (max >= u) unit = u;
                    }
                    const displayed = max / unit;
                    const mag = Math.pow(10, Math.floor(Math.log10(displayed)));
                    const normalized = displayed / mag;
                    const niceMaxes = [1, 2, 4, 5, 8, 10];
                    const niceNorm =
                        niceMaxes.find((n) => n >= normalized) || 10;
                    return [0, niceNorm * mag * unit] as uPlot.Range.MinMax;
                },
            },
        };

        const xScale: uPlot.Scales =
            timeRange && TIME_RANGE_SECONDS[timeRange]
                ? {
                      x: {
                          range: (): uPlot.Range.MinMax => {
                              // Use the later of browser clock and last data timestamp
                              // to tolerate clock skew between browser and server
                              const browserNow = Math.floor(Date.now() / 1000);
                              const lastDataTs =
                                  data?.[0]?.length > 0
                                      ? data[0][data[0].length - 1]
                                      : browserNow;
                              const now = Math.max(browserNow, lastDataTs);
                              return [
                                  now - TIME_RANGE_SECONDS[timeRange!],
                                  now,
                              ];
                          },
                      },
                  }
                : {};

        const mergedScales: uPlot.Scales = {
            ...xScale,
            ...(scales || defaultYScale),
        };

        return {
            width,
            height: chartHeight,
            cursor: {
                drag: { x: false, y: false },
                y: false,
                x: false,
                points: { show: false },
            },
            legend: { show: false },
            series: [{}, ...resolvedSeries],
            axes: mergedAxes,
            scales: mergedScales,
            plugins: [
                tooltipPlugin(),
                cursorOverlayPlugin(),
                isolatedPointsPlugin(),
            ],
        };
    }

    let lastWidth = 0;
    let lastHeight = 0;

    function createChart() {
        destroyChart();
        if (!container) return;
        if (!data || !data[0] || data[0].length === 0) return;
        const width = container.clientWidth;
        const height = container.clientHeight;
        if (width === 0 || height === 0) return;
        lastWidth = width;
        lastHeight = height;

        chart = new uPlot(buildOpts(width, height), data, container);
        chartReady = true;
    }

    function destroyChart() {
        if (chart) {
            chart.destroy();
            chart = null;
            chartReady = false;
        }
    }

    // Track series identity for recreation
    let seriesKey = $derived(series.map((s) => s.label).join(","));

    // Track scales for recreation (timeRange excluded: handled via setData + reactive range fn)
    let scalesKey = $derived(JSON.stringify(scales || {}));
    let prevScalesKey = "";
    let prevTimeFormat = "";

    // When data, series, scales, or timeFormat change, update or recreate the chart
    $effect(() => {
        const _data = data;
        const _key = seriesKey;
        const _scales = scalesKey;
        const _timeFormat = timeFormat;

        if (!mounted || !container) return;

        const scalesChanged = _scales !== prevScalesKey;
        const timeFormatChanged = _timeFormat !== prevTimeFormat;
        prevScalesKey = _scales;
        prevTimeFormat = _timeFormat;

        if (
            chart &&
            _data &&
            _data[0].length > 0 &&
            _data.length === chart.series.length &&
            !scalesChanged &&
            !timeFormatChanged
        ) {
            chart.setData(_data);
        } else if (_data && _data[0].length > 0) {
            createChart();
        }
    });

    // Tick the x-axis forward on a wall-clock timer (re-creates interval when chart or timeRange changes)
    $effect(() => {
        if (!chartReady || !chart || !timeRange) return;
        const tickMs = TICK_INTERVALS[timeRange];
        if (!tickMs) return;
        const id = setInterval(() => {
            if (!chart) return;
            const browserNow = Math.floor(Date.now() / 1000);
            const lastDataTs =
                data?.[0]?.length > 0
                    ? data[0][data[0].length - 1]
                    : browserNow;
            const now = Math.max(browserNow, lastDataTs);
            chart.setScale("x", { min: now - TIME_RANGE_SECONDS[timeRange!], max: now });
        }, tickMs);
        return () => clearInterval(id);
    });

    onMount(() => {
        mounted = true;
        createChart();

        resizeObserver = new ResizeObserver(() => {
            if (!chart || !container) return;
            const width = container.clientWidth;
            const height = container.clientHeight;
            if (
                width > 0 &&
                height > 0 &&
                (width !== lastWidth || height !== lastHeight)
            ) {
                lastWidth = width;
                lastHeight = height;
                chart.setSize({ width, height });
            }
        });

        // Recreate chart on theme change (colors are baked into canvas)
        const themeObserver = new MutationObserver(() => {
            createChart();
        });
        themeObserver.observe(document.documentElement, {
            attributes: true,
            attributeFilter: ["class"],
        });

        return () => {
            resizeObserver?.disconnect();
            resizeObserver = null;
            themeObserver.disconnect();
        };
    });

    // Attach resizeObserver when container becomes available.
    // container lives inside {#if hasData}, so it may appear after onMount
    // (e.g. when the first SSE data arrives). Using $effect ensures we observe
    // it regardless of when it is bound.
    $effect(() => {
        if (container && resizeObserver) {
            resizeObserver.observe(container);
        }
    });

    onDestroy(() => {
        destroyChart();
        cacheObserver?.disconnect();
    });

    const TIME_RANGE_LABELS: Record<string, string> = {
        "1h": "1 hour",
        "12h": "12 hours",
        "24h": "24 hours",
        "7d": "7 days",
        "30d": "30 days",
    };

    let hasData = $derived(data && data[0] && data[0].length > 0);
</script>

<div class="-mx-2 sm:mx-0 h-48 sm:h-64 relative">
    {#if hasData}
        <div class="absolute inset-0" bind:this={container}></div>
    {:else}
        <div
            class="absolute inset-0 flex items-center justify-center text-sm text-muted-foreground"
        >
            {#if timeRange && TIME_RANGE_LABELS[timeRange]}
                Not enough data for {TIME_RANGE_LABELS[timeRange]} view
            {:else}
                No data available
            {/if}
        </div>
    {/if}
</div>
