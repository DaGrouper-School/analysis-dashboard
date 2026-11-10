<script lang="ts" module>
	export interface ChartBars {
		id: string;
		label: string;
		/** A CSS color, e.g. `var(--color-series-1)`. */
		color: string;
		/** The day's value, by day; a missing day stays empty, never zero. */
		values: Record<string, number>;
	}

	export interface ChartLine {
		id: string;
		label: string;
		color: string;
		points: { at: number; value: number }[];
		/** Each day's low to high, drawn as a soft band behind the line. */
		band?: { at: number; low: number; high: number }[];
		dashed?: boolean;
	}
</script>

<script lang="ts">
	/**
	 * The main chart of a value's page (AP-23), in the form the registry gives it: daily bars,
	 * one per source side by side, for totals such as rain; or lines with an optional low-to-high
	 * band for values such as temperature. Days flagged for review get a marker over their bars.
	 * Hover, tap or use the arrow keys to read a day or a reading.
	 */
	let {
		label,
		unit = '',
		height = 240,
		days = [],
		bars = [],
		flagged = [],
		lines = [],
		from = 0,
		to = 0,
		hourly = false
	}: {
		label: string;
		unit?: string;
		height?: number;
		/** Bars: the days shown, oldest first. */
		days?: string[];
		bars?: ChartBars[];
		/** Bars: days flagged for review. */
		flagged?: string[];
		lines?: ChartLine[];
		/** Lines: the time span shown, in ms. */
		from?: number;
		to?: number;
		/** Lines: label the axis with hours rather than dates. */
		hourly?: boolean;
	} = $props();

	const PAD = { top: 20, right: 16, bottom: 28, left: 44 };
	let width = $state(0);
	let active = $state<number | null>(null);

	const isBars = $derived(bars.length > 0);
	const plotWidth = $derived(Math.max(width - PAD.left - PAD.right, 1));
	const plotHeight = $derived(height - PAD.top - PAD.bottom);

	const values = $derived(
		isBars
			? bars.flatMap((b) => Object.values(b.values))
			: lines.flatMap((l) => [
					...l.points.map((p) => p.value),
					...(l.band ?? []).flatMap((b) => [b.low, b.high])
				])
	);
	const empty = $derived(values.length === 0);

	function niceStep(span: number, count: number) {
		const raw = span / count || 1;
		const power = 10 ** Math.floor(Math.log10(raw));
		return ([1, 2, 5, 10].find((m) => m * power >= raw) ?? 10) * power;
	}

	const yScale = $derived.by(() => {
		const min = Math.min(...values, ...(isBars ? [0] : []));
		const max = Math.max(...values, isBars ? 1 : -Infinity);
		const step = niceStep(max - min || Math.abs(max) || 1, 4);
		const low = Math.floor(min / step) * step;
		const high = Math.max(Math.ceil(max / step) * step, low + step);
		const ticks: number[] = [];
		for (let t = low; t <= high + step / 2; t += step) ticks.push(Math.round(t * 1000) / 1000);
		return {
			ticks,
			y: (value: number) => PAD.top + plotHeight - ((value - low) / (high - low)) * plotHeight
		};
	});

	// Bars: one slot per day, the sources side by side with a 2px gap.
	const slot = $derived(plotWidth / Math.max(days.length, 1));
	const barWidth = $derived(
		Math.max(Math.min((slot * 0.7 - 2 * (bars.length - 1)) / Math.max(bars.length, 1), 28), 2)
	);
	const groupWidth = $derived(bars.length * barWidth + 2 * (bars.length - 1));
	const dayX = (i: number) => PAD.left + slot * i + slot / 2;
	const barX = (i: number, b: number) => dayX(i) - groupWidth / 2 + b * (barWidth + 2);
	/** A bar with a rounded top, anchored to the baseline. */
	function barPath(x: number, value: number) {
		const base = yScale.y(Math.max(0, yScale.ticks[0]));
		const top = yScale.y(value);
		const r = Math.min(4, barWidth / 2, Math.abs(base - top));
		return `M${x},${base}V${top + r}Q${x},${top} ${x + r},${top}H${x + barWidth - r}Q${x + barWidth},${top} ${x + barWidth},${top + r}V${base}Z`;
	}

	// Lines: time on x.
	const x = (at: number) =>
		PAD.left + (to === from ? plotWidth / 2 : ((at - from) / (to - from)) * plotWidth);
	const path = (points: { at: number; value: number }[]) =>
		points.map((p, i) => `${i ? 'L' : 'M'}${x(p.at)},${yScale.y(p.value)}`).join('');
	const bandPath = (band: NonNullable<ChartLine['band']>) =>
		band.length
			? band.map((b, i) => `${i ? 'L' : 'M'}${x(b.at)},${yScale.y(b.high)}`).join('') +
				band
					.toReversed()
					.map((b) => `L${x(b.at)},${yScale.y(b.low)}`)
					.join('') +
				'Z'
			: '';
	/** Every reading time across the lines, for the crosshair to snap to. */
	const times = $derived(
		[...new Set(lines.flatMap((l) => l.points.map((p) => p.at)))].sort((a, b) => a - b)
	);
	const stops = $derived(isBars ? days.length : times.length);

	const dayFormat = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' });
	const hourFormat = new Intl.DateTimeFormat(undefined, { hour: 'numeric' });
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const dayLong = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeZone: 'UTC' });
	const atDay = (day: string) => Date.parse(`${day}T12:00:00.000Z`);

	/** Axis labels at least 72px apart, from the newest back. */
	const xTicks = $derived.by(() => {
		const positions = isBars
			? days.map((d, i) => ({ x: dayX(i), text: dayFormat.format(atDay(d)) }))
			: tickTimes().map((at) => ({
					x: x(at),
					text: (hourly ? hourFormat : dayFormat).format(at)
				}));
		const ticks: typeof positions = [];
		for (const tick of positions.toReversed()) {
			if (tick.x < PAD.left + 10) break;
			if (!ticks.length || ticks.at(-1)!.x - tick.x >= 72) ticks.push(tick);
		}
		return ticks;
	});
	function tickTimes() {
		const step = hourly ? 3_600_000 : 86_400_000;
		const result: number[] = [];
		for (let t = Math.ceil(from / step) * step; t <= to; t += step) result.push(t);
		return result;
	}

	function format(value: number) {
		const rounded = Math.abs(value) >= 100 ? Math.round(value) : Math.round(value * 10) / 10;
		return `${rounded}${unit && !unit.startsWith('°') ? ' ' : ''}${unit}`;
	}

	function pick(event: PointerEvent) {
		const box = (event.currentTarget as SVGElement).getBoundingClientRect();
		const px = event.clientX - box.left;
		if (isBars) {
			active = Math.min(Math.max(Math.floor((px - PAD.left) / slot), 0), days.length - 1);
			return;
		}
		let best = 0;
		times.forEach((t, i) => {
			if (Math.abs(x(t) - px) < Math.abs(x(times[best]) - px)) best = i;
		});
		active = times.length ? best : null;
	}

	function keys(event: KeyboardEvent) {
		if (!stops) return;
		if (event.key === 'ArrowLeft') active = Math.max(0, (active ?? stops) - 1);
		else if (event.key === 'ArrowRight') active = Math.min(stops - 1, (active ?? -1) + 1);
		else if (event.key === 'Escape') active = null;
		else return;
		event.preventDefault();
	}

	/** What the tooltip lists for the active day or reading. */
	const tip = $derived.by(() => {
		if (active === null || active >= stops) return null;
		if (isBars) {
			const day = days[active];
			return {
				x: dayX(active),
				title: dayLong.format(atDay(day)),
				rows: bars.map((b) => ({
					color: b.color,
					label: b.label,
					value: b.values[day] === undefined ? 'No reading' : format(b.values[day])
				})),
				note: flagged.includes(day) ? 'Sources disagree: flagged for review' : null
			};
		}
		const at = times[active];
		return {
			x: x(at),
			title: dateTime.format(at),
			rows: lines.flatMap((l) => {
				const point = l.points.find((p) => p.at === at);
				return point ? [{ color: l.color, label: l.label, value: format(point.value) }] : [];
			}),
			note: null
		};
	});
	const tipLeft = $derived(tip ? Math.min(Math.max(tip.x, 90), Math.max(width - 90, 90)) : 0);
	const summary = $derived(
		isBars
			? `${label}, daily, ${days.length} days, ${bars.map((b) => b.label).join(' and ')}. Use the arrow keys to read each day.`
			: `${label} over time, ${lines.map((l) => l.label).join(' and ')}. Use the arrow keys to read each value.`
	);
