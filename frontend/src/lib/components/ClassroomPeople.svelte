<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import CopyButton from '#lib/components/CopyButton.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Modal from '#lib/components/Modal.svelte';
	import { CONSENT_LABELS, localDay, validatePaperConsent } from '#lib/domain/consent.ts';
	import type {
		ClassroomDetail,
		ClassroomRole,
		Invitation,
		MemberConsent
	} from '#lib/domain/types.ts';
	import { CLASSROOM_ROLE_LABELS } from '#lib/labels.ts';
	import { errorMessage, isMockDataSource, services, ServiceError } from '#lib/services/index.ts';

	/**
	 * A classroom's people (AP-45): its members and their roles, pending invitations, and minors'
	 * guardian consent with the printable sheet. A dashboard tab for the institution head and
	 * facilitators; each part shows only what the viewer's permissions allow.
	 */
	let {
		detail,
		consent,
		userId
	}: { detail: ClassroomDetail; consent: MemberConsent[]; userId: string } = $props();

	const classroom = $derived(detail.classroom);
	const permissions = $derived(detail.permissions);
	const ICONS = { members: 'users', invitations: 'mail', consent: 'clipboard' };

	// Paper guardian consent (AP-38): the head or a facilitator records a signed sheet.
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
			await refresh();
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

	let emails = $state('');
	// svelte-ignore state_referenced_locally
	let inviteRole = $state<ClassroomRole>(
		detail.permissions.invite.includes('community_scientist')
			? 'community_scientist'
			: detail.permissions.invite[0]
	);
	let inviteError = $state('');
	let inviting = $state(false);
	let sent = $state<Invitation[]>([]);
	let inviteOpen = $state(false);

	function openInvite() {
		emails = '';
		inviteError = '';
		sent = [];
		inviteOpen = true;
	}
	let actionError = $state('');

	function inviteUrl(token: string) {
		return `${location.origin}${resolve('/invite/[token]', { token })}`;
	}

	async function refresh() {
		await invalidate('app:classroom');
	}

	async function invite(event: SubmitEvent) {
		event.preventDefault();
		inviteError = '';
		inviting = true;
		try {
			sent = await services.classrooms.invite(classroom.id, emails.split(/[\s,;]+/), inviteRole);
			emails = '';
			inviting = false;
			await refresh();
		} catch (e) {
			inviteError =
				e instanceof ServiceError && e.fields.emails ? e.fields.emails : errorMessage(e);
		} finally {
			inviting = false;
		}
	}

	async function run(action: () => Promise<void>) {
		actionError = '';
		try {
			await action();
			await refresh();
		} catch (e) {
			actionError = errorMessage(e);
		}
	}

	/** Sends a fresh link. The old one stops working. */
	function resend(invitation: Invitation) {
		run(async () => {
			await services.classrooms.invite(classroom.id, [invitation.email], invitation.role);
		});
	}

	function remove(userId: string, name: string) {
		if (!confirm(`Remove ${name} from ${classroom.name}?`)) return;
		run(() => services.classrooms.removeMember(classroom.id, userId));
	}
</script>

