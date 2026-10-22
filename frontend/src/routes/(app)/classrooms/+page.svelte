<script lang="ts">
	import { resolve } from '$app/paths';
	import PendingInvitations from '#lib/components/PendingInvitations.svelte';
	import { CLASSROOM_ROLE_LABELS } from '#lib/labels.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head><title>Classrooms · DaGrouper Analysis</title></svelte:head>

<div class="flex flex-wrap items-end justify-between gap-4">
	<div>
		<h1 class="font-display text-3xl sm:text-4xl">Classrooms</h1>
		<p class="mt-1 text-muted">
			{data.user.platformAdmin
				? 'Every classroom on the platform.'
				: 'The global classrooms you belong to.'}
		</p>
	</div>
	{#if data.canCreate}
		<a href={resolve('/(app)/classrooms/new')} class="btn btn-primary">New classroom</a>
	{/if}
</div>

{#if data.invitations.length}
	<div class="mt-8"><PendingInvitations invitations={data.invitations} /></div>
{/if}

{#if data.classrooms.length}
	<ul class="mt-8 grid gap-4 md:grid-cols-2">
		{#each data.classrooms as classroom (classroom.id)}
			<li>
				<a
					href={resolve('/(app)/classrooms/[id]', { id: classroom.id })}
					class="card block transition hover:border-line-strong"
				>
					<div class="flex items-start justify-between gap-3">
						<div>
							<h2 class="text-lg font-semibold">{classroom.name}</h2>
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
	<div class="card mt-8 text-center">
		<h2 class="font-semibold">You're not in a classroom yet</h2>
		<p class="mx-auto mt-2 max-w-md text-sm text-muted">
			{data.canCreate
				? 'Create one and invite its institution head. They set up the lab and invite the rest.'
				: data.invitations.length
					? 'Join from the invitation above.'
					: data.user.role === 'institution_head'
						? 'A platform admin sets up classrooms and invites the institution head. Your invitation arrives by email.'
						: 'Ask your facilitator or institution head for an invitation. It arrives by email.'}
		</p>
	</div>
{/if}
