<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import LocationInput from '#lib/components/LocationInput.svelte';
	import SetupSteps from '#lib/components/SetupSteps.svelte';
	import type { CreateLabInput, SiteSetting } from '#lib/domain/types.ts';
	import { validateLab } from '#lib/domain/validation.ts';
	import { toGeoPoint } from '#lib/geo.ts';
	import { SITE_SETTING_LABELS } from '#lib/labels.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let name = $state('');
	let placeName = $state('');
	let country = $state('');
	let lat = $state('');
	let lng = $state('');
	let setting = $state<SiteSetting>('rural');
	let description = $state('');
	let questions = $state('');
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
			const lab = await services.labs.create(data.classroom.id, input);
			await goto(
				data.onboarding
					? resolve('/(app)/labs/[labId]?onboarding', { labId: lab.id })
					: resolve('/(app)/labs/[labId]', { labId: lab.id })
			);
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			formError = errorMessage(e);
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head><title>Add a lab · DaGrouper Analysis</title></svelte:head>

<a
	href={resolve('/(app)/classrooms/[id]', { id: data.classroom.id })}
	class="text-sm text-muted hover:text-ink">← {data.classroom.name}</a
>
<div class="mt-4">
	{#if data.onboarding}<SetupSteps current={2} />{/if}
	<h1 class="font-display text-5xl">Register your lab</h1>
	<p class="mt-1 max-w-xl text-muted">
		The lab is where your classroom works with the Lab in a Bag. Next you'll register its devices,
		each with its own location.
	</p>
</div>

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
			{saving ? 'Saving…' : 'Save lab and add devices'}
		</button>
		{#if formError}<p class="field-error mt-0" role="alert">{formError}</p>{/if}
	</div>
</form>
