<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import CopyButton from '#lib/components/CopyButton.svelte';
	import HostAgreementCard from '#lib/features/classroom/HostAgreementCard.svelte';
	import InstitutionPhotoForm from '#lib/features/classroom/InstitutionPhotoForm.svelte';
	import SharingSettings from '#lib/features/classroom/SharingSettings.svelte';
	import type { ClassroomDetail, InstitutionInfo } from '#lib/domain/types.ts';
	import { validateClassroomUpdate } from '#lib/domain/validation.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';

	/**
	 * A classroom's settings (AP-45): its name, institution and the institution's photo, for those
	 * who may edit them, how it shares its data and what its facilitators may do, and its permanent
	 * code, for those who may see it. A dashboard tab.
	 */
	let {
		detail,
		institutionInfo = null
	}: { detail: ClassroomDetail; institutionInfo?: InstitutionInfo | null } = $props();

	const classroom = $derived(detail.classroom);
	const permissions = $derived(detail.permissions);

	// svelte-ignore state_referenced_locally
	let name = $state(detail.classroom.name);
	// svelte-ignore state_referenced_locally
	let institution = $state(detail.classroom.institutionName);
	let errors = $state<Record<string, string>>({});
	let error = $state('');
	let saving = $state(false);
	let saved = $state(false);

	async function save(event: SubmitEvent) {
		event.preventDefault();
		const input = { name, institutionName: institution };
		errors = validateClassroomUpdate(input);
		error = '';
		saved = false;
		if (Object.keys(errors).length) return;
		saving = true;
		try {
			await services.classrooms.update(classroom.id, input);
			saved = true;
			await invalidate('app:classroom');
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			error = errorMessage(e);
		} finally {
			saving = false;
		}
	}
</script>

<div class="space-y-5">
	{#if permissions.edit}
		<form class="card max-w-2xl space-y-4" onsubmit={save} novalidate>
			<h2 class="text-lg font-semibold">Classroom details</h2>
			<div>
				<label class="label" for="settings-name">Classroom name</label>
				<input
					id="settings-name"
					class={['input', errors.name && 'input-error']}
					bind:value={name}
				/>
				{#if errors.name}<p class="field-error">{errors.name}</p>{/if}
			</div>
			<div>
				<label class="label" for="settings-institution">Institution</label>
				<input
					id="settings-institution"
					class={['input', errors.institutionName && 'input-error']}
					bind:value={institution}
				/>
				{#if errors.institutionName}<p class="field-error">{errors.institutionName}</p>{/if}
			</div>
			{#if error}<p class="field-error" role="alert">{error}</p>{/if}
			<div class="flex items-center gap-3">
				<button class="btn btn-primary" disabled={saving}>
					{saving ? 'Saving…' : 'Save changes'}
				</button>
				{#if saved}<p class="text-sm text-muted" role="status">Saved.</p>{/if}
			</div>
		</form>
		{#if institutionInfo}
			<InstitutionPhotoForm classroomId={classroom.id} institution={institutionInfo} />
		{/if}
	{/if}

	{#if permissions.setUpInstitution}
		<section class="card flex max-w-2xl flex-wrap items-center justify-between gap-4">
			<div>
				<h2 class="text-lg font-semibold">Institution overview</h2>
				<p class="mt-1 text-sm text-muted">
					The photo, story, ecosystems and links members and visitors see.
				</p>
			</div>
			<a href={resolve('/(app)/classrooms/[id]/setup', { id: classroom.id })} class="btn btn-ghost">
				{detail.setup?.institutionDone ? 'Edit overview' : 'Continue setup'}
			</a>
		</section>
	{/if}

	{#if detail.setup}
		<HostAgreementCard
			classroomId={classroom.id}
			status={detail.setup}
			editable={permissions.markHostAgreement}
		/>
	{/if}

	{#if detail.sharing}
		<!-- Keyed by classroom, so switching classrooms starts from the next one's settings. -->
		{#key classroom.id}
			<SharingSettings
				classroomId={classroom.id}
				initial={detail.sharing}
				editable={permissions.manageSharing}
			/>
		{/key}
	{/if}

	{#if permissions.seeCode && classroom.code}
		<section
			class="card flex flex-wrap items-center justify-between gap-4 border-teal/25 bg-forest text-ink"
		>
			<div>
				<p class="text-sm text-muted">Global classroom code</p>
				<p class="mt-1 font-mono text-3xl tracking-wider text-lime">{classroom.code}</p>
				<p class="mt-2 max-w-xl text-sm text-muted">
					Registered in SAM as this classroom's permanent source identifier; it can't be changed.
					Only facilitators and the institution head can see it.
				</p>
			</div>
			<CopyButton text={classroom.code} label="Copy code" />
		</section>
	{/if}
</div>
