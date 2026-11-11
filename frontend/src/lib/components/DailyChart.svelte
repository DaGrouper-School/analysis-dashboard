<script lang="ts" module>
	/** One day's bar, for rain: solid from the reference gauge, outlined from another source. */
	export interface DailyBar {
		value: number | null;
		reference: boolean;
		flagged: boolean;
	}
</script>

<script lang="ts">
	/**
	 * One point a day (AP-45), as a line, a line with the day's low-to-high band, or bars. A day
	 * with no reading breaks the line and has no bar; it is never filled in. Point at a day to see
	 * its value; the figure's caption says what is drawn, so the colour never carries it alone.
	 */
	let {
		days,
		line = [],
		band = [],
		bars = [],
		label,
		tick,
		tip
	}: {
		/** `YYYY-MM-DD`, oldest first. */
		days: string[];
		line?: (number | null)[];
		band?: ({ low: number; high: number } | null)[];
		bars?: DailyBar[];
		/** What the chart shows, for screen readers. */
		label: string;
		/** An axis value as text, such as "24°". */
		tick: (value: number) => string;
		/** The day's value in words for the tooltip; null when there is none. */
		tip: (index: number) => string | null;
	} = $props();

	const W = 440;
	const H = 200;
	const PL = 34;
	const PR = 8;
	const PT = 10;
	const PB = 24;
	const PH = H - PT - PB;

	// A scale with round steps that holds every value, from zero for bars.
	const scale = $derived.by(() => {
		const values = [
			...line,
			...band.flatMap((b) => (b ? [b.low, b.high] : [])),
			...bars.map((b) => b.value)
		].filter((v): v is number => v !== null);
		let lo = bars.length ? 0 : Math.min(...values);
		let hi = Math.max(...values);
		if (!values.length) [lo, hi] = [0, 1];
		if (hi === lo) hi = lo + 1;
		const raw = (hi - lo) / 4;
		const mag = 10 ** Math.floor(Math.log10(raw));
		const step = [1, 2, 2.5, 5, 10].map((m) => m * mag).find((s) => s >= raw)!;
		const min = Math.floor(lo / step) * step;
		const max = Math.ceil(hi / step) * step;
		const ticks = Array.from(
			{ length: Math.round((max - min) / step) + 1 },
			(_, i) => min + i * step
		);
		return { min, max, ticks };
	});
	const slot = $derived((W - PL - PR) / Math.max(1, days.length));
	const x = (i: number) => PL + slot * (i + 0.5);
	const y = (v: number) => PT + PH - ((v - scale.min) / (scale.max - scale.min)) * PH;

	const path = (points: ({ i: number; v: number } | null)[]) => {
		let d = '';
		let open = false;
		for (const p of points) {
			if (!p) {
				open = false;
				continue;
			}
			d += `${open ? 'L' : 'M'}${x(p.i).toFixed(1)},${y(p.v).toFixed(1)}`;
			open = true;
		}
		return d;
	};
	const linePath = $derived(path(line.map((v, i) => (v === null ? null : { i, v }))));
	// The band as one shape per run of days with a low and high.
	const bandPaths = $derived.by(() => {
		const runs: { i: number; low: number; high: number }[][] = [];
		let run: { i: number; low: number; high: number }[] = [];
		band.forEach((b, i) => {
			if (!b) {
				if (run.length) runs.push(run);
				run = [];
			} else run.push({ i, ...b });
		});
		if (run.length) runs.push(run);
		return runs.map(
			(r) =>
				'M' +
				r.map((p) => `${x(p.i).toFixed(1)},${y(p.high).toFixed(1)}`).join('L') +
				'L' +
				[...r]
					.reverse()
					.map((p) => `${x(p.i).toFixed(1)},${y(p.low).toFixed(1)}`)
					.join('L') +
				'Z'
		);
	});
	const lastPoint = $derived.by(() => {
		for (let i = line.length - 1; i >= 0; i--) if (line[i] !== null) return { i, v: line[i]! };
		return null;
	});

	const short = new Intl.DateTimeFormat(undefined, {
		day: 'numeric',
		month: 'short',
		timeZone: 'UTC'
	});
	const weekday = new Intl.DateTimeFormat(undefined, {
		weekday: 'short',
		day: 'numeric',
		timeZone: 'UTC'
	});
	const long = new Intl.DateTimeFormat(undefined, {
		weekday: 'short',
		day: 'numeric',
		month: 'short',
		timeZone: 'UTC'
	});
	const date = (day: string) => new Date(`${day}T00:00:00Z`);
	// A week names each day; a month marks every seventh.
	const axis = $derived(
		days.length <= 7
			? days.map((d, i) => ({ i, text: weekday.format(date(d)) }))
			: days.flatMap((d, i) => (i % 7 === 0 ? [{ i, text: short.format(date(d)) }] : []))
	);

	let hover = $state<number | null>(null);
	const hoverTip = $derived(hover === null ? null : tip(hover));
	const hoverY = $derived.by(() => {
		if (hover === null) return null;
		const v = line[hover] ?? bars[hover]?.value ?? null;
		return v === null ? null : y(v);
	});
	const barWidth = $derived(Math.min(slot * 0.62, 26));
