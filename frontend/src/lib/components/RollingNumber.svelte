<script lang="ts">
	/**
	 * A formatted value whose digits roll to their new place when it changes, like the wheels
	 * of a mechanical counter. Other characters (the unit, a sign, the decimal point) stay put.
	 * It rolls up from zero when it first shows and whenever it changes, so loaded and refreshed
	 * numbers always roll in.
	 * Screen readers get the plain value; reduced motion swaps the digits without rolling.
	 */
	let { value }: { value: string } = $props();

	// Every wheel starts at 0 and turns to its digit, on first showing and each time the value
	// changes: the reset is drawn without a transition, then the wheels roll on the frame after.
	let shown = $state(false);
	$effect(() => {
		void value;
		shown = false;
		let next = 0;
		const frame = requestAnimationFrame(() => {
			next = requestAnimationFrame(() => (shown = true));
		});
		return () => {
			cancelAnimationFrame(frame);
			cancelAnimationFrame(next);
		};
	});

	// Keyed from the right, so the units and the last digits keep their wheels when the
	// number grows a digit, and only the new leading digit appears. Grouped by word, so a long
	// value wraps between the number and its unit rather than running out of its box.
	const words = $derived.by(() => {
		let key = value.length;
		return value.split(' ').map((word) => ({
			key: key + 1000,
			chars: [...word].map((char) => ({ char, key: key--, digit: /\d/.test(char) }))
		}));
	});
</script>

<span class="tabular-nums">
	<span class="sr-only">{value}</span>
	<span aria-hidden="true"
		>{#each words as word, w (word.key)}{#if w}<span class="whitespace-pre"> </span><wbr
				/>{/if}<span class="inline-flex"
				>{#each word.chars as item (item.key)}
					{#if item.digit}
						<span class="wheel">
							<span class="reel" class:still={!shown} style:--digit={shown ? Number(item.char) : 0}>
								{#each [0, 1, 2, 3, 4, 5, 6, 7, 8, 9] as n (n)}<span>{n}</span>{/each}
							</span>
						</span>
					{:else}
						<span class="whitespace-pre">{item.char}</span>
					{/if}
				{/each}</span
			>{/each}</span
	>
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
	/* Back to 0 at once, ready to roll. */
	.still {
		transition: none;
	}
	@media (prefers-reduced-motion: reduce) {
		.reel {
			transition: none;
		}
	}
</style>
