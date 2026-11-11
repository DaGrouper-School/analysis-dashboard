<script lang="ts">
	import CompletenessCard from '#lib/features/classroom/CompletenessCard.svelte';
	import type { PublicStation } from '#lib/domain/types.ts';

	/**
	 * "How complete is this month?" (AP-22) with its heading, the same on the public classroom
	 * page and the dashboard.
	 */
	let {
		completeness,
		class: className = ''
	}: { completeness: PublicStation['completeness']; class?: string } = $props();

	const monthName = new Intl.DateTimeFormat(undefined, {
		month: 'long',
		year: 'numeric',
		timeZone: 'UTC'
	});
</script>

<section class={className} aria-labelledby="complete-heading">
	<h2 id="complete-heading" class="font-display text-2xl">How complete is this month?</h2>
	{#if completeness}
		<p class="mt-1 max-w-2xl text-sm text-muted">
			Readings received in {monthName.format(new Date(`${completeness.month}-01T00:00:00Z`))}, as a
			share of what the classroom's research protocol expects. Missing days stay empty; they are
			never filled in.
		</p>
		<CompletenessCard class="mt-5" domains={completeness.domains} />
	{:else}
		<p class="mt-2 text-muted">This classroom has no research protocol yet.</p>
	{/if}
</section>