</script>

<div class="relative" role="img" aria-label={label}>
	<svg viewBox="0 0 {W} {H}" class="block w-full overflow-visible" aria-hidden="true">
		{#each scale.ticks as t (t)}
			<line
				x1={PL}
				x2={W - PR}
				y1={y(t)}
				y2={y(t)}
				class={t === scale.min ? 'stroke-line-strong' : 'stroke-line'}
				stroke-width="1"
			/>
			<text x={PL - 6} y={y(t) + 3.5} text-anchor="end" class="fill-muted text-[10px]"
				>{tick(t)}</text
			>
		{/each}
		{#each axis as a (a.i)}
			<text x={x(a.i)} y={H - 6} text-anchor="middle" class="fill-muted text-[10px]">{a.text}</text>
		{/each}

		{#each bandPaths as d, k (k)}
			<path {d} class="fill-series-1" opacity="0.18" />
		{/each}
		{#if linePath}
			<path
				d={linePath}
				fill="none"
				class="stroke-series-1"
				stroke-width="2"
				stroke-linejoin="round"
				stroke-linecap="round"
			/>
			{#if days.length <= 7}
				{#each line as v, i (i)}
					{#if v !== null}<circle cx={x(i)} cy={y(v)} r="3" class="fill-series-1" />{/if}
				{/each}
			{/if}
			{#if lastPoint}
				<circle
					cx={x(lastPoint.i)}
					cy={y(lastPoint.v)}
					r="4.5"
					class="fill-series-1 stroke-surface"
					stroke-width="2"
				/>
			{/if}
		{/if}

		{#each bars as bar, i (i)}
			{#if bar.value !== null}
				{@const bx = x(i) - barWidth / 2}
				{#if bar.value <= 0}
					<rect x={bx} y={y(0) - 2} width={barWidth} height="2" rx="1" class="fill-muted" />
				{:else}
					<rect
						x={bx}
						y={y(bar.value)}
						width={barWidth}
						height={Math.max(2, y(0) - y(bar.value))}
						rx="3"
						class={bar.reference
							? 'fill-series-1'
							: 'fill-transparent stroke-series-1 [stroke-dasharray:3_2]'}
						stroke-width={bar.reference ? 0 : 1.5}
					/>
				{/if}
				{#if bar.flagged}
					{@const ty = y(bar.value) - 8}
					<path d="M{x(i)},{ty - 6}L{x(i) + 5},{ty + 3}H{x(i) - 5}Z" class="fill-orange" />
				{/if}
			{/if}
		{/each}

		{#if hover !== null}
			<line x1={x(hover)} x2={x(hover)} y1={PT} y2={PT + PH} class="stroke-line-strong" />
			{#if hoverY !== null}
				<circle cx={x(hover)} cy={hoverY} r="4" class="fill-ink" />
			{/if}
		{/if}
		<!-- Hit areas wider than the marks: a whole day's column. -->
		{#each days as day, i (day)}
			<rect
				role="presentation"
				x={PL + i * slot}
				y={PT}
				width={slot}
				height={PH}
				fill="transparent"
				onpointerenter={() => (hover = i)}
				onpointerleave={() => (hover = null)}
			/>
		{/each}
	</svg>
	{#if hover !== null}
		<div
			class="pointer-events-none absolute top-0 z-10 -translate-x-1/2 rounded-md border border-line-strong bg-surface px-2.5 py-1.5 text-xs whitespace-nowrap shadow-lg"
			style:left="{Math.min(85, Math.max(15, (x(hover) / W) * 100))}%"
		>
			<p class="text-muted">{long.format(date(days[hover]))}</p>
			<p class="font-mono text-ink">{hoverTip ?? 'No reading'}</p>
		</div>
	{/if}
</div>
