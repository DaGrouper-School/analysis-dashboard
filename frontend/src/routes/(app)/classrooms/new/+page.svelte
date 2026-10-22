<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import Wizard from '#lib/components/Wizard.svelte';
	import { isValidEmail, validateClassroomUpdate } from '#lib/domain/validation.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let name = $state('');
	let institutionName = $state('');
	let headEmail = $state('');
	let errors = $state<Record<string, string>>({});
	let formError = $state('');
	let saving = $state(false);
	let step = $state(0);

	const stepIds = ['classroom', 'head', 'review'] as const;
	const STEP_INFO = {
		classroom: { label: 'Classroom', icon: 'flask' },
		head: { label: 'Institution head', icon: 'user' },
		review: { label: 'Review', icon: 'review' }
	};
	const steps = stepIds.map((id) => STEP_INFO[id]);
	const stepId = $derived(stepIds[step]);

	/** Fields checked before leaving each step. The server checks them again. */
	function check(at: number) {
		const id = stepIds[at];
		errors = {};
		if (id === 'classroom' || id === 'review') {
			errors = validateClassroomUpdate({ name, institutionName });
		}
		if (id === 'head' || id === 'review') {
			if (!headEmail.trim()) errors.headEmail = "Enter the institution head's email.";
			else if (!isValidEmail(headEmail.trim())) errors.headEmail = 'Enter a valid email address.';
		}
		const fields = Object.keys(errors);
		if (id === 'review' && fields.length) step = stepIds.indexOf(stepOf(fields[0]));
		return !fields.length;
	}

	const stepOf = (field: string) => (field === 'headEmail' ? 'head' : 'classroom');

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		errors = {};
		formError = '';
		saving = true;
		try {
			const classroom = await services.classrooms.create({ name, institutionName, headEmail });
			// The head takes it from here: they accept, set up the lab and invite the rest.
			await goto(resolve('/(app)/classrooms/[id]?tab=invitations', { id: classroom.id }));
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			const fields = Object.keys(errors);
			if (fields.length) step = Math.max(0, stepIds.indexOf(stepOf(fields[0])));
			formError = errorMessage(e);
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head><title>New classroom · DaGrouper Analysis</title></svelte:head>

<a href={resolve('/(app)/classrooms')} class="text-sm text-muted hover:text-ink">← Classrooms</a>
<h1 class="mt-4 font-display text-3xl sm:text-4xl">New classroom</h1>
<p class="mt-1 max-w-xl text-muted">
	The classroom gets a unique global classroom code. It is registered in SAM as the source
	identifier for the classroom's stations and observations.
</p>

{#if !data.user.platformAdmin}
	<div class="card mt-8 max-w-xl">
		<p class="text-sm text-muted">
			A platform admin sets up classrooms and invites the institution head.
		</p>
	</div>
{:else}
	<div class="card mt-8 max-w-xl">
		<Wizard
			{steps}
			bind:current={step}
			{check}
			onsubmit={submit}
			submitLabel="Create classroom"
			busy={saving}
			busyLabel="Creating…"
			label="New classroom"
		>
			{#if stepId === 'classroom'}
				<div class="space-y-5">
					<h2 class="flex items-center gap-2 text-lg font-semibold">
						<Icon name={STEP_INFO.classroom.icon} class="size-5 text-muted" /> The classroom
					</h2>
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
				</div>
			{:else if stepId === 'head'}
				<div class="space-y-3">
					<h2 class="flex items-center gap-2 text-lg font-semibold">
						<Icon name={STEP_INFO.head.icon} class="size-5 text-muted" /> Institution head
					</h2>
					<p class="text-sm text-muted">
						They get an invitation by email. Once they accept, they set up the lab and invite
						facilitators, community scientists and researchers.
					</p>
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
				</div>
			{:else}
				<h2 class="flex items-center gap-2 text-lg font-semibold">
					<Icon name={STEP_INFO.review.icon} class="size-5 text-muted" /> Check and create
				</h2>
				<dl class="mt-4 grid gap-3 text-sm sm:grid-cols-[9rem_1fr]">
					<dt class="text-muted">Classroom</dt>
					<dd class="text-ink">{name}</dd>
					<dt class="text-muted">Institution</dt>
					<dd class="text-ink">{institutionName}</dd>
					<dt class="text-muted">Institution head</dt>
					<dd class="text-ink">{headEmail}, invited when you create it</dd>
				</dl>
				{#if formError}<p class="field-error mt-3" role="alert">{formError}</p>{/if}
			{/if}
		</Wizard>
	</div>
{/if}
