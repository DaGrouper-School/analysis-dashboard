<script lang="ts">
	import RawDataButton from '#lib/features/measurements/RawDataButton.svelte';
	import { invalidate } from '$app/navigation';
	import Modal from '#lib/components/Modal.svelte';
	import type { ReleaseReview } from '#lib/domain/types.ts';
	import { errorMessage, services } from '#lib/services/index.ts';

	/**
	 * Reviewing what waits before the public sees it, and releasing it (AP-20): each source with
	 * how many readings wait and from when to when, a link to check its values, then one release.
	 * It releases what was reviewed, up to the newest reading listed, never what arrives later.
	 * For the institution head, and facilitators when the head allows it.
	 */
	let {
		open = $bindable(false),
		classroomId,
		review
	}: { open?: boolean; classroomId: string; review: ReleaseReview } = $props();

	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const through = $derived(
		review.sources
			.map((s) => s.last)
			.sort()
			.at(-1) ?? null
	);
	let releasing = $state(false);
	let error = $state('');

	async function release() {
		if (!through) return;
		releasing = true;
		error = '';
		try {
			await services.classrooms.releaseData(classroomId, through);
			await invalidate('app:classroom');
			open = false;
		} catch (e) {
			error = errorMessage(e);
		} finally {
			releasing = false;
		}
	}
</script>

<Modal bind:open title="Review and release" eyebrow="Public data">
	<p class="text-sm">
		The public classroom page shows readings only once they are released. Check the values, then
		release them.
		{review.releasedThrough
			? `Released so far up to ${dateTime.format(new Date(review.releasedThrough))}.`
			: 'Nothing is released yet.'}
	</p>
	<ul class="mt-4 space-y-2">
		{#each review.sources as source (source.id)}
			<li class="flex flex-wrap items-center gap-3 rounded-md border border-line bg-raised p-3">
				<div class="min-w-0 flex-1">
					<p class="font-medium text-ink">{source.name ?? 'Field observations'}</p>
					<p class="text-xs text-muted">
						{source.count}
						{source.count === 1 ? 'reading' : 'readings'} · {dateTime.format(
							new Date(source.first)
						)}
						{#if source.last !== source.first}to {dateTime.format(new Date(source.last))}{/if}
					</p>
				</div>
				{#if source.kind === 'device'}
					<RawDataButton label="Check values" query={{ classroomId, sourceId: source.id }} />
				{/if}
			</li>
		{/each}
	</ul>
	{#if error}<p class="field-error mt-3" role="alert">{error}</p>{/if}
	{#snippet footer()}
		<button type="button" class="btn btn-ghost" onclick={() => (open = false)}>Not now</button>
		<button
			type="button"
			class="btn btn-primary"
			disabled={releasing || !through}
			onclick={release}
		>
			{releasing
				? 'Releasing…'
				: through
					? `Release up to ${dateTime.format(new Date(through))}`
					: 'Nothing to release'}
		</button>
	{/snippet}
</Modal>