</script>

<div class="relative" style:min-height="{height}px" bind:clientWidth={width}>
	{#if empty}
		<p
			class="flex items-center justify-center text-center text-sm text-muted"
			style:height="{height}px"
		>
			No released readings in this range.
		</p>
	{:else if width}
		<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
		<svg
			{width}
			{height}
			role="img"
			tabindex="0"
			aria-label={summary}
			class="block touch-pan-y outline-none focus-visible:ring-2 focus-visible:ring-teal"
			onpointermove={pick}
			onpointerdown={pick}
			onpointerleave={() => (active = null)}
			onkeydown={keys}
			onblur={() => (active = null)}
		>
			{#each yScale.ticks as tick (tick)}
				<line
					x1={PAD.left}
					x2={width - PAD.right}
					y1={yScale.y(tick)}
					y2={yScale.y(tick)}
					class="stroke-line"
					stroke-width="1"
				/>
				<text
					x={PAD.left - 8}
					y={yScale.y(tick)}
					dy="0.32em"
					text-anchor="end"
					class="fill-muted text-[11px] tabular-nums">{format(tick).replace(` ${unit}`, '')}</text
				>
			{/each}
			{#each xTicks as tick (tick.x)}
				<text x={tick.x} y={height - 8} text-anchor="middle" class="fill-muted text-[11px]"
					>{tick.text}</text
				>
			{/each}

			{#if isBars}
				{#if tip}
					<rect
						x={tip.x - slot / 2}
						y={PAD.top}
						width={slot}
						height={plotHeight}
						class="fill-line"
					/>
				{/if}
				{#each days as day, i (day)}
					{#each bars as bar, b (bar.id)}
						{#if bar.values[day] !== undefined}
							<path d={barPath(barX(i, b), bar.values[day])} fill={bar.color} />
						{/if}
					{/each}
					{#if flagged.includes(day)}
						{@const highest = Math.max(
							...bars.map((b) => b.values[day] ?? -Infinity),
							yScale.ticks[0]
						)}
						<g transform="translate({dayX(i)},{Math.max(yScale.y(highest) - 10, 8)})">
							<title>Flagged for review</title>
							<path d="M0,-6L6,5H-6Z" class="fill-orange stroke-surface" stroke-width="1.5" />
							<text y="3.5" text-anchor="middle" class="fill-surface text-[8px] font-bold">!</text>
						</g>
					{/if}
				{/each}
			{:else}
				{#each lines as line (line.id)}
					{#if line.band?.length}
						<path d={bandPath(line.band)} fill={line.color} fill-opacity="0.16" />
					{/if}
				{/each}
				{#if tip}
					<line
						x1={tip.x}
						x2={tip.x}
						y1={PAD.top}
						y2={height - PAD.bottom}
						class="stroke-line-strong"
						stroke-width="1"
					/>
				{/if}
				{#each lines as line (line.id)}
					<path
						d={path(line.points)}
						fill="none"
						stroke={line.color}
						stroke-width="2"
						stroke-linejoin="round"
						stroke-linecap="round"
						stroke-dasharray={line.dashed ? '5 4' : undefined}
					/>
					{#if line.points.length === 1}
						<circle
							cx={x(line.points[0].at)}
							cy={yScale.y(line.points[0].value)}
							r="4"
							fill={line.color}
						/>
					{/if}
					{#if tip}
						{@const point = line.points.find((p) => p.at === times[active!])}
						{#if point}
							<circle
								cx={x(point.at)}
								cy={yScale.y(point.value)}
								r="5"
								fill={line.color}
								class="stroke-surface"
								stroke-width="2"
							/>
						{/if}
					{/if}
				{/each}
			{/if}
		</svg>
		{#if tip}
			<div
				class="pointer-events-none absolute top-0 -translate-x-1/2 rounded-md border border-line bg-surface px-3 py-2 text-xs shadow-sm"
				style:left="{tipLeft}px"
				role="status"
			>
				<p class="text-muted">{tip.title}</p>
				{#each tip.rows as row (row.label)}
					<p class="mt-0.5 flex items-center gap-2 text-ink">
						<span class="size-2 rounded-sm" style:background={row.color} aria-hidden="true"></span>
						<span class="text-text">{row.label}</span>
						<span class="ml-auto pl-3 font-medium tabular-nums">{row.value}</span>
					</p>
				{/each}
				{#if tip.note}<p class="mt-1 text-orange">{tip.note}</p>{/if}
			</div>
		{/if}
	{/if}
</div>
