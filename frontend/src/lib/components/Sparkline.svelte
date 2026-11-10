<script lang="ts">
	/**
	 * Seven days at a glance (AP-22): one point per day, oldest first. A day with no reading breaks
	 * the line instead of being drawn through, and the latest day gets a dot. Decorative: the value
	 * is always written next to it.
	 */
	let { values, class: className = '' }: { values: (number | null)[]; class?: string } = $props();

	const W = 72;
	const H = 24;
	const PAD = 3;

	const numbers = $derived(values.filter((v): v is number => v !== null));
	const low = $derived(Math.min(...numbers));
	const high = $derived(Math.max(...numbers));
	const x = (i: number) => PAD + (i * (W - 2 * PAD)) / Math.max(1, values.length - 1);
	const y = (v: number) =>
		high === low ? H / 2 : H - PAD - ((v - low) * (H - 2 * PAD)) / (high - low);

	/** Runs of consecutive days with a reading; each becomes its own line. */
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
	const last = $derived(values.at(-1));
</script>

{#if numbers.length > 1}
	<svg
		class={['shrink-0 overflow-visible text-teal', className]}
		width={W}
		height={H}
		viewBox="0 0 {W} {H}"
		aria-hidden="true"
	>
		{#each runs as run, r (r)}
			{#if run.length === 1}
				<circle cx={x(run[0].i)} cy={y(run[0].v)} r="1.5" fill="currentColor" />
			{:else}
				<polyline
					points={run.map((p) => `${x(p.i)},${y(p.v)}`).join(' ')}
					fill="none"
					stroke="currentColor"
					stroke-width="1.5"
					stroke-linecap="round"
					stroke-linejoin="round"
				/>
			{/if}
		{/each}
		{#if last !== null && last !== undefined}
			<circle cx={x(values.length - 1)} cy={y(last)} r="2.5" fill="currentColor" />
		{/if}
	</svg>
{:else}
	<span class="inline-block shrink-0" style:width="{W}px" aria-hidden="true"></span>
{/if}
