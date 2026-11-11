<script lang="ts">
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import Modal from '#lib/components/Modal.svelte';
	import { CONSENT_LABELS, localDay, validatePaperConsent } from '#lib/domain/consent.ts';
	import type { ClassroomDetail, MemberConsent } from '#lib/domain/types.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';

	/**
	 * Minors' guardian consent (AP-38): where each one stands, the printable sheet, and recording
	 * a signed paper sheet or a withdrawal. Only the institution head and facilitators see it.
	 */
	let {
		classroom,
		consent,
		icon,
		onrefresh
	}: {
		classroom: ClassroomDetail['classroom'];
		consent: MemberConsent[];
		icon: string;
		onrefresh: () => Promise<void>;
	} = $props();

	const dateOnly = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeZone: 'UTC' });
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const formatDay = (day: string) => dateOnly.format(new Date(`${day}T00:00:00Z`));
	let recordOpen = $state(false);
	let recordFor = $state<MemberConsent | null>(null);
	let recordGive = $state(true);
	let recordGuardian = $state('');
	let recordDay = $state('');
	let recordErrors = $state<Record<string, string>>({});
	let recordError = $state('');
	let recording = $state(false);

	function openRecord(member: MemberConsent, give: boolean) {
		recordFor = member;
		recordGive = give;
		recordGuardian = member.guardianName ?? '';
		recordDay = localDay();
		recordErrors = {};
		recordError = '';
		recordOpen = true;
	}

	async function saveRecord(event: SubmitEvent) {
		event.preventDefault();
		if (!recordFor) return;
		const input = { give: recordGive, guardianName: recordGuardian, signedOn: recordDay };
		recordErrors = validatePaperConsent(input, localDay());
		recordError = '';
		if (Object.keys(recordErrors).length) return;
		recording = true;
		try {
			await services.classrooms.recordPaperConsent(classroom.id, recordFor.userId, input);
			recordOpen = false;
			await onrefresh();
		} catch (e) {
			if (e instanceof ServiceError) recordErrors = e.fields;
			recordError = errorMessage(e);
		} finally {
			recording = false;
		}
	}

	/** The printable sheet with this minor's name on it. The path is resolved; the query added. */
	function sheetFor(userId: string) {
		return (
			resolve('/(app)/classrooms/[id]/consent-sheet', { id: classroom.id }) +
			`?member=${encodeURIComponent(userId)}`
		);
	}

	function consentSource(member: MemberConsent): string {
		if (member.method === 'paper' && member.signedOn) {
			return `Paper sheet signed ${formatDay(member.signedOn)} by ${member.guardianName ?? 'the guardian'}${
				member.recordedByName ? ` · recorded by ${member.recordedByName}` : ''
			}`;
		}
		if (member.method === 'link') {
			return member.decidedAt
				? `Answered by link ${dateTime.format(new Date(member.decidedAt))}`
				: `Link sent to ${member.guardianName ?? 'the guardian'}, not answered yet`;
		}
		return 'No sheet recorded and no link sent';
	}
</script>

<section class="card">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<h2 class="flex items-center gap-2 text-lg font-semibold">
			<Icon name={icon} class="size-5 text-muted" /> Guardian consent
			<span class="text-muted">{consent.length}</span>
		</h2>
		<a
			href={resolve('/(app)/classrooms/[id]/consent-sheet', { id: classroom.id })}
			class="btn btn-ghost"><Icon name="printer" class="size-4" /> Print consent sheet</a
		>
	</div>
	<p class="mt-1 max-w-2xl text-sm text-muted">
		Members under 18 send observations only while a guardian's consent is recorded. Hand the
		guardian a printed sheet, then record it here when it comes back signed. Only the institution
		head and facilitators see this.
	</p>
	{#if consent.length}
		<ul class="mt-4 divide-y divide-line">
			{#each consent as member (member.userId)}
				<li class="flex flex-wrap items-center gap-3 py-3">
					<div class="min-w-0 flex-1">
						<p class="flex flex-wrap items-center gap-2 font-medium">
							{member.fullName}
							<span
								class={[
									'chip text-xs',
									member.status === 'given' && 'border-lime/50 text-lime',
									member.status === 'pending' && 'border-orange/50 text-orange',
									member.status === 'withdrawn' && 'border-danger/50 text-danger'
								]}>{CONSENT_LABELS[member.status]}</span
							>
						</p>
						<p class="text-sm text-muted">{consentSource(member)}</p>
					</div>
					<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
					<a href={sheetFor(member.userId)} class="btn btn-ghost px-3 py-1.5 text-xs">Print sheet</a
					>
					{#if member.status !== 'given'}
						<button
							class="btn btn-primary px-3 py-1.5 text-xs"
							onclick={() => openRecord(member, true)}>Paper consent received</button
						>
					{/if}
					{#if member.status !== 'withdrawn'}
						<button
							class="btn btn-ghost px-3 py-1.5 text-xs"
							onclick={() => openRecord(member, false)}>Record withdrawal</button
						>
					{/if}
				</li>
			{/each}
		</ul>
	{:else}
		<p class="mt-4 text-sm text-muted">No members under 18 in this classroom.</p>
	{/if}
</section>

{#if recordFor}
	<Modal
		bind:open={recordOpen}
		eyebrow={recordFor.fullName}
		title={recordGive ? 'Paper consent received' : 'Record a withdrawal'}
	>
		<form id="record-form" class="space-y-4" onsubmit={saveRecord} novalidate>
			<p class="text-sm text-muted">
				{recordGive
					? `${recordFor.fullName} can send observations as soon as you save this.`
					: `${recordFor.fullName} can no longer send observations once you save this.`}
				The decision is recorded with your name and today's date.
			</p>
			<div>
				<label class="label" for="record-guardian">Guardian's name on the sheet</label>
				<input
					id="record-guardian"
					class={['input', recordErrors.guardianName && 'input-error']}
					bind:value={recordGuardian}
				/>
				{#if recordErrors.guardianName}<p class="field-error">{recordErrors.guardianName}</p>{/if}
			</div>
			<div>
				<label class="label" for="record-day">Date on the sheet</label>
				<input
					id="record-day"
					type="date"
					max={localDay()}
					class={['input', recordErrors.signedOn && 'input-error']}
					bind:value={recordDay}
				/>
				{#if recordErrors.signedOn}<p class="field-error">{recordErrors.signedOn}</p>{/if}
			</div>
			{#if recordError && !Object.keys(recordErrors).length}
				<p class="field-error" role="alert">{recordError}</p>
			{/if}
		</form>
		{#snippet footer()}
			<button type="button" class="btn btn-ghost" onclick={() => (recordOpen = false)}
				>Cancel</button
			>
			<button form="record-form" class="btn btn-primary" disabled={recording}>
				{recording ? 'Saving…' : recordGive ? 'Record consent' : 'Record withdrawal'}
			</button>
		{/snippet}
	</Modal>
{/if}
