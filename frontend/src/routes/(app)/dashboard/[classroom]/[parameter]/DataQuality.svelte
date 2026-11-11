<script lang="ts">
	import RollingNumber from '#lib/components/RollingNumber.svelte';
	import { dateTime } from './value-format.ts';

	/** Data quality (AP-23): how complete the last 30 days are and how far review has got. */
	let {
		quality,
		releasedThrough,
		inReview,
		rain,
		sources
	}: {
		quality: { received: number; expected: number; flagged: number };
		releasedThrough: string | null;
		inReview: boolean;
		/** Rain counts the days flagged for review; other values list their sources. */
		rain: boolean;
		sources: string;
	} = $props();
</script>

<section class="mt-8" aria-labelledby="quality-heading">
	<h2 id="quality-heading" class="font-display text-2xl">Data quality</h2>
	<dl class="mt-3 grid gap-3 sm:grid-cols-3">
		<div class="card">
			<dt class="text-sm text-muted">Days with readings, last 30</dt>
			<dd class="mt-1 font-mono text-2xl text-ink tabular-nums">
				<RollingNumber value={String(quality.received)} /><span class="text-base text-muted">
					/ {quality.expected}</span
				>
			</dd>
		</div>
		<div class="card">
			<dt class="text-sm text-muted">Reviewed and released through</dt>
			<dd class="mt-1 text-ink">
				{releasedThrough ? dateTime.format(new Date(releasedThrough)) : 'Nothing yet'}
			</dd>
			{#if inReview}<dd class="mt-1 text-sm text-muted">
					Newer readings are in review. They show here, but not on the public site yet.
				</dd>{/if}
		</div>
		{#if rain}
			<div class="card">
				<dt class="text-sm text-muted">Days flagged for review, last 30</dt>
				<dd
					class={[
						'mt-1 font-mono text-2xl tabular-nums',
						quality.flagged ? 'text-orange' : 'text-ink'
					]}
				>
					<RollingNumber value={String(quality.flagged)} />
				</dd>
			</div>
		{:else}
			<div class="card">
				<dt class="text-sm text-muted">Sources</dt>
				<dd class="mt-1 text-ink">{sources}</dd>
			</div>
		{/if}
	</dl>
</section>
