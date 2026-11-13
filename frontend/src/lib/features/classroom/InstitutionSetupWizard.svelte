<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import Wizard from '#lib/components/Wizard.svelte';
	import InstitutionQuestionField from '#lib/features/classroom/InstitutionQuestionField.svelte';
	import { stepOfField, validateInstitutionAnswers } from '#lib/domain/institution-setup.ts';
	import type {
		InstitutionField,
		InstitutionQuestion,
		InstitutionSetup,
		InstitutionSetupForm
	} from '#lib/domain/types.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';

	/**
	 * The institution setup (AP-40): the questions from configuration, a step at a time, then a
	 * review. Every step change saves, so "Finish later" (or closing the tab) loses nothing and the
	 * head comes back to the same step.
	 */
	let {
		form,
		setup,
		ondone,
		onlater
	}: {
		form: InstitutionSetupForm;
		setup: InstitutionSetup;
		/** After the last step is saved. */
		ondone: () => unknown;
		/** After "Finish later" saved the answers so far. */
		onlater: () => unknown;
	} = $props();

	// svelte-ignore state_referenced_locally
	let answers = $state(structuredClone(setup.answers));
	// A finished setup opens at the start, for changes; an unfinished one where it was left.
	// svelte-ignore state_referenced_locally
	let step = $state(setup.completedAt ? 0 : Math.min(setup.step, form.steps.length));
	let errors = $state<Partial<Record<InstitutionField, string>>>({});
	let formError = $state('');
	let busy = $state(false);

	const steps = $derived([
		...form.steps.map((s) => ({ label: s.label, icon: s.icon })),
		{ label: 'Review', icon: 'review' }
	]);
	const reviewing = $derived(step === form.steps.length);
	const shown = $derived(form.steps[step]);
	const questions = $derived(form.questions.filter((q) => q.step === shown?.id));

	function check(at: number) {
		errors = validateInstitutionAnswers(answers, form, {
			step: at < form.steps.length ? form.steps[at].id : undefined
		});
		const fields = Object.keys(errors);
		if (at === form.steps.length && fields.length) step = stepOfField(form, fields[0]);
		return !fields.length;
	}

	function clear(field: InstitutionField, pickError = '') {
		const rest = Object.fromEntries(Object.entries(errors).filter(([f]) => f !== field));
		errors = pickError ? { ...rest, [field]: pickError } : rest;
	}

	async function save(finish: boolean) {
		formError = '';
		busy = true;
		try {
			await services.classrooms.saveInstitutionSetup(setup.classroomId, {
				answers: $state.snapshot(answers),
				step,
				finish
			});
			return true;
		} catch (e) {
			if (e instanceof ServiceError && Object.keys(e.fields).length) {
				errors = e.fields;
				step = stepOfField(form, Object.keys(e.fields)[0]);
			}
			formError = errorMessage(e);
			return false;
		} finally {
			busy = false;
		}
	}

	async function finish(event: SubmitEvent) {
		event.preventDefault();
		if (await save(true)) await ondone();
	}

	async function later() {
		if (await save(false)) await onlater();
	}

	/** An answer as the review lists it. */
	function shownAnswer(q: InstitutionQuestion): string {
		const value = answers[q.field];
		if (Array.isArray(value)) return value.join(', ') || '—';
		if (q.kind === 'yes_no') return value === 'yes' ? 'Yes' : value === 'no' ? 'No' : '—';
		return value?.trim() || '—';
	}
</script>

<Wizard
	{steps}
	bind:current={step}
	{check}
	onsubmit={finish}
	onstep={() => save(false)}
	submitLabel={setup.completedAt ? 'Save changes' : 'Finish setup'}
	{busy}
	label="Set up {setup.institutionName}"
>
	{#snippet extra()}
		<button type="button" class="btn btn-ghost" disabled={busy} onclick={later}>
			Finish later
		</button>
	{/snippet}

	<div class="space-y-6">
		{#if reviewing}
			<h2 class="flex items-center gap-2 text-lg font-semibold">
				<Icon name="review" class="size-5 text-muted" /> Review
			</h2>
			<p class="text-sm text-muted">Check your answers. You can change them later from Settings.</p>
			{#if answers.photo}
				<img
					src={answers.photo}
					alt={answers.photoAlt}
					class="aspect-[16/9] w-full rounded-md border border-line object-cover"
				/>
			{/if}
			<dl class="grid gap-x-6 gap-y-3 text-sm sm:grid-cols-[minmax(0,14rem)_1fr]">
				{#each form.questions.filter((q) => q.kind !== 'photo') as q (q.field)}
					<dt class="text-muted">{q.label}</dt>
					<dd class="break-words whitespace-pre-line text-ink">{shownAnswer(q)}</dd>
				{/each}
			</dl>
		{:else if shown}
			<div>
				<h2 class="flex items-center gap-2 text-lg font-semibold">
					<Icon name={shown.icon} class="size-5 text-muted" />
					{shown.label}
				</h2>
				{#if shown.intro}<p class="mt-1 text-sm text-muted">{shown.intro}</p>{/if}
			</div>
			{#each questions as question (question.field)}
				<InstitutionQuestionField
					{question}
					bind:answers
					error={errors[question.field]}
					onchange={(pickError) => clear(question.field, pickError)}
				/>
			{/each}
		{/if}
		{#if formError}<p class="field-error" role="alert">{formError}</p>{/if}
	</div>
</Wizard>
