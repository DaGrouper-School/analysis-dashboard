<script lang="ts">
	import { invalidate } from '$app/navigation';
	import type { InstitutionInfo } from '#lib/domain/types.ts';
	import { validateInstitutionPhoto } from '#lib/domain/validation.ts';
	import PhotoPicker from '#lib/components/PhotoPicker.svelte';
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
	let picker = $state<PhotoPicker>();

	function picked(pickError: string) {
		saved = false;
		errors = pickError ? { image: pickError } : {};
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		await send({ image, alt });
	}

	async function remove() {
		image = null;
		alt = '';
		picker?.reset();
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
	<PhotoPicker
		bind:this={picker}
		id="institution-photo"
		label="Choose a photo"
		bind:image
		{alt}
		required
		error={errors.image}
		onpick={picked}
	/>
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
