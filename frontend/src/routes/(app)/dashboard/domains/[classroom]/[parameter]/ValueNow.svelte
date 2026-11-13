<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import RollingNumber from '#lib/components/RollingNumber.svelte';
	import type { thresholdStatus } from '#lib/domain/alerts.ts';

	/** The value now (AP-23): the number, its alert level, and where and when it was read. */
	let {
		now,
		alertNow
	}: {
		now: { value: string; caption: string; range?: string | null; review: boolean };
		alertNow: ReturnType<typeof thresholdStatus> | null;
	} = $props();

	const ALERT_BADGE = {
		normal: 'bg-lime/15 text-lime ring-lime/40',
		caution: 'bg-orange/15 text-orange ring-orange/40',
		warning: 'bg-danger/15 text-danger ring-danger/40'
	};
</script>

<section class="mt-6" aria-labelledby="now-heading">
	<h2 id="now-heading" class="sr-only">Value now</h2>
	<p class="flex flex-wrap items-center gap-x-4 gap-y-2">
		<span class="font-mono text-5xl text-ink tabular-nums"><RollingNumber value={now.value} /></span
		>
		{#if alertNow}
			<span
				class={[
					'rounded-full px-2.5 py-0.5 text-sm font-medium ring-1',
					ALERT_BADGE[alertNow.level]
				]}
			>
				{#if alertNow.level !== 'normal'}<span aria-hidden="true">▲</span>{/if}
				{alertNow.title}
			</span>
		{/if}
	</p>
	<p class="mt-1 text-sm text-muted">{now.caption}</p>
	{#if now.range}<p class="text-sm text-muted">{now.range}</p>{/if}
	{#if now.review}
		<p class="mt-2 inline-flex items-center gap-1.5 text-sm text-orange">
			<Icon name="alert" class="size-4" /> The sources disagree on this day: flagged for review.
		</p>
	{/if}
</section>
