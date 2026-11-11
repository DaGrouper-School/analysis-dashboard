<script lang="ts">
	/**
	 * Recent values at a glance, oldest first: a domain card's last 7 days (AP-22), or a tile's
	 * last 24 hours, filled and as wide as its box (`fill`, AP-43). A gap in the values breaks the
	 * line instead of being drawn through, and the latest gets a dot. Decorative: the value is
	 * always written next to it.
	 */
	let {
		values,
		fill = false,
		class: className = ''
	}: { values: (number | null)[]; fill?: boolean; class?: string } = $props();

	const W = $derived(fill ? 140 : 72);
	const H = $derived(fill ? 30 : 24);
	const PAD = 3;

	const numbers = $derived(values.filter((v): v is number => v !== null));
	const low = $derived(Math.min(...numbers));
	const high = $derived(Math.max(...numbers));
	const x = (i: number) =>
		(fill ? 0 : PAD) + (i * (W - (fill ? 0 : 2 * PAD))) / Math.max(1, values.length - 1);
	const y = (v: number) =>
		high === low ? H / 2 : H - PAD - ((v - low) * (H - 2 * PAD)) / (high - low);

	/** Runs of consecutive values; each becomes its own line. */
	const runs = $derived.by(() => {
		const result: { i: number; v: number }[][] = [];
		let run: { i: number; v: number }[] = [];
		values.forEach((v, i) => {
			if (v === null) {
				if (run.length) result.push(run);
				run = [];
			} else run.push({ i, v });
		});
		if (run.length) result.push(run);
		return result;
	});
	const line = (run: { i: number; v: number }[]) =>
		run.map((p, k) => `${k ? 'L' : 'M'}${x(p.i).toFixed(1)},${y(p.v).toFixed(1)}`).join('');
	const last = $derived(values.at(-1));
</script>

{#if numbers.length > 1}
	<svg
		class={['overflow-visible text-teal', fill ? 'block h-8 w-full' : 'shrink-0', className]}
		width={fill ? undefined : W}
		height={fill ? undefined : H}
		viewBox="0 0 {W} {H}"
		preserveAspectRatio={fill ? 'none' : undefined}
		aria-hidden="true"
	>
		{#each runs as run, r (r)}
			{#if run.length === 1}
				{#if !fill}<circle cx={x(run[0].i)} cy={y(run[0].v)} r="1.5" fill="currentColor" />{/if}
			{:else}
				{#if fill}
					<path
						d="{line(run)}L{x(run.at(-1)!.i)},{H}L{x(run[0].i)},{H}Z"
						fill="currentColor"
						opacity="0.14"
					/>
				{/if}
				<path
					d={line(run)}
					fill="none"
					stroke="currentColor"
					stroke-width="1.5"
					stroke-linecap="round"
					stroke-linejoin="round"
					vector-effect="non-scaling-stroke"
				/>
			{/if}
		{/each}
		{#if last !== null && last !== undefined && !fill}
			<circle cx={x(values.length - 1)} cy={y(last)} r="2.5" fill="currentColor" />
		{/if}
	</svg>
{:else if !fill}
	<span class="inline-block shrink-0" style:width="{W}px" aria-hidden="true"></span>
{/if}
