<script lang="ts">
	import Breadcrumbs from '#lib/components/Breadcrumbs.svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import PublicShell from '#lib/components/PublicShell.svelte';
	import Wizard from '#lib/components/Wizard.svelte';
	import YesNoField from '#lib/components/YesNoField.svelte';
	import {
		emptyHostApplication,
		parseEcosystems,
		stepOfField,
		validateHostApplication,
		type HostStep
	} from '#lib/domain/hosting.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let form = $state(emptyHostApplication());
	let ecosystems = $state('');
	let errors = $state<Record<string, string>>({});
	let formError = $state('');
	let saving = $state(false);
	let step = $state(0);

	const stepIds = ['institution', 'readiness', 'site', 'review'] as const;
	const STEP_INFO = {
		institution: { label: 'Institution', icon: 'users' },
		readiness: { label: 'Readiness', icon: 'clipboard' },
		site: { label: 'Site', icon: 'pin' },
		review: { label: 'Send', icon: 'mail' }
	};
	const steps = stepIds.map((id) => STEP_INFO[id]);
	const stepId = $derived(stepIds[step]);
	const directory = $derived(data.content.sections.find((s) => s.id === 'classrooms'));

	const input = () => ({
		...form,
		site: { ...form.site, ecosystems: parseEcosystems(ecosystems) }
	});

	/** Fields checked before leaving each step. The server checks them all again. */
	function check(at: number) {
		const id = stepIds[at];
		errors = validateHostApplication(
			input(),
			data.questions,
			id === 'review' ? undefined : (id as HostStep)
		);
		const fields = Object.keys(errors);
		if (id === 'review' && fields.length) step = stepIds.indexOf(stepOfField(fields[0]));
		return !fields.length;
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		errors = {};
		formError = '';
		saving = true;
		try {
			const receipt = await services.hosting.apply(input());
			await goto(resolve('/host/status/[token]', { token: receipt.token }));
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			const fields = Object.keys(errors);
			if (fields.length) step = stepIds.indexOf(stepOfField(fields[0]));
			formError = errorMessage(e);
		} finally {
			saving = false;
		}
	}

	/** A field's error goes once it is changed; Next checks it again. */
	function clear(field: string) {
		if (errors[field])
			errors = Object.fromEntries(Object.entries(errors).filter(([f]) => f !== field));
	}

	const answerLabel = (id: string) => {
		const answer = form.answers[id];
		return answer === 'yes' ? 'Yes' : answer === 'no' ? 'No' : answer || '—';
	};
</script>

<svelte:head><title>{data.content.hosting.title} · {data.content.hero.eyebrow}</title></svelte:head>

