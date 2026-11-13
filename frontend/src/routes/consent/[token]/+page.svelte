<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import { CONSENT_LABELS } from '#lib/domain/consent.ts';
	import type { ConsentRequest } from '#lib/domain/types.ts';
	import { errorMessage, services } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// svelte-ignore state_referenced_locally
	let request = $state<ConsentRequest | null>(data.request);
	let busy = $state(false);
	let error = $state('');
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

	async function decide(give: boolean) {
		busy = true;
		error = '';
		try {
			request = await services.consent.decide(data.token, give);
		} catch (e) {
			error = errorMessage(e);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>Guardian consent · Da Grouper School</title></svelte:head>

<PublicShell content={data.content} user={data.user}>
	<div class="mx-auto max-w-xl py-10">
		<p class="text-sm text-muted">Guardian consent</p>
		{#if !request}
			<h1 class="mt-1 font-display text-3xl">This link is not valid any more</h1>
			<p class="mt-3 text-muted">
				A newer link may have been sent. Ask the young contributor to send you a new one.
			</p>
		{:else}
			<h1 class="mt-1 font-display text-3xl">
				{request.contributorName} would like to send observations
			</h1>
			<p class="mt-3">
				Hello {request.guardianName},
				{request.contributorName} has joined
				{request.classroomNames.length ? request.classroomNames.join(', ') : 'a Global Classroom'} and
				is under 18, so they need your consent before they can send environmental observations.
			</p>

			<section class="card mt-6" aria-labelledby="while-heading">
				<h2 id="while-heading" class="font-semibold">While you consent</h2>
				<ul class="mt-3 list-disc space-y-1.5 pl-5 text-sm">
					<li>{request.contributorName} can send observations to their classroom.</li>
					<li>Their name is never shown with their observations.</li>
					<li>
						Positions from their phone are rounded to {request.coordinateDecimals} decimal places before
						they are stored.
					</li>
					<li>You can withdraw at any time from this page. They then cannot send observations.</li>
				</ul>
			</section>

			<div class="mt-6 flex flex-wrap items-center gap-3">
				<p class="text-sm" role="status">
					Consent:
					<span
						class={[
							'chip ml-1',
							request.status === 'given' && 'border-lime/50 text-lime',
							request.status === 'withdrawn' && 'border-danger/50 text-danger'
						]}>{CONSENT_LABELS[request.status]}</span
					>
					{#if request.decidedAt}
						<span class="ml-1 text-muted">since {dateTime.format(new Date(request.decidedAt))}</span
						>
					{/if}
				</p>
			</div>
			<div class="mt-4 flex flex-wrap gap-2">
				{#if request.status !== 'given'}
					<button type="button" class="btn btn-primary" disabled={busy} onclick={() => decide(true)}
						><Icon name="check" class="size-4" /> {busy ? 'Saving…' : 'Give consent'}</button
					>
				{/if}
				{#if request.status === 'given' || request.status === 'pending'}
					<button type="button" class="btn btn-ghost" disabled={busy} onclick={() => decide(false)}
						><Icon name="close" class="size-4" />
						{request.status === 'given' ? 'Withdraw consent' : 'Do not consent'}</button
					>
				{/if}
			</div>
			{#if error}<p class="field-error mt-3" role="alert">{error}</p>{/if}
		{/if}
	</div>
</PublicShell>
