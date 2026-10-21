<script lang="ts">
	/**
	 * A formatted value whose digits roll to their new place when it changes, like the wheels
	 * of a mechanical counter. Other characters (the unit, a sign, the decimal point) stay put.
	 * Screen readers get the plain value; reduced motion swaps the digits without rolling.
	 */
	let { value }: { value: string } = $props();

	// Keyed from the right, so the units and the last digits keep their wheels when the
	// number grows a digit, and only the new leading digit appears.
	const chars = $derived(
		[...value].map((char, i, all) => ({ char, key: all.length - i, digit: /\d/.test(char) }))
	);
</script>

<span class="tabular-nums">
	<span class="sr-only">{value}</span>
	<span class="inline-flex" aria-hidden="true">
		{#each chars as item (item.key)}
			{#if item.digit}
				<span class="wheel">
					<span class="reel" style:--digit={Number(item.char)}>
						{#each [0, 1, 2, 3, 4, 5, 6, 7, 8, 9] as n (n)}<span>{n}</span>{/each}
					</span>
				</span>
			{:else}
				<span class="whitespace-pre">{item.char}</span>
			{/if}
		{/each}
	</span>
</span>

<style>
	.wheel {
		display: inline-block;
		height: 1.15em;
		overflow: hidden;
		line-height: 1.15em;
		/* Fade the wheel's top and bottom edges, as if it curved away behind a window. */
		mask-image: linear-gradient(transparent, #000 10%, #000 90%, transparent);
	}
	.reel {
		display: flex;
		flex-direction: column;
		transform: translateY(calc(var(--digit) * -1.15em));
		/* A slight overshoot, then it settles: the click of a mechanical wheel. */
		transition: transform 700ms cubic-bezier(0.3, 1.3, 0.6, 1);
	}
	@media (prefers-reduced-motion: reduce) {
		.reel {
			transition: none;
		}
	}
</style>
