<script lang="ts">
	/**
	 * A small filled line of recent values, oldest first, such as a tile's last 24 hours. An hour
	 * without a reading breaks the line. Decorative: the value is always written beside it.
	 */
	let { values, class: className = '' }: { values: (number | null)[]; class?: string } = $props();

	const W = 140;
	const H = 30;
	const PAD = 2;

	const numbers = $derived(values.filter((v): v is number => v !== null));
	const low = $derived(Math.min(...numbers));
	const high = $derived(Math.max(...numbers));
	const x = (i: number) => (i * W) / Math.max(1, values.length - 1);
	const y = (v: number) =>
		high === low ? H / 2 : H - PAD - ((v - low) * (H - 2 * PAD)) / (high - low);

	/** Runs of consecutive points with a value; each is drawn on its own. */
	const runs = $derived.by(() => {
		const result: { i: number; v: number }[][] = [];
		let run: { i: number; v: number }[] = [];
		values.forEach((v, i) => {
			if (v === null) {
				if (run.length > 1) result.push(run);
				run = [];
			} else run.push({ i, v });
		});
		if (run.length > 1) result.push(run);
		return result;
	});
	const line = (run: { i: number; v: number }[]) =>
		run.map((p, k) => `${k ? 'L' : 'M'}${x(p.i).toFixed(1)},${y(p.v).toFixed(1)}`).join('');
</script>

{#if runs.length}
	<svg
		class={['block h-8 w-full overflow-visible text-teal', className]}
		viewBox="0 0 {W} {H}"
		preserveAspectRatio="none"
		aria-hidden="true"
	>
		{#each runs as run, r (r)}
			<path
				d="{line(run)}L{x(run.at(-1)!.i)},{H}L{x(run[0].i)},{H}Z"
				fill="currentColor"
				opacity="0.14"
			/>
			<path
				d={line(run)}
				fill="none"
				stroke="currentColor"
				stroke-width="1.5"
				vector-effect="non-scaling-stroke"
				stroke-linejoin="round"
			/>
		{/each}
	</svg>
{/if}
