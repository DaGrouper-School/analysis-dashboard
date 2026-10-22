<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import SetupSteps from '#lib/components/SetupSteps.svelte';
	import Wizard from '#lib/components/Wizard.svelte';
	import { validateClassroomUpdate } from '#lib/domain/validation.ts';
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
	let step = $state(0);

	/** Admins say who runs the classroom; everyone else runs their own. */
	const stepIds = $derived([
		'classroom',
		...(data.user.platformAdmin ? ['owner'] : []),
		'review'
	] as const);
	const STEP_INFO = {
		classroom: { label: 'Classroom', icon: 'flask' },
		owner: { label: 'Who runs it', icon: 'user' },
		review: { label: 'Review', icon: 'review' }
	};
	const steps = $derived(stepIds.map((id) => STEP_INFO[id as keyof typeof STEP_INFO]));
	const stepId = $derived(stepIds[step]);

	/** Fields checked before leaving each step. The server checks them again. */
	function check(at: number) {
		const id = stepIds[at];
		errors = {};
		if (id === 'classroom' || id === 'review') {
			errors = validateClassroomUpdate({ name, institutionName });
		}
		if ((id === 'owner' || id === 'review') && forSomeoneElse && !headEmail.trim()) {
			errors.headEmail = "Enter the institution head's email.";
		}
		const fields = Object.keys(errors);
		if (id === 'review' && fields.length) step = stepIds.indexOf(stepOf(fields[0]));
		return !fields.length;
	}

	const stepOf = (field: string) => (field === 'headEmail' ? 'owner' : 'classroom');

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		errors = {};
		formError = '';
		saving = true;
		try {
			const classroom = await services.classrooms.create({
				name,
				institutionName,
				headEmail: forSomeoneElse ? headEmail : undefined
			});
			// Classroom onboarding continues with the lab and its devices (AP-05).
			await goto(resolve('/(app)/classrooms/[id]/labs/new?onboarding', { id: classroom.id }));
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
<div class="mt-4"><SetupSteps current={1} /></div>
<h1 class="font-display text-3xl sm:text-4xl">New classroom</h1>
<p class="mt-1 max-w-xl text-muted">
	The classroom gets a unique global classroom code. It is registered in SAM as the source
	identifier for the classroom's stations and observations.
</p>

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
		{:else if stepId === 'owner'}
			<fieldset class="space-y-3">
				<legend class="flex items-center gap-2 text-lg font-semibold">
					<Icon name={STEP_INFO.owner.icon} class="size-5 text-muted" /> Who runs this classroom?
				</legend>
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
		{:else}
			<h2 class="flex items-center gap-2 text-lg font-semibold">
				<Icon name={STEP_INFO.review.icon} class="size-5 text-muted" /> Check and create
			</h2>
			<dl class="mt-4 grid gap-3 text-sm sm:grid-cols-[9rem_1fr]">
				<dt class="text-muted">Classroom</dt>
				<dd class="text-ink">{name}</dd>
				<dt class="text-muted">Institution</dt>
				<dd class="text-ink">{institutionName}</dd>
				<dt class="text-muted">Run by</dt>
				<dd class="text-ink">
					{forSomeoneElse
						? `${headEmail}, invited as institution head`
						: 'You, as institution head'}
				</dd>
			</dl>
			{#if formError}<p class="field-error mt-3" role="alert">{formError}</p>{/if}
		{/if}
	</Wizard>
</div>
