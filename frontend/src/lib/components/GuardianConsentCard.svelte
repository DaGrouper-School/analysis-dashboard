<script lang="ts">
	import { resolve } from '$app/paths';
	import { invalidate } from '$app/navigation';
	import { CONSENT_LABELS, validateGuardian } from '#lib/domain/consent.ts';
	import type { GuardianConsent } from '#lib/domain/types.ts';
	import { errorMessage, isMockDataSource, services, ServiceError } from '#lib/services/index.ts';

	/**
	 * A minor's guardian consent (AP-04): where it stands, and a new link when it is needed. Only
	 * the guardian gives or withdraws it, from their link.
	 */
	let { consent }: { consent: GuardianConsent | null } = $props();

	// svelte-ignore state_referenced_locally
	let current = $state(consent);
	// svelte-ignore state_referenced_locally
	let guardianName = $state(consent?.guardianName ?? '');
	// svelte-ignore state_referenced_locally
	let guardianContact = $state(consent?.guardianContact ?? '');
	let errors = $state<Record<string, string>>({});
	let message = $state('');
	let sending = $state(false);
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

	const STATUS_TEXT = {
		not_needed: '',
		pending:
			'Your guardian has not answered yet. You can send observations once they give consent.',
		given: 'Your guardian gave consent. You can send observations.',
		withdrawn: 'Your guardian withdrew consent, so you cannot send observations.'
	};

	async function send(event: SubmitEvent) {
		event.preventDefault();
		message = '';
		errors = validateGuardian({ guardianName, guardianContact });
		if (Object.keys(errors).length) return;
		sending = true;
		try {
			current = await services.profile.requestGuardianConsent({ guardianName, guardianContact });
			message = `A new link was sent to ${current.guardianContact}. The old one no longer works.`;
			await invalidate('app:session');
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			message = errorMessage(e);
		} finally {
			sending = false;
		}
	}
</script>

<section class="card max-w-2xl" aria-labelledby="consent-heading">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<h2 id="consent-heading" class="text-lg font-semibold">Guardian consent</h2>
		{#if current}
			<span
				class={[
					'chip',
					current.status === 'given' && 'border-lime/50 text-lime',
					current.status === 'pending' && 'border-orange/50 text-orange',
					current.status === 'withdrawn' && 'border-danger/50 text-danger'
				]}>{CONSENT_LABELS[current.status]}</span
			>
		{/if}
	</div>
	{#if current}
		<p class="mt-2 text-sm">{STATUS_TEXT[current.status]}</p>
		<p class="mt-1 text-sm text-muted">
			Asked {current.guardianName} ({current.guardianContact}) on
			{dateTime.format(new Date(current.requestedAt))}{current.decidedAt
				? `; answered ${dateTime.format(new Date(current.decidedAt))}`
				: ''}.
		</p>
		{#if isMockDataSource && current.devToken}
			<p class="mt-3 text-sm">
				<span class="text-muted">Demo:</span>
				<a class="link" href={resolve('/consent/[token]', { token: current.devToken })}
					>open the guardian's consent page</a
				>
			</p>
		{/if}
	{/if}
	<p class="mt-2 text-sm text-muted">
		Your name is never shown with your observations, and your phone's position is rounded before it
		is stored.
	</p>
	{#if !current || current.status !== 'given'}
		<form class="mt-5 grid gap-4 sm:grid-cols-2" onsubmit={send} novalidate>
			<div>
				<label class="label" for="consent-guardian-name">Guardian's name</label>
				<input
					id="consent-guardian-name"
					class={['input', errors.guardianName && 'input-error']}
					bind:value={guardianName}
				/>
				{#if errors.guardianName}<p class="field-error">{errors.guardianName}</p>{/if}
			</div>
			<div>
				<label class="label" for="consent-guardian-contact">Their email or phone</label>
				<input
					id="consent-guardian-contact"
					class={['input', errors.guardianContact && 'input-error']}
					bind:value={guardianContact}
				/>
				{#if errors.guardianContact}<p class="field-error">{errors.guardianContact}</p>{/if}
			</div>
			<div class="flex flex-wrap items-center gap-3 sm:col-span-2">
				<button class="btn btn-ghost" disabled={sending}
					>{sending ? 'Sending…' : 'Send a new link'}</button
				>
				{#if message}<p class="text-sm text-muted" role="status">{message}</p>{/if}
			</div>
		</form>
	{/if}
</section>
