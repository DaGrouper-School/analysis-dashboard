<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import LocationInput from '#lib/components/LocationInput.svelte';
	import Wizard from '#lib/components/Wizard.svelte';
	import type { CreateLabInput, SiteSetting } from '#lib/domain/types.ts';
	import { validateLab } from '#lib/domain/validation.ts';
	import { toGeoPoint } from '#lib/geo.ts';
	import { SITE_SETTING_LABELS } from '#lib/labels.ts';
	import { errorMessage, ServiceError } from '#lib/services/index.ts';

	/** The lab and its site profile, a step each. Used to create a lab and to edit one. */
	let {
		initial = null,
		submitLabel,
		savingLabel = 'Saving…',
		save
	}: {
		initial?: CreateLabInput | null;
		submitLabel: string;
		savingLabel?: string;
		/** Rejects with a `ServiceError` to show field errors. */
		save: (input: CreateLabInput) => Promise<void>;
	} = $props();

	// svelte-ignore state_referenced_locally
	const start = initial;
	let name = $state(start?.name ?? '');
	let placeName = $state(start?.placeName ?? '');
	let country = $state(start?.country ?? '');
	let lat = $state(start ? String(start.location.lat) : '');
	let lng = $state(start ? String(start.location.lng) : '');
	let setting = $state<SiteSetting>(start?.siteProfile.setting ?? 'rural');
	let description = $state(start?.siteProfile.description ?? '');
	let questions = $state(start?.siteProfile.researchQuestions.join('\n') ?? '');
	let errors = $state<Record<string, string>>({});
	let formError = $state('');
	let saving = $state(false);

	const STEPS = [
		{ label: 'The lab', icon: 'flask' },
		{ label: 'Site profile', icon: 'pin' },
		{ label: 'Review', icon: 'review' }
	];
	let step = $state(0);
	/** Site profile fields are on step two; the rest on step one. */
	const SITE_FIELDS = ['setting', 'description', 'researchQuestions'];
	const stepOf = (field: string) => (SITE_FIELDS.includes(field.split('.')[0]) ? 1 : 0);

	function input(): CreateLabInput {
		return {
			name,
			placeName,
			country,
			location: toGeoPoint(lat, lng),
			siteProfile: { setting, description, researchQuestions: questions.split('\n') }
		};
	}

	/** Shows the errors of `at` (all of them on review, which sends back to the first). */
	function check(at: number) {
		const all = validateLab(input());
		errors = Object.fromEntries(
			Object.entries(all).filter(([field]) => at === 2 || stepOf(field) === at)
		);
		const fields = Object.keys(errors);
		if (at === 2 && fields.length) step = Math.min(...fields.map(stepOf));
		return !fields.length;
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		formError = '';
		saving = true;
		try {
			await save(input());
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			const fields = Object.keys(errors);
			if (fields.length) step = Math.min(...fields.map(stepOf));
			formError = errorMessage(e);
		} finally {
			saving = false;
		}
	}
</script>

<div class="card mt-8 max-w-2xl">
	<Wizard
		steps={STEPS}
		bind:current={step}
		{check}
		onsubmit={submit}
		{submitLabel}
		busy={saving}
		busyLabel={savingLabel}
		label="Lab and site profile"
	>
		{#if step === 0}
			<section class="space-y-5">
				<h2 class="flex items-center gap-2 text-lg font-semibold">
					<Icon name={STEPS[0].icon} class="size-5 text-muted" /> The lab
				</h2>
				<div>
					<label class="label" for="name">Lab name</label>
					<input
						id="name"
						class={['input', errors.name && 'input-error']}
						placeholder="Trinity Lab"
						bind:value={name}
					/>
					{#if errors.name}<p class="field-error">{errors.name}</p>{/if}
				</div>
				<div class="grid grid-cols-2 gap-3">
					<div class="col-span-2 sm:col-span-1">
						<label class="label" for="placeName">Where it is</label>
						<input
							id="placeName"
							class={['input', errors.placeName && 'input-error']}
							placeholder="Trinity School, Addis Ababa"
							bind:value={placeName}
						/>
						{#if errors.placeName}<p class="field-error">{errors.placeName}</p>{/if}
					</div>
					<div class="col-span-2 sm:col-span-1">
						<label class="label" for="country">Country</label>
						<input
							id="country"
							class={['input', errors.country && 'input-error']}
							autocomplete="country-name"
							bind:value={country}
						/>
						{#if errors.country}<p class="field-error">{errors.country}</p>{/if}
					</div>
				</div>
				<div>
					<p class="label">Lab location</p>
					<p class="-mt-1 mb-2 text-xs text-muted">
						Readings from devices without GPS are shown here, marked as not from the device.
					</p>
					<LocationInput id="lab-location" bind:lat bind:lng error={errors.location} />
				</div>
			</section>
		{:else if step === 1}
			<section class="space-y-5">
				<h2 class="flex items-center gap-2 text-lg font-semibold">
					<Icon name={STEPS[1].icon} class="size-5 text-muted" /> Site profile
				</h2>
				<div>
					<label class="label" for="setting">Setting</label>
					<select id="setting" class="input" bind:value={setting}>
						{#each Object.entries(SITE_SETTING_LABELS) as [value, label] (value)}
							<option {value}>{label}</option>
						{/each}
					</select>
				</div>
				<div>
					<label class="label" for="description">Describe the site</label>
					<textarea
						id="description"
						rows="3"
						class={['input', errors.description && 'input-error']}
						placeholder="What the land, water and community around the lab are like."
						bind:value={description}></textarea>
					{#if errors.description}<p class="field-error">{errors.description}</p>{/if}
				</div>
				<div>
					<label class="label" for="questions">Local research questions</label>
					<textarea
						id="questions"
						rows="4"
						class={['input', errors.researchQuestions && 'input-error']}
						placeholder="One per line, e.g. When does the rainy season really start here?"
						bind:value={questions}></textarea>
					{#if errors.researchQuestions}<p class="field-error">{errors.researchQuestions}</p>{/if}
				</div>
			</section>
		{:else}
			<h2 class="flex items-center gap-2 text-lg font-semibold">
				<Icon name={STEPS[2].icon} class="size-5 text-muted" /> Check and save
			</h2>
			<dl class="mt-4 grid gap-3 text-sm sm:grid-cols-[9rem_1fr]">
				<dt class="text-muted">Lab</dt>
				<dd class="text-ink">{name}</dd>
				<dt class="text-muted">Where</dt>
				<dd class="text-ink">{placeName}, {country} · {lat}, {lng}</dd>
				<dt class="text-muted">Setting</dt>
				<dd class="text-ink">{SITE_SETTING_LABELS[setting]}</dd>
				<dt class="text-muted">Site</dt>
				<dd class="text-ink">{description}</dd>
				<dt class="text-muted">Research questions</dt>
				<dd>
					<ul class="list-disc space-y-1 pl-5 text-ink">
						{#each questions.split('\n').filter((q) => q.trim()) as question, i (i)}
							<li>{question}</li>
						{/each}
					</ul>
				</dd>
			</dl>
			{#if formError}<p class="field-error mt-3" role="alert">{formError}</p>{/if}
		{/if}
	</Wizard>
</div>
