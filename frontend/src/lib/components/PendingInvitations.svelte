<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { InvitationPreview } from '#lib/domain/types.ts';
	import { CLASSROOM_ROLE_LABELS } from '#lib/labels.ts';
	import { errorMessage, services } from '#lib/services/index.ts';

	/** Invitations waiting for the signed-in user, with a button to join each classroom. */
	let { invitations }: { invitations: InvitationPreview[] } = $props();

	let busy = $state('');
	let error = $state('');

	async function join(token: string) {
		busy = token;
		error = '';
		try {
			const classroom = await services.classrooms.acceptInvitation(token);
			await invalidateAll();
			await goto(resolve('/(app)/classrooms/[id]', { id: classroom.id }));
		} catch (e) {
			error = errorMessage(e);
		} finally {
			busy = '';
		}
	}
</script>

{#if invitations.length}
	<section class="card border-teal/40" aria-labelledby="invitations-heading">
		<h2 id="invitations-heading" class="font-semibold">
			{invitations.length === 1 ? 'You have an invitation' : 'You have invitations'}
		</h2>
		<ul class="mt-3 divide-y divide-line">
			{#each invitations as invitation (invitation.token)}
				<li class="flex flex-wrap items-center justify-between gap-3 py-3">
					<div>
						<p class="text-ink">
							{invitation.classroomName}
							<span class="text-muted">· {invitation.institutionName}</span>
						</p>
						<p class="text-sm text-muted">
							{invitation.invitedByName} invited you as {CLASSROOM_ROLE_LABELS[
								invitation.role
							].toLowerCase()}
						</p>
					</div>
					<button
						type="button"
						class="btn btn-primary"
						disabled={busy !== ''}
						onclick={() => join(invitation.token)}
					>
						{busy === invitation.token ? 'Joining…' : 'Join classroom'}
					</button>
				</li>
			{/each}
		</ul>
		{#if error}<p class="field-error" role="alert">{error}</p>{/if}
	</section>
{/if}
