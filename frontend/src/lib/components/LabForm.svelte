<script lang="ts">
	import LocationInput from '#lib/components/LocationInput.svelte';
	import type { CreateLabInput, SiteSetting } from '#lib/domain/types.ts';
	import { validateLab } from '#lib/domain/validation.ts';
	import { toGeoPoint } from '#lib/geo.ts';
	import { SITE_SETTING_LABELS } from '#lib/labels.ts';
	import { errorMessage, ServiceError } from '#lib/services/index.ts';

	/** The lab and its site profile. Used to create a lab and to edit one. */
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

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		formError = '';
		const input: CreateLabInput = {
			name,
			placeName,
			country,
			location: toGeoPoint(lat, lng),
			siteProfile: { setting, description, researchQuestions: questions.split('\n') }
		};
		errors = validateLab(input);
		if (Object.keys(errors).length) return;
		saving = true;
		try {
			await save(input);
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			formError = errorMessage(e);
		} finally {
			saving = false;
		}
	}
</script>

<form class="mt-8 grid max-w-4xl gap-6 lg:grid-cols-2" onsubmit={submit} novalidate>
	<section class="card space-y-5">
		<h2 class="text-lg font-semibold">The lab</h2>
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

	<section class="card space-y-5">
		<h2 class="text-lg font-semibold">Site profile</h2>
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

	<div class="flex items-center gap-4 lg:col-span-2">
		<button class="btn btn-primary" disabled={saving}>
			{saving ? savingLabel : submitLabel}
		</button>
		{#if formError}<p class="field-error mt-0" role="alert">{formError}</p>{/if}
	</div>
</form>
