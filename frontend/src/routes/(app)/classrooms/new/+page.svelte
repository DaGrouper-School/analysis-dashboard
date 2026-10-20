<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let name = $state('');
	let institutionName = $state('');
	// Admins usually set up a classroom on behalf of an institution head (admin-assisted).
	// svelte-ignore state_referenced_locally
	let forSomeoneElse = $state(data.user.platformAdmin);
	let headEmail = $state('');
	let errors = $state<Record<string, string>>({});
	let formError = $state('');
	let saving = $state(false);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		errors = {};
		formError = '';
		if (forSomeoneElse && !headEmail.trim()) {
			errors = { headEmail: "Enter the institution head's email." };
			return;
		}
		saving = true;
		try {
			const classroom = await services.classrooms.create({
				name,
				institutionName,
				headEmail: forSomeoneElse ? headEmail : undefined
			});
			await goto(resolve('/(app)/classrooms/[id]', { id: classroom.id }));
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			formError = errorMessage(e);
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head><title>New classroom · DaGrouper Analysis</title></svelte:head>

<a href={resolve('/(app)/classrooms')} class="text-sm text-muted hover:text-ink">← Classrooms</a>
<h1 class="mt-4 font-display text-5xl">New classroom</h1>
<p class="mt-1 max-w-xl text-muted">
	The classroom gets a unique global classroom code. It is registered in SAM as the source
	identifier for the classroom's stations and observations.
</p>

<form class="card mt-8 max-w-xl space-y-5" onsubmit={submit} novalidate>
	<div>
		<label class="label" for="name">Classroom name</label>
		<input
			id="name"
			class={['input', errors.name && 'input-error']}
			placeholder="Trinity Lab Classroom"
			bind:value={name}
		/>
		{#if errors.name}<p class="field-error">{errors.name}</p>{/if}
	</div>
	<div>
		<label class="label" for="institutionName">Institution</label>
		<input
			id="institutionName"
			class={['input', errors.institutionName && 'input-error']}
			placeholder="Trinity School, Shashamane"
			bind:value={institutionName}
		/>
		{#if errors.institutionName}<p class="field-error">{errors.institutionName}</p>{/if}
	</div>

	{#if data.user.platformAdmin}
		<fieldset class="space-y-3 rounded-2xl border border-line p-4">
			<legend class="px-1 text-sm font-medium">Who runs this classroom?</legend>
			<label class="flex items-center gap-2 text-sm">
				<input type="radio" bind:group={forSomeoneElse} value={true} />
				An institution head I'll invite (admin-assisted setup)
			</label>
			<label class="flex items-center gap-2 text-sm">
				<input type="radio" bind:group={forSomeoneElse} value={false} />
				Me, as institution head
			</label>
			{#if forSomeoneElse}
				<div>
					<label class="label" for="headEmail">Institution head's email</label>
					<input
						id="headEmail"
						type="email"
						class={['input', errors.headEmail && 'input-error']}
						bind:value={headEmail}
					/>
					{#if errors.headEmail}<p class="field-error">{errors.headEmail}</p>{/if}
				</div>
			{/if}
		</fieldset>
	{/if}

	{#if formError}<p class="field-error" role="alert">{formError}</p>{/if}

	<button class="btn btn-primary" disabled={saving}>
		{saving ? 'Creating…' : 'Create classroom'}
	</button>
</form>