{#if actionError}<p class="field-error mb-4" role="alert">{actionError}</p>{/if}

<div class="space-y-5">
	<section class="card">
		<h2 class="flex items-center gap-2 text-lg font-semibold">
			<Icon name={ICONS.members} class="size-5 text-muted" /> Members
			<span class="text-muted">{detail.members.length}</span>
		</h2>
		<ul class="mt-4 divide-y divide-line">
			{#each detail.members as member (member.userId)}
				{@const editable =
					permissions.manageMembers &&
					member.role !== 'institution_head' &&
					member.userId !== userId}
				<li class="flex flex-wrap items-center gap-3 py-3">
					<div class="min-w-0 flex-1">
						<p class="font-medium">
							{member.fullName}
							{#if member.userId === userId}<span class="text-muted">(you)</span>{/if}
						</p>
						{#if member.email}<p class="truncate text-sm text-muted">{member.email}</p>{/if}
					</div>
					{#if editable}
						<label class="sr-only" for="role-{member.userId}">Role for {member.fullName}</label>
						<select
							id="role-{member.userId}"
							class="input w-auto py-1.5 pr-9 text-sm"
							value={member.role}
							onchange={(e) =>
								run(() =>
									services.classrooms.setMemberRole(
										classroom.id,
										member.userId,
										e.currentTarget.value as ClassroomRole
									)
								)}
						>
							<option value="facilitator">Facilitator</option>
							<option value="community_scientist">Community scientist</option>
							<option value="researcher">Researcher</option>
						</select>
						<button
							class="btn btn-ghost px-3 py-1.5 text-xs"
							onclick={() => remove(member.userId, member.fullName)}>Remove</button
						>
					{:else}
						<span class="rounded-full bg-raised px-2.5 py-1 text-xs font-medium">
							{CLASSROOM_ROLE_LABELS[member.role]}
						</span>
					{/if}
				</li>
			{:else}
				<li class="py-3 text-sm text-muted">
					No members yet. People join when they accept an invitation.
				</li>
			{/each}
		</ul>
	</section>
	{#if permissions.invite.length}
		<section class="card">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<h2 class="flex items-center gap-2 text-lg font-semibold">
					<Icon name={ICONS.invitations} class="size-5 text-muted" /> Pending invitations
					<span class="text-muted">{detail.invitations.length}</span>
				</h2>
				<button type="button" class="btn btn-primary" aria-haspopup="dialog" onclick={openInvite}
					><Icon name="plus" class="size-4" /> Invite people</button
				>
			</div>
			<p class="mt-1 text-sm text-muted">
				Each person gets a link to sign up and join. Only invited people can join.
			</p>
			{#if detail.invitations.length}
				<ul class="mt-4 divide-y divide-line">
					{#each detail.invitations as invitation (invitation.id)}
						<li class="flex flex-wrap items-center gap-2 py-3">
							<div class="min-w-0 flex-1">
								<p class="truncate text-sm font-medium">{invitation.email}</p>
								<p class="text-xs text-muted">
									{CLASSROOM_ROLE_LABELS[invitation.role]} · invited by {invitation.invitedByName}
								</p>
							</div>
							<CopyButton text={inviteUrl(invitation.token)} label="Copy link" />
							{#if permissions.invite.includes(invitation.role)}
								<button class="btn btn-ghost px-3 py-1.5 text-xs" onclick={() => resend(invitation)}
									>Resend</button
								>
								<button
									class="btn btn-ghost px-3 py-1.5 text-xs"
									onclick={() => run(() => services.classrooms.revokeInvitation(invitation.id))}
									>Revoke</button
								>
							{/if}
						</li>
					{/each}
				</ul>
			{:else}
				<p class="mt-2 text-sm text-muted">No pending invitations.</p>
			{/if}
		</section>
	{/if}
	{#if permissions.manageConsent}
		<section class="card">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<h2 class="flex items-center gap-2 text-lg font-semibold">
					<Icon name={ICONS.consent} class="size-5 text-muted" /> Guardian consent
					<span class="text-muted">{consent.length}</span>
				</h2>
				<a
					href={resolve('/(app)/classrooms/[id]/consent-sheet', { id: classroom.id })}
					class="btn btn-ghost"><Icon name="printer" class="size-4" /> Print consent sheet</a
				>
			</div>
			<p class="mt-1 max-w-2xl text-sm text-muted">
				Members under 18 send observations only while a guardian's consent is recorded. Hand the
				guardian a printed sheet, then record it here when it comes back signed. Only the
				institution head and facilitators see this.
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
							<a href={sheetFor(member.userId)} class="btn btn-ghost px-3 py-1.5 text-xs"
								>Print sheet</a
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
	{/if}
</div>

{#if permissions.invite.length}
	<Modal bind:open={inviteOpen} eyebrow={classroom.name} title="Invite people">
		{#if sent.length}
			<div class="rounded-md border border-lime-strong/50 bg-lime/25 p-4 text-sm" role="status">
				<p class="font-medium">
					Sent {sent.length}
					{sent.length === 1 ? 'invitation' : 'invitations'}.
				</p>
				{#if isMockDataSource}
					<p class="mt-1 text-muted">Demo mode sends no email. Share the links below.</p>
				{/if}
			</div>
			<ul class="mt-4 divide-y divide-line">
				{#each sent as invitation (invitation.id)}
					<li class="flex flex-wrap items-center gap-2 py-3">
						<div class="min-w-0 flex-1">
							<p class="truncate text-sm font-medium">{invitation.email}</p>
							<p class="text-xs text-muted">{CLASSROOM_ROLE_LABELS[invitation.role]}</p>
						</div>
						<CopyButton text={inviteUrl(invitation.token)} label="Copy link" />
					</li>
				{/each}
			</ul>
		{:else}
			<form id="invite-form" class="space-y-4" onsubmit={invite} novalidate>
				<div>
					<label class="label" for="emails">Email addresses</label>
					<textarea
						id="emails"
						rows="3"
						class={['input', inviteError && 'input-error']}
						placeholder="one@example.org, two@example.org"
						bind:value={emails}></textarea>
					{#if inviteError}<p class="field-error">{inviteError}</p>{/if}
				</div>
				<div>
					<label class="label" for="inviteRole">Role</label>
					<select id="inviteRole" class="input" bind:value={inviteRole}>
						{#each permissions.invite as role (role)}
							<option value={role}>{CLASSROOM_ROLE_LABELS[role]}</option>
						{/each}
					</select>
				</div>
			</form>
		{/if}
		{#snippet footer()}
			{#if sent.length}
				<button type="button" class="btn btn-ghost" onclick={openInvite}>Invite more</button>
				<button type="button" class="btn btn-primary" onclick={() => (inviteOpen = false)}
					>Done</button
				>
			{:else}
				<button type="button" class="btn btn-ghost" onclick={() => (inviteOpen = false)}
					>Cancel</button
				>
				<button form="invite-form" class="btn btn-primary" disabled={inviting || !emails.trim()}>
					{inviting ? 'Sending…' : 'Send invitations'}
				</button>
			{/if}
		{/snippet}
	</Modal>
{/if}

{#if permissions.manageConsent && recordFor}
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
