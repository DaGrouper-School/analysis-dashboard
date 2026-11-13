<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import { invalidate } from '$app/navigation';
	import type { InstitutionInfo } from '#lib/domain/types.ts';
	import {
		MAX_DESCRIPTION_LENGTH,
		validateClassroomUpdate,
		validateInstitutionPhoto
	} from '#lib/domain/validation.ts';
	import { scaledImage } from '#lib/utils/image.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';

	/**
	 * The host institution as its overview card shows it (AP-43), in one card: its photo and what
	 * the photo shows, its name and its description. For the institution head, in the classroom's
	 * Settings. One Save stores the name and description, and the photo when it changed.
	 */
	let {
		classroomId,
		classroomName,
		institutionName,
		institution
	}: {
		classroomId: string;
		/** Sent unchanged with the institution, since one update carries both names. */
		classroomName: string;
		institutionName: string;
		institution: Pick<InstitutionInfo, 'photoUrl' | 'photoAlt' | 'history'>;
	} = $props();

	// svelte-ignore state_referenced_locally
	let image = $state<string | null>(institution.photoUrl);
	// svelte-ignore state_referenced_locally
	let alt = $state(institution.photoAlt);
	// svelte-ignore state_referenced_locally
	let name = $state(institutionName);
	// svelte-ignore state_referenced_locally
	let history = $state(institution.history);
	let errors = $state<Record<string, string>>({});
	let error = $state('');
	let saving = $state(false);
	let saved = $state(false);
	let fileInput = $state<HTMLInputElement>();

	const photoChanged = $derived(image !== institution.photoUrl || alt !== institution.photoAlt);

	async function pick(event: Event) {
		const file = (event.currentTarget as HTMLInputElement).files?.[0];
		if (!file) return;
		errors = {};
		saved = false;
		try {
			image = await scaledImage(file);
		} catch {
			errors = { image: 'That file could not be read as an image.' };
		}
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		const update = { name: classroomName, institutionName: name, history };
		const photo = { image, alt };
		errors = {
			...validateClassroomUpdate(update),
			...(photoChanged ? validateInstitutionPhoto(photo) : {})
		};
		await run(async () => {
			await services.classrooms.update(classroomId, update);
			if (photoChanged) await services.classrooms.setInstitutionPhoto(classroomId, photo);
		});
	}

	async function removePhoto() {
		image = null;
		alt = '';
		if (fileInput) fileInput.value = '';
		await run(() => services.classrooms.setInstitutionPhoto(classroomId, { image: null, alt: '' }));
	}

	async function run(task: () => Promise<unknown>) {
		error = '';
		saved = false;
		if (Object.keys(errors).length) return;
		saving = true;
		try {
			await task();
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			error = errorMessage(e);
			return;
		} finally {
			saving = false;
		}
		saved = true;
		// The overview and About read the institution from the classroom's public profile.
		await invalidate('app:classroom');
	}
</script>

<form class="card max-w-2xl space-y-4" onsubmit={save} novalidate>
	<div>
		<h2 class="text-lg font-semibold">Institution</h2>
		<p class="mt-1 text-sm text-muted">Shown in the institution overview and on the public site.</p>
	</div>
	<div class="aspect-[16/9] overflow-hidden rounded-md border border-line bg-raised">
		{#if image}
			<img src={image} alt={alt || 'The new photo'} class="size-full object-cover" />
		{:else}
			<p class="grid size-full place-items-center text-sm text-muted">No photo yet</p>
		{/if}
	</div>
	<div>
		<label class="label" for="institution-photo">Choose a photo</label>
		<input
			bind:this={fileInput}
			id="institution-photo"
			type="file"
			accept="image/png,image/jpeg,image/webp"
			class="block w-full text-sm text-text file:mr-3 file:rounded-sm file:border file:border-line-strong file:bg-transparent file:px-3 file:py-2 file:text-ink"
			onchange={pick}
		/>
		{#if errors.image}<p class="field-error">{errors.image}</p>{/if}
	</div>
	<div>
		<label class="label" for="institution-photo-alt">What the photo shows</label>
		<input
			id="institution-photo-alt"
			class={['input', errors.alt && 'input-error']}
			bind:value={alt}
			placeholder="The school buildings and the garden"
		/>
		{#if errors.alt}<p class="field-error">{errors.alt}</p>{/if}
	</div>
	<div>
		<label class="label" for="institution-name">Institution name</label>
		<input
			id="institution-name"
			class={['input', errors.institutionName && 'input-error']}
			bind:value={name}
		/>
		{#if errors.institutionName}<p class="field-error">{errors.institutionName}</p>{/if}
	</div>
	<div>
		<label class="label" for="institution-description">Description</label>
		<textarea
			id="institution-description"
			class={['input min-h-32', errors.history && 'input-error']}
			bind:value={history}
			maxlength={MAX_DESCRIPTION_LENGTH}
			placeholder="Who you are and what your community measures here"></textarea>
		<p class="mt-1 text-xs text-muted">{history.length} / {MAX_DESCRIPTION_LENGTH}</p>
		{#if errors.history}<p class="field-error">{errors.history}</p>{/if}
	</div>
	{#if error}<p class="field-error" role="alert">{error}</p>{/if}
	<div class="flex flex-wrap items-center gap-3">
		<button class="btn btn-primary" disabled={saving}>
			<Icon name="check" class="size-4" />
			{saving ? 'Saving…' : 'Save institution'}
		</button>
		{#if institution.photoUrl}
			<button type="button" class="btn btn-ghost" disabled={saving} onclick={removePhoto}
				><Icon name="trash" class="size-4" /> Remove photo</button
			>
		{/if}
		{#if saved}<p class="text-sm text-muted" role="status">Saved.</p>{/if}
	</div>
</form>
