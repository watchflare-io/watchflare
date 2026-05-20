<script lang="ts">
	import type { TimeRange } from '$lib/types';

	const {
		values,
		timestamps,
		timeRange,
		yMin,
		yMax,
		class: className = "",
	}: {
		values: number[];
		timestamps?: number[];
		timeRange?: TimeRange;
		yMin?: number;
		yMax?: number;
		class?: string;
	} = $props();

	const TENSION = 0.4;

	// 1.5× bucket interval per time range, in seconds — mirrors UPlotChart GAP_THRESHOLDS
	const GAP_THRESHOLDS_S: Record<string, number> = {
		"1h":  45,
		"12h": 900,
		"24h": 1350,
		"7d":  10800,
		"30d": 43200,
	};

	// Duration of each time range in seconds
	const TIME_RANGE_S: Record<string, number> = {
		"1h":  3600,
		"12h": 43200,
		"24h": 86400,
		"7d":  604800,
		"30d": 2592000,
	};

	let canvas: HTMLCanvasElement | null = $state(null);

	// Per-instance cache: resolving a color creates a temporary canvas — only do it once per value.
	// Modern browsers may return oklch()/lab() from getComputedStyle instead of rgb(),
	// so we let the canvas 2D context parse the color and sample the resulting sRGB bytes.
	const colorCache = new Map<string, [number, number, number, number]>();

	function resolveColor(cssColor: string): [number, number, number, number] {
		if (colorCache.has(cssColor)) return colorCache.get(cssColor)!;
		const tmp = document.createElement('canvas');
		tmp.width = tmp.height = 1;
		const tmpCtx = tmp.getContext('2d')!;
		tmpCtx.fillStyle = cssColor;
		tmpCtx.fillRect(0, 0, 1, 1);
		const [r, g, b, a] = tmpCtx.getImageData(0, 0, 1, 1).data;
		const result: [number, number, number, number] = [r, g, b, a / 255];
		colorCache.set(cssColor, result);
		return result;
	}

	function catmullRomPath(ctx: CanvasRenderingContext2D, s: { x: number; y: number }[]) {
		ctx.moveTo(s[0].x, s[0].y);
		for (let i = 0; i < s.length - 1; i++) {
			const p0 = s[Math.max(0, i - 1)];
			const p1 = s[i];
			const p2 = s[i + 1];
			const p3 = s[Math.min(s.length - 1, i + 2)];
			ctx.bezierCurveTo(
				p1.x + (p2.x - p0.x) * TENSION,
				p1.y + (p2.y - p0.y) * TENSION,
				p2.x - (p3.x - p1.x) * TENSION,
				p2.y - (p3.y - p1.y) * TENSION,
				p2.x,
				p2.y,
			);
		}
	}

	function redraw(
		el: HTMLCanvasElement,
		vals: number[],
		ts: number[] | undefined,
		tr: TimeRange | undefined,
		fixedMin: number | undefined,
		fixedMax: number | undefined,
	) {
		const dpr = window.devicePixelRatio || 1;
		const W = el.offsetWidth;
		const H = el.offsetHeight;
		if (W === 0 || H === 0) return;

		// Resize backing store only when needed
		const pw = Math.round(W * dpr);
		const ph = Math.round(H * dpr);
		if (el.width !== pw || el.height !== ph) {
			el.width = pw;
			el.height = ph;
		}

		const ctx = el.getContext('2d')!;
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		ctx.clearRect(0, 0, W, H);

		if (vals.length < 2) return;

		const [r, g, b, a] = resolveColor(getComputedStyle(el).color);
		const strokeColor = `rgba(${r},${g},${b},${a})`;
		const gradTop    = `rgba(${r},${g},${b},${a * 0.35})`;
		const gradBottom = `rgba(${r},${g},${b},0)`;
		const min = fixedMin ?? Math.min(...vals);
		const max = fixedMax ?? Math.max(...vals);
		const range = max - min || 1;

		const duration = tr ? TIME_RANGE_S[tr] : null;
		const useTime = ts && ts.length === vals.length && duration != null;
		const xStart = useTime ? ts![ts!.length - 1] - duration! : 0;

		const pts = vals.map((v, i) => ({
			x: useTime
				? Math.max(0, Math.min(W, ((ts![i] - xStart) / duration!) * W))
				: (i / (vals.length - 1)) * W,
			y: H - ((v - min) / range) * (H * 0.85) - H * 0.075,
		}));

		// Gap detection: use time-range-aware threshold when available,
		// otherwise fall back to 2× average interval heuristic.
		const gapAfter = new Set<number>();
		if (ts && ts.length === vals.length && ts.length > 1) {
			const threshold = tr && GAP_THRESHOLDS_S[tr]
				? GAP_THRESHOLDS_S[tr]
				: (ts[ts.length - 1] - ts[0]) / (ts.length - 1) * 2;
			for (let i = 0; i < ts.length - 1; i++) {
				if (ts[i + 1] - ts[i] > threshold) gapAfter.add(i);
			}
		}

		// Split into continuous segments; collect isolated single points as dots
		const segments: { x: number; y: number }[][] = [];
		const dots: { x: number; y: number }[] = [];
		let seg: { x: number; y: number }[] = [];
		for (let i = 0; i < pts.length; i++) {
			seg.push(pts[i]);
			if (gapAfter.has(i)) {
				if (seg.length >= 2) segments.push(seg);
				else if (seg.length === 1) dots.push(seg[0]);
				seg = [];
			}
		}
		if (seg.length >= 2) segments.push(seg);
		else if (seg.length === 1) dots.push(seg[0]);

		// Draw gradient fill under each segment
		const grad = ctx.createLinearGradient(0, 0, 0, H);
		grad.addColorStop(0, gradTop);
		grad.addColorStop(1, gradBottom);
		ctx.fillStyle = grad;
		for (const s of segments) {
			ctx.beginPath();
			catmullRomPath(ctx, s);
			ctx.lineTo(s[s.length - 1].x, H);
			ctx.lineTo(s[0].x, H);
			ctx.closePath();
			ctx.fill();
		}

		// Draw lines
		ctx.strokeStyle = strokeColor;
		ctx.lineWidth = 1;
		ctx.lineCap = 'round';
		ctx.lineJoin = 'round';
		for (const s of segments) {
			ctx.beginPath();
			catmullRomPath(ctx, s);
			ctx.stroke();
		}

		// Draw isolated dots
		ctx.fillStyle = strokeColor;
		for (const dot of dots) {
			ctx.beginPath();
			ctx.arc(dot.x, dot.y, 1, 0, Math.PI * 2);
			ctx.fill();
		}
	}

	// Redraw when props change — RAF batches rapid successive updates into one frame
	$effect(() => {
		const el   = canvas;
		const vals = values;
		const ts   = timestamps;
		const tr   = timeRange;
		const fMin = yMin;
		const fMax = yMax;

		const id = requestAnimationFrame(() => {
			if (el) redraw(el, vals, ts, tr, fMin, fMax);
		});
		return () => cancelAnimationFrame(id);
	});

	// Redraw on container resize (window resize, sidebar toggle, etc.)
	$effect(() => {
		if (!canvas) return;
		const el = canvas;
		const ro = new ResizeObserver(() => {
			redraw(el, values, timestamps, timeRange, yMin, yMax);
		});
		ro.observe(el);
		return () => ro.disconnect();
	});
</script>

<canvas
	bind:this={canvas}
	class="w-full h-8 {className}"
	aria-hidden="true"
></canvas>
