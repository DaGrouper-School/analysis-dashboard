<script lang="ts">
	import type { Reading } from '#lib/domain/types.ts';

	/**
	 * One parameter of one device over time: a 2px line with a soft wash, the latest value
	 * labelled at the end, and a crosshair that snaps to the nearest reading on hover or with
	 * the arrow keys.
	 */
	let {
		readings,
		parameterId,
		label,
		unit = '',
		height = 220,
		color
	}: {
		/** Any order; the chart sorts by time. */
		readings: Reading[];
		parameterId: string;
		label: string;
		unit?: string;
		height?: number;
		/** The device's registry color (deviceTypeColor). Falls back to the theme's data color. */
		color?: string;
	} = $props();

	const PAD = { top: 16, bottom: 28, left: 44 };

	let width = $state(0);
	let active = $state<number | null>(null);

	const points = $derived(
		readings
			.map((r) => ({ at: Date.parse(r.observedAt), value: r.values[parameterId] }))
			.filter((p): p is { at: number; value: number } => typeof p.value === 'number')
			.sort((a, b) => a.at - b.at)
	);

	// Room for the latest value's label at the end of the line, at about 7px a character.
	const padRight = $derived(Math.max(56, format(points.at(-1)?.value ?? 0).length * 7 + 16));

	/** Clean tick step (1, 2 or 5 times a power of ten) for about `count` ticks. */
	function niceStep(span: number, count: number) {
		const raw = span / count || 1;
		const power = 10 ** Math.floor(Math.log10(raw));
		return ([1, 2, 5, 10].find((m) => m * power >= raw) ?? 10) * power;
	}

	const scale = $derived.by(() => {
		const values = points.map((p) => p.value);
		const min = Math.min(...values);
		const max = Math.max(...values);
		const step = niceStep(max - min || Math.abs(max) || 1, 4);
		// Start at zero for amounts that can't go below it, otherwise hug the data.
		const low = min >= 0 && min - (max - min) < 0 ? 0 : Math.floor(min / step) * step;
		const high = Math.max(Math.ceil(max / step) * step, low + step);
		const ticks: number[] = [];
		for (let t = low; t <= high + step / 2; t += step) ticks.push(Math.round(t * 1000) / 1000);
		const first = points[0]?.at ?? 0;
		const last = points.at(-1)?.at ?? first;
		const plotWidth = Math.max(width - PAD.left - padRight, 1);
		const plotHeight = height - PAD.top - PAD.bottom;
		return {
			ticks,
			x: (at: number) =>
				PAD.left + (last === first ? plotWidth / 2 : ((at - first) / (last - first)) * plotWidth),
			y: (value: number) => PAD.top + plotHeight - ((value - low) / (high - low)) * plotHeight
		};
	});

	const line = $derived(
		points.map((p, i) => `${i ? 'L' : 'M'}${scale.x(p.at)},${scale.y(p.value)}`).join('')
	);
	const area = $derived(
		points.length
			? `${line}L${scale.x(points.at(-1)!.at)},${height - PAD.bottom}L${scale.x(points[0].at)},${height - PAD.bottom}Z`
			: ''
	);

	const day = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' });
	const hour = new Intl.DateTimeFormat(undefined, { hour: 'numeric' });
	// Within a day, label the hours; over longer spans, the dates.
	const tickFormat = $derived(
		points.length > 1 && points.at(-1)!.at - points[0].at <= 36 * 3_600_000 ? hour : day
	);
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	/** Labels from the latest reading back, at least 90px apart so they never overlap. */
	const xTicks = $derived.by(() => {
		const ticks: typeof points = [];
		for (const point of points.toReversed()) {
			const x = scale.x(point.at);
			if (x < PAD.left + 20) break;
			if (!ticks.length || scale.x(ticks.at(-1)!.at) - x >= 90) ticks.push(point);
		}
		return ticks;
	});

	function format(value: number) {
		const rounded = Math.abs(value) >= 100 ? Math.round(value) : Math.round(value * 10) / 10;
		return `${rounded}${unit && !unit.startsWith('°') ? ' ' : ''}${unit}`;
	}

	function nearest(event: PointerEvent) {
		const box = (event.currentTarget as SVGElement).getBoundingClientRect();
		const x = event.clientX - box.left;
		let best = 0;
		points.forEach((p, i) => {
			if (Math.abs(scale.x(p.at) - x) < Math.abs(scale.x(points[best].at) - x)) best = i;
		});
		active = best;
	}

	function keys(event: KeyboardEvent) {
		if (!points.length) return;
		if (event.key === 'ArrowLeft') active = Math.max(0, (active ?? points.length) - 1);
		else if (event.key === 'ArrowRight') active = Math.min(points.length - 1, (active ?? -1) + 1);
		else if (event.key === 'Escape') active = null;
		else return;
		event.preventDefault();
	}

	const last = $derived(points.at(-1) ?? { at: 0, value: 0 });
	const shown = $derived(active === null ? null : points[active]);
	const tooltipLeft = $derived(
		shown ? Math.min(Math.max(scale.x(shown.at), 70), Math.max(width - 70, 70)) : 0
	);
