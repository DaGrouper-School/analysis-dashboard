<script lang="ts">
	import { resolve } from '$app/paths';
	import PendingInvitations from '#lib/components/PendingInvitations.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import type { ClassroomSummary, InvitationPreview } from '#lib/domain/types.ts';
	import { CLASSROOM_ROLE_LABELS } from '#lib/labels.ts';

	/**
	 * The classrooms someone belongs to (every classroom, for a platform admin), with their
	 * pending invitations. Each opens its classroom page, where labs are added.
	 */
	let {
		classrooms,
		invitations,
		canCreate,
		platformAdmin,
		head,
		icon
	}: {
		classrooms: ClassroomSummary[];
		invitations: InvitationPreview[];
		canCreate: boolean;
		platformAdmin: boolean;
		/** An institution head waits for a platform admin's invitation. */
		head: boolean;
		/** The section title's icon, as on its tab. */
		icon: string;
	} = $props();
</script>

<div class="flex flex-wrap items-end justify-between gap-4">
	<div>
		<h2 class="flex items-center gap-2 text-xl font-semibold">
			<Icon name={icon} class="size-5 text-muted" /> Classrooms
		</h2>
		<p class="mt-1 text-sm text-muted">
			{platformAdmin
				? 'Every classroom on the platform.'
				: 'The global classrooms you belong to. Open one to add labs and invite people.'}
		</p>
	</div>
	{#if canCreate}
		<a href={resolve('/(app)/classrooms/new')} class="btn btn-primary">New classroom</a>
	{/if}
</div>

{#if invitations.length}
	<div class="mt-6"><PendingInvitations {invitations} /></div>
{/if}

{#if classrooms.length}
	<ul class="mt-6 grid gap-4 md:grid-cols-2">
		{#each classrooms as classroom (classroom.id)}
			<li>
				<a
					href={resolve('/(app)/classrooms/[id]', { id: classroom.id })}
					class="card block transition hover:border-line-strong"
				>
					<div class="flex items-start justify-between gap-3">
						<div class="min-w-0">
							<h3 class="text-lg font-semibold break-words">{classroom.name}</h3>
							<p class="text-sm text-muted">{classroom.institutionName}</p>
						</div>
						{#if classroom.myRole}
							<span
								class="rounded-full bg-lime/15 px-2.5 py-0.5 text-xs font-medium whitespace-nowrap text-lime ring-1 ring-lime/40"
							>
								{CLASSROOM_ROLE_LABELS[classroom.myRole]}
							</span>
						{/if}
					</div>
					<p class="mt-6 flex items-center justify-between text-sm text-muted">
						<span>{classroom.memberCount} {classroom.memberCount === 1 ? 'member' : 'members'}</span
						>
						{#if classroom.code}<span class="font-mono text-ink">{classroom.code}</span>{/if}
					</p>
				</a>
			</li>
		{/each}
	</ul>
{:else}
	<div class="card mt-6 text-center">
		<h3 class="font-semibold">You're not in a classroom yet</h3>
		<p class="mx-auto mt-2 max-w-md text-sm text-muted">
			{canCreate
				? 'Create one and invite its institution head. They set up the lab and invite the rest.'
				: invitations.length
					? 'Join from the invitation above.'
					: head
						? 'A platform admin sets up classrooms and invites the institution head. Your invitation arrives by email.'
						: 'Ask your facilitator or institution head for an invitation. It arrives by email.'}
		</p>
	</div>
{/if}
