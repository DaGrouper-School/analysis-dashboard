<script lang="ts">
	import { onMount } from 'svelte';
	import landDots from '#lib/data/land-dots.json';
	import { theme } from '#lib/theme.svelte.ts';

	export interface GlobeMarker {
		id: string;
		lat: number;
		lng: number;
		color: string;
		/** Hollow, dashed ring: the position is a stand-in, not the device's own GPS. */
		hollow?: boolean;
		label: string;
	}

	/**
	 * A dotted globe on a 2D canvas: land as a grid of teal dots, a soft
	 * atmosphere, and glowing markers. Drag to turn it, use the buttons or arrow keys to zoom
	 * and turn, click a marker to select it.
	 */
	let {
		markers,
		selectedId = $bindable(''),
		ariaLabel = 'Globe',
		interactive = true,
		tint
	}: {
		markers: GlobeMarker[];
		selectedId?: string;
		ariaLabel?: string;
		/** False for a decorative globe: no focus, no dragging, hidden from assistive tech. */
		interactive?: boolean;
		/** A hex color (e.g. a device type's registry color) for the land, rim and glow. */
		tint?: string;
	} = $props();

	// Canvas colors per theme: a dark sphere in space, or a pale one on paper.
	const PALETTES = {
		dark: {
			stars: true,
			glow: ['rgba(47,204,169,0.35)', 'rgba(0,189,255,0.12)', 'rgba(0,189,255,0)'],
			sphere: ['#0d1c1c', '#030807'],
			rim: 'rgba(47,204,169,0.35)',
			land: '47,204,169',
			landAlpha: [0.25, 0.65],
			dotScale: 1,
			halo: '88',
			hollowFill: '#000000',
			markerEdge: null,
			ring: '#ffffff'
		},
		light: {
			stars: false,
			glow: ['rgba(15,138,112,0.22)', 'rgba(0,123,179,0.07)', 'rgba(0,123,179,0)'],
			// Deep teal dots, nearly solid facing the viewer, on a lightly tinted sphere: the land
			// has to read against a near-white page.
			sphere: ['#f2f7f5', '#d3e3de'],
			rim: 'rgba(15,138,112,0.75)',
			land: '8,98,79',
			landAlpha: [0.55, 0.45],
			dotScale: 1.7,
			halo: '55',
			hollowFill: '#ffffff',
			markerEdge: 'rgba(11,18,21,0.55)',
			ring: '#0b1215'
		}
	};

	/** `#3cc04b` as `60,192,75`, or null for anything that is not a 6-digit hex color. */
	function rgb(hex: string | undefined) {
		const m = hex?.match(/^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i);
		return m
			? m
					.slice(1)
					.map((c) => parseInt(c, 16))
					.join(',')
			: null;
	}

	/** The theme's palette, with land, rim and glow in the tint when there is one. */
	function palette() {
		const base = PALETTES[theme.current];
		const c = rgb(tint);
		if (!c) return base;
		const strong = theme.current === 'dark' ? 0.35 : 0.22;
		return {
			...base,
			glow: [`rgba(${c},${strong})`, `rgba(${c},${strong / 3})`, `rgba(${c},0)`],
			rim: `rgba(${c},${theme.current === 'dark' ? 0.35 : 0.45})`,
			land: c
		};
	}

	const RAD = Math.PI / 180;
	const MIN_ZOOM = 0.8;
	const MAX_ZOOM = 12;

	let canvas = $state<HTMLCanvasElement>();
	let wrapper: HTMLDivElement;
	let size = { w: 0, h: 0, dpr: 1 };
	// View: the point at the centre of the globe, and the zoom.
	let view = { lat: 10, lng: 20, zoom: 1 };
	let hovered = $state<GlobeMarker | null>(null);
	let hoverAt = $state({ x: 0, y: 0 });
	let frame = 0;
	let tween = 0;
	const reduceMotion =
		typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches;

	// Faint stars, fixed per load.
	const stars = Array.from({ length: 90 }, () => ({
		x: Math.random(),
		y: Math.random(),
		a: 0.15 + Math.random() * 0.35
	}));

	function radius() {
		return Math.min(size.w, size.h) * 0.38 * view.zoom;
	}

	/** Orthographic projection. Returns null for points on the far side. */
	function project(lat: number, lng: number) {
		const φ = lat * RAD;
		const λ = (lng - view.lng) * RAD;
		const φ0 = view.lat * RAD;
		const cosc = Math.sin(φ0) * Math.sin(φ) + Math.cos(φ0) * Math.cos(φ) * Math.cos(λ);
		if (cosc < 0) return null;
		const r = radius();
		return {
			x: size.w / 2 + r * Math.cos(φ) * Math.sin(λ),
			y: size.h / 2 - r * (Math.cos(φ0) * Math.sin(φ) - Math.sin(φ0) * Math.cos(φ) * Math.cos(λ)),
			depth: cosc
		};
	}

	function draw() {
		frame = 0;
		const ctx = canvas?.getContext('2d');
		if (!ctx || !size.w) return;
		const { w, h, dpr } = size;
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		ctx.clearRect(0, 0, w, h);
		const pal = palette();

		for (const s of pal.stars ? stars : []) {
			ctx.fillStyle = `rgba(255,255,255,${s.a})`;
			ctx.fillRect(s.x * w, s.y * h, 1, 1);
		}

		const cx = w / 2;
		const cy = h / 2;
		const r = radius();

		// Atmosphere glow, then the sphere.
		const glow = ctx.createRadialGradient(cx, cy, r * 0.92, cx, cy, r * 1.22);
		glow.addColorStop(0, pal.glow[0]);
		glow.addColorStop(0.35, pal.glow[1]);
		glow.addColorStop(1, pal.glow[2]);
		ctx.fillStyle = glow;
		ctx.beginPath();
		ctx.arc(cx, cy, r * 1.22, 0, Math.PI * 2);
		ctx.fill();
		const sphere = ctx.createRadialGradient(cx - r * 0.3, cy - r * 0.35, r * 0.1, cx, cy, r);
		sphere.addColorStop(0, pal.sphere[0]);
		sphere.addColorStop(1, pal.sphere[1]);
		ctx.fillStyle = sphere;
		ctx.beginPath();
		ctx.arc(cx, cy, r, 0, Math.PI * 2);
		ctx.fill();
		ctx.strokeStyle = pal.rim;
		ctx.lineWidth = 1;
		ctx.stroke();

		// Land dots, dimmer toward the edge of the disc.
		const dot = Math.max(0.9, Math.min(2.2, r / 260)) * pal.dotScale;
		for (let i = 0; i < landDots.length; i += 2) {
			const p = project(landDots[i], landDots[i + 1]);
			if (!p || p.x < -4 || p.y < -4 || p.x > w + 4 || p.y > h + 4) continue;
			ctx.fillStyle = `rgba(${pal.land},${pal.landAlpha[0] + p.depth * pal.landAlpha[1]})`;
			ctx.fillRect(p.x - dot / 2, p.y - dot / 2, dot, dot);
		}

		// Markers: glow, then the dot (or a dashed ring for stand-in positions).
		const ordered = [...markers].sort(
			(a, b) => Number(a.id === selectedId) - Number(b.id === selectedId)
		);
		for (const m of ordered) {
			const p = project(m.lat, m.lng);
			if (!p) continue;
			const selected = m.id === selectedId;
			const dotR = selected ? 7 : 5;
			const halo = ctx.createRadialGradient(p.x, p.y, 0, p.x, p.y, dotR * 3.2);
			halo.addColorStop(0, `${m.color}${pal.halo}`);
			halo.addColorStop(1, `${m.color}00`);
			ctx.fillStyle = halo;
			ctx.beginPath();
			ctx.arc(p.x, p.y, dotR * 3.2, 0, Math.PI * 2);
			ctx.fill();
			ctx.beginPath();
			ctx.arc(p.x, p.y, dotR, 0, Math.PI * 2);
			if (m.hollow) {
				ctx.setLineDash([3, 3]);
				ctx.strokeStyle = m.color;
				ctx.lineWidth = 2;
				ctx.fillStyle = pal.hollowFill;
				ctx.fill();
				ctx.stroke();
				ctx.setLineDash([]);
			} else {
				ctx.fillStyle = m.color;
				ctx.fill();
				if (pal.markerEdge) {
					ctx.strokeStyle = pal.markerEdge;
					ctx.lineWidth = 1;
					ctx.stroke();
				}
			}
			if (selected) {
				ctx.beginPath();
				ctx.arc(p.x, p.y, dotR + 5, 0, Math.PI * 2);
				ctx.strokeStyle = pal.ring;
				ctx.lineWidth = 1.5;
				ctx.stroke();
			}
		}
	}

	function redraw() {
		if (!frame) frame = requestAnimationFrame(draw);
	}

	function clampView() {
		view.lat = Math.max(-85, Math.min(85, view.lat));
		view.lng = ((((view.lng + 180) % 360) + 360) % 360) - 180;
		view.zoom = Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, view.zoom));
	}

	/** Turn (and optionally zoom) to a point, animated unless the user prefers reduced motion. */
	function flyTo(lat: number, lng: number, zoom = view.zoom) {
		cancelAnimationFrame(tween);
		const from = { ...view };
		let dLng = lng - from.lng;
		dLng = ((((dLng + 180) % 360) + 360) % 360) - 180;
		if (reduceMotion) {
			view = { lat, lng, zoom };
			clampView();
			redraw();
			return;
		}
		const start = performance.now();
		const step = (now: number) => {
			const t = Math.min(1, (now - start) / 700);
			const e = 1 - Math.pow(1 - t, 3);
			view = {
				lat: from.lat + (lat - from.lat) * e,
				lng: from.lng + dLng * e,
				zoom: from.zoom + (zoom - from.zoom) * e
			};
			clampView();
			draw();
			if (t < 1) tween = requestAnimationFrame(step);
		};
		tween = requestAnimationFrame(step);
	}

	export function zoomBy(factor: number) {
		flyTo(view.lat, view.lng, Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, view.zoom * factor)));
	}

	function markerAt(x: number, y: number) {
		let best: GlobeMarker | null = null;
		let bestD = 16;
		for (const m of markers) {
			const p = project(m.lat, m.lng);
			if (!p) continue;
			const d = Math.hypot(p.x - x, p.y - y);
			if (d < bestD) {
				best = m;
				bestD = d;
			}
		}
		return best;
	}

	// Dragging turns the globe; a press that barely moves is a click.
	let drag: { x: number; y: number; moved: boolean; lat: number; lng: number } | null = null;

	function onpointerdown(e: PointerEvent) {
		cancelAnimationFrame(tween);
		canvas?.setPointerCapture(e.pointerId);
		drag = { x: e.clientX, y: e.clientY, moved: false, lat: view.lat, lng: view.lng };
	}

	function onpointermove(e: PointerEvent) {
		const rect = (e.currentTarget as HTMLCanvasElement).getBoundingClientRect();
		if (drag) {
			const dx = e.clientX - drag.x;
			const dy = e.clientY - drag.y;
			if (Math.hypot(dx, dy) > 3) drag.moved = true;
			const degPerPx = 1 / (radius() * RAD);
			view.lng = drag.lng - dx * degPerPx;
			view.lat = drag.lat + dy * degPerPx;
			clampView();
			redraw();
			hovered = null;
			return;
		}
		hovered = markerAt(e.clientX - rect.left, e.clientY - rect.top);
		hoverAt = { x: e.clientX - rect.left, y: e.clientY - rect.top };
	}

	function onpointerup(e: PointerEvent) {
		const rect = (e.currentTarget as HTMLCanvasElement).getBoundingClientRect();
		if (drag && !drag.moved) {
			const hit = markerAt(e.clientX - rect.left, e.clientY - rect.top);
			if (hit) selectedId = hit.id;
		}
		drag = null;
	}

	function onkeydown(e: KeyboardEvent) {
		const turn = 10 / view.zoom;
		const keys: Record<string, () => void> = {
			ArrowLeft: () => (view.lng -= turn),
			ArrowRight: () => (view.lng += turn),
			ArrowUp: () => (view.lat += turn),
			ArrowDown: () => (view.lat -= turn),
			'+': () => (view.zoom *= 1.4),
			'=': () => (view.zoom *= 1.4),
			'-': () => (view.zoom /= 1.4)
		};
		const action = keys[e.key];
		if (!action) return;
		e.preventDefault();
		action();
		clampView();
		redraw();
	}

	/** Centre on the selected marker (or the first), keeping the whole sphere in view. */
	function fitMarkers() {
		if (!markers.length) return;
		const target = markers.find((m) => m.id === selectedId) ?? markers[0];
		view = { lat: target.lat - 8, lng: target.lng, zoom: 1 };
		clampView();
	}

	onMount(() => {
		fitMarkers();
		const observer = new ResizeObserver(([entry]) => {
			const dpr = Math.min(window.devicePixelRatio || 1, 2);
			size = { w: entry.contentRect.width, h: entry.contentRect.height, dpr };
			if (!canvas) return;
			canvas.width = Math.round(size.w * dpr);
			canvas.height = Math.round(size.h * dpr);
			draw();
		});
		observer.observe(wrapper);
		return () => {
			observer.disconnect();
			cancelAnimationFrame(frame);
			cancelAnimationFrame(tween);
		};
	});

	// Selecting a device turns the globe to it; marker and theme changes repaint.
	let lastSelected = '';
	$effect(() => {
		const id = selectedId;
		void markers;
		void theme.current;
		void tint;
		if (id && id !== lastSelected && size.w) {
			const m = markers.find((x) => x.id === id);
			if (m) flyTo(m.lat - 8 / view.zoom, m.lng);
		}
		lastSelected = id;
		redraw();
	});
</script>

<div bind:this={wrapper} class="relative h-full w-full">
	{#if interactive}
		<canvas
			bind:this={canvas}
			class={[
				'block h-full w-full touch-none outline-none focus-visible:ring-1 focus-visible:ring-teal/60',
				hovered ? 'cursor-pointer' : 'cursor-grab active:cursor-grabbing'
			]}
			tabindex="0"
			aria-label="{ariaLabel}. Drag or use the arrow keys to turn it, plus and minus to zoom."
			{onpointerdown}
			{onpointermove}
			{onpointerup}
			onpointerleave={() => (hovered = null)}
			{onkeydown}
		></canvas>
	{:else}
		<canvas bind:this={canvas} class="block h-full w-full" aria-hidden="true"></canvas>
	{/if}
	{#if hovered}
		<p
			class="pointer-events-none absolute z-10 -translate-x-1/2 -translate-y-full rounded-sm border border-line-strong bg-canvas/90 px-2 py-1 text-xs whitespace-nowrap text-ink"
			style:left="{hoverAt.x}px"
			style:top="{hoverAt.y - 12}px"
		>
			{hovered.label}
		</p>
	{/if}
</div>