</script>

<!-- Holds its height while it measures its width or swaps data, so nothing below it jumps. -->
<div
	class="relative"
	style:min-height="{height}px"
	style:--series={color ?? 'var(--color-teal)'}
	bind:clientWidth={width}
>
	{#if points.length < 2}
		<p
			class="flex items-center justify-center text-center text-sm text-muted"
			style:height="{height}px"
		>
			{points.length
				? 'Only one reading so far: the chart needs at least two.'
				: `No ${label.toLowerCase()} readings yet.`}
		</p>
	{:else if width}
		<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
		<svg
			{width}
			{height}
			role="img"
			tabindex="0"
			aria-label={`${label} over time, ${points.length} readings, latest ${format(points.at(-1)!.value)}. Use the arrow keys to read each value.`}
			class="block touch-pan-y outline-none focus-visible:ring-2 focus-visible:ring-teal"
			onpointermove={nearest}
			onpointerdown={nearest}
			onpointerleave={() => (active = null)}
			onkeydown={keys}
			onblur={() => (active = null)}
		>
			{#each scale.ticks as tick (tick)}
				<line
					x1={PAD.left}
					x2={width - padRight}
					y1={scale.y(tick)}
					y2={scale.y(tick)}
					class="stroke-line"
					stroke-width="1"
				/>
				<text
					x={PAD.left - 8}
					y={scale.y(tick)}
					dy="0.32em"
					text-anchor="end"
					class="fill-muted text-[11px] tabular-nums">{format(tick).replace(` ${unit}`, '')}</text
				>
			{/each}
			{#each xTicks as point (point.at)}
				<text
					x={scale.x(point.at)}
					y={height - 8}
					text-anchor="middle"
					class="fill-muted text-[11px]">{tickFormat.format(point.at)}</text
				>
			{/each}

			<path d={area} class="fill-(--series)" fill-opacity="0.1" />
			<path
				d={line}
				fill="none"
				class="stroke-(--series)"
				stroke-width="2"
				stroke-linejoin="round"
				stroke-linecap="round"
			/>

			{#if shown}
				<line
					x1={scale.x(shown.at)}
					x2={scale.x(shown.at)}
					y1={PAD.top}
					y2={height - PAD.bottom}
					class="stroke-line-strong"
					stroke-width="1"
				/>
				<circle
					cx={scale.x(shown.at)}
					cy={scale.y(shown.value)}
					r="5"
					class="fill-(--series) stroke-surface"
					stroke-width="2"
				/>
			{/if}
			<circle
				cx={scale.x(last.at)}
				cy={scale.y(last.value)}
				r="4"
				class="fill-(--series) stroke-surface"
				stroke-width="2"
			/>
			<text
				x={scale.x(last.at) + 8}
				y={scale.y(last.value)}
				dy="0.32em"
				class="fill-ink text-xs font-medium">{format(last.value)}</text
			>
		</svg>
		{#if shown}
			<div
				class="pointer-events-none absolute top-0 -translate-x-1/2 rounded-md border border-line bg-surface px-3 py-2 text-xs shadow-sm"
				style:left="{tooltipLeft}px"
				role="status"
			>
				<p class="text-muted">{dateTime.format(shown.at)}</p>
				<p class="mt-0.5 flex items-center gap-2 font-medium text-ink">
					<span class="h-0.5 w-3 rounded-full bg-(--series)" aria-hidden="true"></span>
					{label}
					{format(shown.value)}
				</p>
			</div>
		{/if}
	{/if}
</div>
