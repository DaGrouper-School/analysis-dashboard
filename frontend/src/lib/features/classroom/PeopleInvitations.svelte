<script lang="ts">
	import { resolve } from '$app/paths';
	import CopyButton from '#lib/components/CopyButton.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Modal from '#lib/components/Modal.svelte';
	import Select, { toOptions } from '#lib/components/Select.svelte';
	import type { ClassroomDetail, ClassroomRole, Invitation } from '#lib/domain/types.ts';
	import { CLASSROOM_ROLE_LABELS } from '#lib/utils/labels.ts';
	import { errorMessage, isMockDataSource, services, ServiceError } from '#lib/services/index.ts';

	/**
	 * A classroom's pending invitations, with the dialog that sends new ones. Shown to those who
	 * may invite; resend and revoke only for the roles the viewer can invite.
	 */
	let {
		detail,
		icon,
		onrun,
		onrefresh
	}: {
		detail: ClassroomDetail;
		icon: string;
		/** Runs a change, reports its error, and reloads the classroom. */
		onrun: (action: () => Promise<unknown>) => void;
		onrefresh: () => Promise<void>;
	} = $props();

	const classroom = $derived(detail.classroom);
	const permissions = $derived(detail.permissions);

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

	function inviteUrl(token: string) {
		return `${location.origin}${resolve('/invite/[token]', { token })}`;
	}

	async function invite(event: SubmitEvent) {
		event.preventDefault();
		inviteError = '';
		inviting = true;
		try {
			sent = await services.classrooms.invite(classroom.id, emails.split(/[\s,;]+/), inviteRole);
			emails = '';
			inviting = false;
			await onrefresh();
		} catch (e) {
			inviteError =
				e instanceof ServiceError && e.fields.emails ? e.fields.emails : errorMessage(e);
		} finally {
			inviting = false;
		}
	}

	/** Sends a fresh link. The old one stops working. */
	function resend(invitation: Invitation) {
		onrun(() => services.classrooms.invite(classroom.id, [invitation.email], invitation.role));
	}
</script>

<section class="card">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<h2 class="flex items-center gap-2 text-lg font-semibold">
			<Icon name={icon} class="size-5 text-muted" /> Pending invitations
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
							><Icon name="mail" class="size-3.5" /> Resend</button
						>
						<button
							class="btn btn-ghost px-3 py-1.5 text-xs"
							onclick={() => onrun(() => services.classrooms.revokeInvitation(invitation.id))}
							><Icon name="trash" class="size-3.5" /> Revoke</button
						>
					{/if}
				</li>
			{/each}
		</ul>
	{:else}
		<p class="mt-2 text-sm text-muted">No pending invitations.</p>
	{/if}
</section>

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
				<Select
					id="inviteRole"
					options={toOptions(permissions.invite, CLASSROOM_ROLE_LABELS)}
					bind:value={inviteRole}
				/>
			</div>
		</form>
	{/if}
	{#snippet footer()}
		{#if sent.length}
			<button type="button" class="btn btn-ghost" onclick={openInvite}
				><Icon name="plus" class="size-4" /> Invite more</button
			>
			<button type="button" class="btn btn-primary" onclick={() => (inviteOpen = false)}
				><Icon name="check" class="size-4" /> Done</button
			>
		{:else}
			<button type="button" class="btn btn-ghost" onclick={() => (inviteOpen = false)}
				><Icon name="close" class="size-4" /> Cancel</button
			>
			<button form="invite-form" class="btn btn-primary" disabled={inviting || !emails.trim()}>
				<Icon name="mail" class="size-4" />
				{inviting ? 'Sending…' : 'Send invitations'}
			</button>
		{/if}
	{/snippet}
</Modal>
