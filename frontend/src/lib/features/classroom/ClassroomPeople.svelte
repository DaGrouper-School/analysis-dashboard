<script lang="ts">
	import { invalidate } from '$app/navigation';
	import Icon from '#lib/components/Icon.svelte';
	import Select, { toOptions } from '#lib/components/Select.svelte';
	import type { ClassroomDetail, ClassroomRole, MemberConsent } from '#lib/domain/types.ts';
	import { CLASSROOM_ROLE_LABELS } from '#lib/utils/labels.ts';
	import { errorMessage, services } from '#lib/services/index.ts';
	import PeopleConsent from './PeopleConsent.svelte';
	import PeopleInvitations from './PeopleInvitations.svelte';

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
	/** Roles a member can be moved between; the institution head is set elsewhere. */
	const MEMBER_ROLES: ClassroomRole[] = ['facilitator', 'community_scientist', 'researcher'];

	let actionError = $state('');

	async function run(action: () => Promise<unknown>) {
		actionError = '';
		try {
			await action();
			await refresh();
		} catch (e) {
			actionError = errorMessage(e);
		}
	}

	async function refresh() {
		await invalidate('app:classroom');
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
						<Select
							id="role-{member.userId}"
							size="compact"
							align="end"
							options={toOptions(MEMBER_ROLES, CLASSROOM_ROLE_LABELS)}
							value={member.role}
							onchange={(role) =>
								run(() => services.classrooms.setMemberRole(classroom.id, member.userId, role))}
						/>
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
		<PeopleInvitations {detail} icon={ICONS.invitations} onrun={run} onrefresh={refresh} />
	{/if}
	{#if permissions.manageConsent}
		<PeopleConsent {classroom} {consent} icon={ICONS.consent} onrefresh={refresh} />
	{/if}
</div>
