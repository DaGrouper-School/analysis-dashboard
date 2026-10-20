<script lang="ts">
	import { goto, invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import AuthShell from '#lib/components/AuthShell.svelte';
	import { CLASSROOM_ROLE_LABELS } from '#lib/labels.ts';
	import { savePendingInvite } from '#lib/pending-invite.ts';
	import { nextRoute } from '#lib/routing.ts';
	import { errorMessage, services } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data, params }: PageProps = $props();

	let busy = $state(false);
	let error = $state('');

	async function startSignUp() {
		savePendingInvite(params.token);
		await goto(resolve(nextRoute(data.user)));
	}

	async function join() {
		busy = true;
		error = '';
		try {
			const classroom = await services.classrooms.acceptInvitation(params.token);
			await invalidate('app:session');
			await goto(resolve('/(app)/classrooms/[id]', { id: classroom.id }), { replaceState: true });
		} catch (e) {
			error = errorMessage(e);
		} finally {
			busy = false;
		}
	}

	async function switchAccount() {
		await services.auth.signOut();
		await invalidate('app:session');
		await startSignUp();
	}
</script>

<svelte:head><title>Classroom invitation · DaGrouper Analysis</title></svelte:head>

<AuthShell>
	{#if !data.invitation}
		<h1 class="font-display text-4xl">Invitation not found</h1>
		<p class="mt-2 text-muted">{data.error} Ask the person who invited you for a new link.</p>
		<a href={resolve('/')} class="btn btn-ghost mt-8 self-start">Go to DaGrouper</a>
	{:else}
		{@const invitation = data.invitation}
		<p class="text-sm text-muted">{invitation.invitedByName} invited you to join</p>
		<h1 class="mt-1 font-display text-4xl">{invitation.classroomName}</h1>
		<p class="mt-1 text-muted">{invitation.institutionName}</p>

		<dl class="card mt-8 grid grid-cols-2 gap-4 p-5 text-sm">
			<div>
				<dt class="text-muted">Your role</dt>
				<dd class="mt-1 font-semibold">{CLASSROOM_ROLE_LABELS[invitation.role]}</dd>
			</div>
			<div>
				<dt class="text-muted">Sent to</dt>
				<dd class="mt-1 font-semibold break-all">{invitation.email}</dd>
			</div>
		</dl>

		<div class="mt-8 space-y-4">
			{#if invitation.status === 'accepted'}
				<p class="text-muted">This invitation has already been used.</p>
				<a href={resolve('/')} class="btn btn-ghost">Go to DaGrouper</a>
			{:else if !data.user}
				<button class="btn btn-primary w-full py-3" onclick={startSignUp}>
					Sign up or sign in to join
				</button>
				<p class="text-sm text-muted">
					Use {invitation.email} as your email so we can match you to this invitation.
				</p>
			{:else if !data.user.onboardingCompleted}
				<button class="btn btn-primary w-full py-3" onclick={startSignUp}>
					Finish setting up your account
				</button>
			{:else}
				<button class="btn btn-primary w-full py-3" onclick={join} disabled={busy}>
					{busy ? 'Joining…' : 'Join classroom'}
				</button>
				<p class="text-sm text-muted">Signed in as {data.user.fullName} ({data.user.email}).</p>
				{#if error}
					<p class="field-error" role="alert">{error}</p>
					<button class="link text-sm" onclick={switchAccount}>Use a different account</button>
				{/if}
			{/if}
		</div>
	{/if}
</AuthShell>
