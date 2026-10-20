<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import CopyButton from '#lib/components/CopyButton.svelte';
	import type { ClassroomRole, Invitation } from '#lib/domain/types.ts';
	import { CLASSROOM_ROLE_LABELS } from '#lib/labels.ts';
	import { errorMessage, isMockDataSource, services, ServiceError } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const classroom = $derived(data.detail.classroom);
	const permissions = $derived(data.detail.permissions);

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

	function remove(userId: string, name: string) {
		if (!confirm(`Remove ${name} from ${classroom.name}?`)) return;
		run(() => services.classrooms.removeMember(classroom.id, userId));
	}
</script>

<svelte:head><title>{classroom.name} · DaGrouper Analysis</title></svelte:head>

<a href={resolve('/(app)/classrooms')} class="text-sm text-muted hover:text-ink">← Classrooms</a>

<div class="mt-4 flex flex-wrap items-end justify-between gap-4">
	<div>
		<h1 class="font-display text-5xl">{classroom.name}</h1>
		<p class="mt-1 text-muted">
			{classroom.institutionName}
			{#if classroom.myRole}· you are {CLASSROOM_ROLE_LABELS[classroom.myRole].toLowerCase()}{/if}
		</p>
	</div>
</div>

<section class="card mt-8 flex flex-wrap items-center justify-between gap-4 bg-forest text-white">
	<div>
		<p class="text-sm text-white/60">Global classroom code</p>
		{#if classroom.code}
			<p class="mt-1 font-mono text-3xl tracking-wider text-lime">{classroom.code}</p>
			<p class="mt-2 max-w-xl text-sm text-white/60">
				Registered in SAM as this classroom's source identifier. Only facilitators and the
				institution head can see it.
			</p>
		{:else}
			<p class="mt-1 text-lg">Visible to facilitators and the institution head</p>
		{/if}
	</div>
	{#if classroom.code}<CopyButton text={classroom.code} label="Copy code" />{/if}
</section>

{#if actionError}<p class="field-error mt-4" role="alert">{actionError}</p>{/if}

<section class="card mt-6">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<h2 class="text-lg font-semibold">Labs <span class="text-muted">{data.labs.length}</span></h2>
		{#if permissions.invite.length && data.labs.length}
			<a
				href={resolve('/(app)/classrooms/[id]/labs/new', { id: classroom.id })}
				class="btn btn-ghost px-3 py-1.5 text-xs">Add a lab</a
			>
		{/if}
	</div>
	{#if data.labs.length}
		<ul class="mt-4 grid gap-3 sm:grid-cols-2">
			{#each data.labs as lab (lab.id)}
				<li>
					<a
						href={resolve('/(app)/labs/[labId]', { labId: lab.id })}
						class="block rounded-2xl border border-line p-4 transition hover:border-forest/40"
					>
						<p class="font-medium">{lab.name}</p>
						<p class="text-sm text-muted">{lab.placeName}, {lab.country}</p>
					</a>
				</li>
			{/each}
		</ul>
	{:else if permissions.invite.length}
		<div class="mt-4 flex flex-wrap items-center justify-between gap-4 rounded-2xl bg-lime/25 p-4">
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

<div class="mt-6 grid gap-6 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
	<section class="card">
		<h2 class="text-lg font-semibold">
			Members <span class="text-muted">{data.detail.members.length}</span>
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
							class="input w-auto py-1.5 text-sm"
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
						</select>
						<button
							class="btn btn-ghost px-3 py-1.5 text-xs"
							onclick={() => remove(member.userId, member.fullName)}>Remove</button
						>
					{:else}
						<span class="rounded-full bg-canvas px-2.5 py-1 text-xs font-medium">
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
		<div class="space-y-6">
			<section class="card">
				<h2 class="text-lg font-semibold">Invite by email</h2>
				<p class="mt-1 text-sm text-muted">
					Each person gets a link to sign up and join. Only invited people can join.
				</p>
				<form class="mt-4 space-y-4" onsubmit={invite} novalidate>
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
					<button class="btn btn-primary" disabled={inviting || !emails.trim()}>
						{inviting ? 'Sending…' : 'Send invitations'}
					</button>
				</form>
				{#if sent.length}
					<div
						class="mt-4 rounded-2xl border border-lime-strong/50 bg-lime/25 p-4 text-sm"
						role="status"
					>
						<p class="font-medium">
							Sent {sent.length}
							{sent.length === 1 ? 'invitation' : 'invitations'}.
						</p>
						{#if isMockDataSource}
							<p class="mt-1 text-muted">Demo mode sends no email. Share the links below.</p>
						{/if}
					</div>
				{/if}
			</section>

			<section class="card">
				<h2 class="text-lg font-semibold">
					Pending invitations <span class="text-muted">{data.detail.invitations.length}</span>
				</h2>
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
								<button
									class="btn btn-ghost px-3 py-1.5 text-xs"
									onclick={() => run(() => services.classrooms.revokeInvitation(invitation.id))}
									>Revoke</button
								>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="mt-2 text-sm text-muted">No pending invitations.</p>
				{/if}
			</section>
		</div>
	{/if}
</div>