{#snippet field(
	id: string,
	label: string,
	value: string,
	set: (v: string) => void,
	opts: { type?: string; optional?: boolean; area?: boolean; hint?: string } = {}
)}
	<div>
		<label class="label" for={id}>
			{label}
			{#if opts.optional}<span class="font-normal text-muted">(optional)</span>{/if}
		</label>
		{#if opts.area}
			<textarea
				{id}
				rows="3"
				class={['input', errors[id] && 'input-error']}
				{value}
				oninput={(e) => set(e.currentTarget.value)}></textarea>
		{:else}
			<input
				{id}
				type={opts.type ?? 'text'}
				class={['input', errors[id] && 'input-error']}
				{value}
				oninput={(e) => {
					set(e.currentTarget.value);
					clear(id);
				}}
			/>
		{/if}
		{#if opts.hint}<p class="mt-1 text-xs text-muted">{opts.hint}</p>{/if}
		{#if errors[id]}<p class="field-error">{errors[id]}</p>{/if}
	</div>
{/snippet}

<PublicShell content={data.content} user={data.user}>
	<Breadcrumbs
		items={[
			{ label: directory?.label ?? 'Classrooms', href: resolve('/classrooms') },
			{ label: data.content.hosting.title }
		]}
	/>
	<h1 class="mt-4 font-display text-4xl">{data.content.hosting.title}</h1>
	<p class="mt-2 max-w-2xl text-muted">{data.content.hosting.intro}</p>

	<div class="card mt-8 max-w-2xl">
		<Wizard
			{steps}
			bind:current={step}
			{check}
			onsubmit={submit}
			submitLabel="Send application"
			busy={saving}
			busyLabel="Sending…"
			label={data.content.hosting.title}
		>
			{#if stepId === 'institution'}
				<div class="space-y-5">
					<h2 class="flex items-center gap-2 text-lg font-semibold">
						<Icon name={STEP_INFO.institution.icon} class="size-5 text-muted" /> Institution and contact
					</h2>
					{@render field(
						'institutionName',
						'Institution',
						form.institutionName,
						(v) => (form.institutionName = v)
					)}
					{@render field(
						'contactName',
						'Your name',
						form.contactName,
						(v) => (form.contactName = v)
					)}
					{@render field(
						'contactRole',
						'Your role',
						form.contactRole,
						(v) => (form.contactRole = v),
						{
							hint: 'For example head teacher or science teacher.'
						}
					)}
					{@render field(
						'contactEmail',
						'Email',
						form.contactEmail,
						(v) => (form.contactEmail = v),
						{
							type: 'email'
						}
					)}
					{@render field(
						'contactPhone',
						'Phone',
						form.contactPhone,
						(v) => (form.contactPhone = v),
						{
							type: 'tel',
							optional: true,
							hint: 'With the country code, for example +251 911 000 000.'
						}
					)}
				</div>
			{:else if stepId === 'readiness'}
				<div class="space-y-6">
					<h2 class="flex items-center gap-2 text-lg font-semibold">
						<Icon name={STEP_INFO.readiness.icon} class="size-5 text-muted" /> Readiness
					</h2>
					{#each data.questions as q (q.id)}
						{@const key = `answers.${q.id}`}
						{#if q.kind === 'yes_no'}
							<YesNoField
								name={key}
								label={q.label}
								help={q.help}
								value={form.answers[q.id]}
								error={errors[key]}
								onchange={(value) => {
									form.answers[q.id] = value;
									clear(key);
								}}
							/>
						{:else}
							{@render field(
								key,
								q.label,
								form.answers[q.id] ?? '',
								(v) => (form.answers[q.id] = v),
								{
									type: q.kind === 'number' ? 'number' : 'text',
									optional: !q.required,
									area: q.kind === 'text',
									hint: q.help
								}
							)}
						{/if}
					{/each}
				</div>
			{:else if stepId === 'site'}
				<div class="space-y-5">
					<h2 class="flex items-center gap-2 text-lg font-semibold">
						<Icon name={STEP_INFO.site.icon} class="size-5 text-muted" /> Site profile
					</h2>
					<div class="grid gap-5 sm:grid-cols-2">
						{@render field(
							'site.placeName',
							'Town or area',
							form.site.placeName,
							(v) => (form.site.placeName = v)
						)}
						{@render field(
							'site.country',
							'Country',
							form.site.country,
							(v) => (form.site.country = v)
						)}
					</div>
					{@render field('site.ecosystems', 'Ecosystems', ecosystems, (v) => (ecosystems = v), {
						hint: 'Separate them with commas, for example mangrove, coral reef.'
					})}
					{@render field(
						'site.learners',
						'Learners',
						form.site.learners,
						(v) => (form.site.learners = v),
						{
							area: true,
							hint: 'Who would take part: ages, grades, community members.'
						}
					)}
					{@render field(
						'site.studyGoals',
						'What the classroom wants to study',
						form.site.studyGoals,
						(v) => (form.site.studyGoals = v),
						{ area: true }
					)}
				</div>
			{:else}
				<h2 class="flex items-center gap-2 text-lg font-semibold">
					<Icon name={STEP_INFO.review.icon} class="size-5 text-muted" /> Check and send
				</h2>
				<dl class="mt-4 grid gap-3 text-sm sm:grid-cols-[12rem_1fr]">
					<dt class="text-muted">Institution</dt>
					<dd class="text-ink">{form.institutionName}</dd>
					<dt class="text-muted">Contact</dt>
					<dd class="text-ink">
						{form.contactName}, {form.contactRole} · {form.contactEmail}{form.contactPhone
							? ` · ${form.contactPhone}`
							: ''}
					</dd>
					{#each data.questions as q (q.id)}
						<dt class="text-muted">{q.label}</dt>
						<dd class="text-ink">{answerLabel(q.id)}</dd>
					{/each}
					<dt class="text-muted">Site</dt>
					<dd class="text-ink">{form.site.placeName}, {form.site.country}</dd>
					<dt class="text-muted">Ecosystems</dt>
					<dd class="text-ink">{parseEcosystems(ecosystems).join(', ')}</dd>
					<dt class="text-muted">Learners</dt>
					<dd class="text-ink">{form.site.learners}</dd>
					<dt class="text-muted">Wants to study</dt>
					<dd class="text-ink">{form.site.studyGoals}</dd>
				</dl>
				{#if formError}<p class="field-error mt-3" role="alert">{formError}</p>{/if}
			{/if}
		</Wizard>
	</div>
</PublicShell>
