<script lang="ts">
	import { invalidate } from '$app/navigation';
	import type { InstitutionInfo } from '#lib/domain/types.ts';
	import { validateInstitutionPhoto } from '#lib/domain/validation.ts';
	import { scaledImage } from '#lib/utils/image.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';

	/**
	 * The host institution's photo (AP-43), as the institution overview shows it: pick a new one,
	 * say what it shows, or remove it. For the institution head, in the classroom's Settings.
	 */
	let {
		classroomId,
		institution
	}: { classroomId: string; institution: Pick<InstitutionInfo, 'photoUrl' | 'photoAlt'> } =
		$props();

	// svelte-ignore state_referenced_locally
	let image = $state<string | null>(institution.photoUrl);
	// svelte-ignore state_referenced_locally
	let alt = $state(institution.photoAlt);
	let errors = $state<Record<string, string>>({});
	let error = $state('');
	let saving = $state(false);
	let saved = $state(false);
	let fileInput = $state<HTMLInputElement>();

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
		await send({ image, alt });
	}

	async function remove() {
		image = null;
		alt = '';
		if (fileInput) fileInput.value = '';
		await send({ image: null, alt: '' });
	}

	async function send(input: { image: string | null; alt: string }) {
		errors = validateInstitutionPhoto(input);
		error = '';
		saved = false;
		if (Object.keys(errors).length) return;
		saving = true;
		try {
			await services.classrooms.setInstitutionPhoto(classroomId, input);
			saved = true;
			// The overview and About read the photo from the classroom's public profile.
			await invalidate('app:classroom');
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			error = errorMessage(e);
		} finally {
			saving = false;
		}
	}
</script>

<form class="card max-w-2xl space-y-4" onsubmit={save} novalidate>
	<div>
		<h2 class="text-lg font-semibold">Institution photo</h2>
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
	{#if error}<p class="field-error" role="alert">{error}</p>{/if}
	<div class="flex flex-wrap items-center gap-3">
		<button class="btn btn-primary" disabled={saving || !image}>
			{saving ? 'Saving…' : 'Save photo'}
		</button>
		{#if institution.photoUrl}
			<button type="button" class="btn btn-ghost" disabled={saving} onclick={remove}
				>Remove photo</button
			>
		{/if}
		{#if saved}<p class="text-sm text-muted" role="status">Saved.</p>{/if}
	</div>
</form>
