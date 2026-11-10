<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import CopyButton from '#lib/components/CopyButton.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Modal from '#lib/components/Modal.svelte';
	import VerticalTabs, { type TabItem } from '#lib/components/VerticalTabs.svelte';
	import { CONSENT_LABELS, localDay, validatePaperConsent } from '#lib/domain/consent.ts';
	import type { ClassroomRole, Invitation, MemberConsent } from '#lib/domain/types.ts';
	import { validateClassroomUpdate } from '#lib/domain/validation.ts';
	import { CLASSROOM_ROLE_LABELS } from '#lib/labels.ts';
	import { errorMessage, isMockDataSource, services, ServiceError } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const classroom = $derived(data.detail.classroom);
	const permissions = $derived(data.detail.permissions);

	/** Each tab's icon, also drawn by its section titles. */
	const ICONS = { labs: 'flask', members: 'users', invitations: 'mail', consent: 'clipboard' };
	type Tab = keyof typeof ICONS;
	let tab = $state<Tab>();
	const tabs = $derived<TabItem<Tab>[]>([
		{ id: 'labs', label: 'Labs', icon: ICONS.labs, count: data.labs.length },
		{ id: 'members', label: 'Members', icon: ICONS.members, count: data.detail.members.length },
		...(permissions.invite.length
			? [
					{
						id: 'invitations' as const,
						label: 'Invitations',
						icon: ICONS.invitations,
						count: data.detail.invitations.length
					}
				]
			: []),
		...(permissions.manageConsent
			? [
					{
						id: 'consent' as const,
						label: 'Consent',
						icon: ICONS.consent,
						count: data.consent.length
					}
				]
			: [])
	]);

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
		data.detail.permissions.invite.includes('community_scientist')
			? 'community_scientist'
			: data.detail.permissions.invite[0]
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

	let editing = $state(false);
	let editName = $state('');
	let editInstitution = $state('');
	let editErrors = $state<Record<string, string>>({});
	let editError = $state('');
	let savingEdit = $state(false);

	function startEdit() {
		editName = classroom.name;
		editInstitution = classroom.institutionName;
		editErrors = {};
		editError = '';
		editing = true;
	}

	async function saveEdit(event: SubmitEvent) {
		event.preventDefault();
		const input = { name: editName, institutionName: editInstitution };
		editErrors = validateClassroomUpdate(input);
		editError = '';
		if (Object.keys(editErrors).length) return;
		savingEdit = true;
		try {
			await services.classrooms.update(classroom.id, input);
			editing = savingEdit = false;
			await refresh();
		} catch (e) {
			if (e instanceof ServiceError) editErrors = e.fields;
			editError = errorMessage(e);
		} finally {
			savingEdit = false;
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

<svelte:head><title>{classroom.name} · DaGrouper Analysis</title></svelte:head>

<a href={resolve('/(app)/classrooms')} class="text-sm text-muted hover:text-ink">← Classrooms</a>

{#if editing}
	<form class="card mt-4 max-w-2xl space-y-4" onsubmit={saveEdit} novalidate>
		<h1 class="text-lg font-semibold">Edit classroom</h1>
		<div>
			<label class="label" for="edit-name">Classroom name</label>
			<input
				id="edit-name"
				class={['input', editErrors.name && 'input-error']}
				bind:value={editName}
			/>
			{#if editErrors.name}<p class="field-error">{editErrors.name}</p>{/if}
		</div>
		<div>
			<label class="label" for="edit-institution">Institution</label>
			<input
				id="edit-institution"
				class={['input', editErrors.institutionName && 'input-error']}
				bind:value={editInstitution}
			/>
			{#if editErrors.institutionName}
				<p class="field-error">{editErrors.institutionName}</p>
			{/if}
		</div>
		{#if permissions.viewCode && classroom.code}
			<div>
				<label class="label" for="edit-code">Classroom code</label>
				<input id="edit-code" class="input font-mono" value={classroom.code} readonly disabled />
				<p class="mt-1 text-xs text-muted">
					The code is the classroom's permanent identifier in SAM and can't be changed.
				</p>
			</div>
		{/if}
		{#if editError}<p class="field-error" role="alert">{editError}</p>{/if}
		<div class="flex gap-3">
			<button class="btn btn-primary" disabled={savingEdit}>
				{savingEdit ? 'Saving…' : 'Save changes'}
			</button>
			<button type="button" class="btn btn-ghost" onclick={() => (editing = false)}>Cancel</button>
		</div>
	</form>
{:else}
	<div class="mt-4 flex flex-wrap items-end justify-between gap-4">
		<div>
			<h1 class="font-display text-3xl sm:text-4xl">{classroom.name}</h1>
			<p class="mt-1 text-muted">
				{classroom.institutionName}
				{#if classroom.myRole}· you are {CLASSROOM_ROLE_LABELS[classroom.myRole].toLowerCase()}{/if}
			</p>
		</div>
		{#if permissions.edit}
			<button class="btn btn-ghost" onclick={startEdit}>Edit details</button>
		{/if}
	</div>
{/if}

{#if permissions.viewCode && classroom.code}
	<section
		class="card mt-8 flex flex-wrap items-center justify-between gap-4 border-teal/25 bg-forest text-ink"
	>
		<div>
			<p class="text-sm text-muted">Global classroom code</p>
			<p class="mt-1 font-mono text-3xl tracking-wider text-lime">{classroom.code}</p>
			<p class="mt-2 max-w-xl text-sm text-muted">
				Registered in SAM as this classroom's source identifier. Only facilitators and the
				institution head can see it.
			</p>
		</div>
		<CopyButton text={classroom.code} label="Copy code" />
	</section>
{/if}

{#if actionError}<p class="field-error mt-4" role="alert">{actionError}</p>{/if}

<div class="mt-8">
	<VerticalTabs {tabs} bind:active={tab} label="Classroom">
		{#if tab === 'labs'}
			<section class="card">
				<div class="flex flex-wrap items-center justify-between gap-3">
					<h2 class="flex items-center gap-2 text-lg font-semibold">
						<Icon name={ICONS.labs} class="size-5 text-muted" /> Labs
						<span class="text-muted">{data.labs.length}</span>
					</h2>
					{#if permissions.invite.length && data.labs.length}
						<a
							href={resolve('/(app)/classrooms/[id]/labs/new', { id: classroom.id })}
							class="btn btn-primary"><Icon name="plus" class="size-4" /> Add a lab</a
						>
					{/if}
				</div>
				{#if data.labs.length}
					<ul class="mt-4 grid gap-3 sm:grid-cols-2">
						{#each data.labs as lab (lab.id)}
							<li>
								<a
									href={resolve('/(app)/labs/[labId]', { labId: lab.id })}
									class="block rounded-md border border-line p-4 transition hover:border-line-strong"
								>
									<p class="font-medium">{lab.name}</p>
									<p class="text-sm text-muted">{lab.placeName}, {lab.country}</p>
								</a>
							</li>
						{/each}
					</ul>
				{:else if permissions.invite.length}
					<div
						class="mt-4 flex flex-wrap items-center justify-between gap-4 rounded-md bg-lime/25 p-4"
					>
						<p class="text-sm">
							<strong>Next step:</strong> register your lab, its site profile and its devices.
						</p>
						<a
							href={resolve('/(app)/classrooms/[id]/labs/new?onboarding', { id: classroom.id })}
							class="btn btn-primary">Set up your lab</a
						>
					</div>
				{:else}
					<p class="mt-2 text-sm text-muted">Your facilitator hasn't registered a lab yet.</p>
				{/if}
			</section>
		{:else if tab === 'members'}
			<section class="card">
				<h2 class="flex items-center gap-2 text-lg font-semibold">
					<Icon name={ICONS.members} class="size-5 text-muted" /> Members
					<span class="text-muted">{data.detail.members.length}</span>
				</h2>
				<ul class="mt-4 divide-y divide-line">
					{#each data.detail.members as member (member.userId)}
						{@const editable =
							permissions.manageMembers &&
							member.role !== 'institution_head' &&
							member.userId !== data.user.id}
						<li class="flex flex-wrap items-center gap-3 py-3">
							<div class="min-w-0 flex-1">
								<p class="font-medium">
									{member.fullName}
									{#if member.userId === data.user.id}<span class="text-muted">(you)</span>{/if}
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
		{:else if tab === 'consent'}
			<section class="card">
				<div class="flex flex-wrap items-center justify-between gap-3">
					<h2 class="flex items-center gap-2 text-lg font-semibold">
						<Icon name={ICONS.consent} class="size-5 text-muted" /> Guardian consent
						<span class="text-muted">{data.consent.length}</span>
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
				{#if data.consent.length}
					<ul class="mt-4 divide-y divide-line">
						{#each data.consent as member (member.userId)}
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
		{:else}
			<section class="card">
				<div class="flex flex-wrap items-center justify-between gap-3">
					<h2 class="flex items-center gap-2 text-lg font-semibold">
						<Icon name={ICONS.invitations} class="size-5 text-muted" /> Pending invitations
						<span class="text-muted">{data.detail.invitations.length}</span>
					</h2>
					<button type="button" class="btn btn-primary" aria-haspopup="dialog" onclick={openInvite}
						><Icon name="plus" class="size-4" /> Invite people</button
					>
				</div>
				<p class="mt-1 text-sm text-muted">
					Each person gets a link to sign up and join. Only invited people can join.
				</p>
				{#if data.detail.invitations.length}
					<ul class="mt-4 divide-y divide-line">
						{#each data.detail.invitations as invitation (invitation.id)}
							<li class="flex flex-wrap items-center gap-2 py-3">
								<div class="min-w-0 flex-1">
									<p class="truncate text-sm font-medium">{invitation.email}</p>
									<p class="text-xs text-muted">
										{CLASSROOM_ROLE_LABELS[invitation.role]} · invited by {invitation.invitedByName}
									</p>
								</div>
								<CopyButton text={inviteUrl(invitation.token)} label="Copy link" />
								{#if permissions.invite.includes(invitation.role)}
									<button
										class="btn btn-ghost px-3 py-1.5 text-xs"
										onclick={() => resend(invitation)}>Resend</button
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
	</VerticalTabs>
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
