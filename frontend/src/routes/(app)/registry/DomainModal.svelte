<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import { invalidateAll } from '$app/navigation';
	import IconPicker from '#lib/components/IconPicker.svelte';
	import Modal from '#lib/components/Modal.svelte';
	import type { DomainDefinition, DomainInput } from '#lib/domain/types.ts';
	import { validateDomain } from '#lib/domain/validation.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';

	/**
	 * One dialog adds a domain (from the Add button or a value's domain picker) or edits one. Open
	 * it with `openNew` or `edit` through `bind:this`. `onstatus` reports what happened for the
	 * page's notice, and `onclose` runs whenever the dialog closes.
	 */
	let {
		onstatus,
		onclose
	}: {
		onstatus: (status: { notice: string; error: string }) => void;
		onclose: () => void;
	} = $props();

	const blankDomain = (): DomainInput => ({ id: '', label: '', icon: 'tag', description: '' });

	let open = $state(false);
	let editing = $state<string | null>(null);
	let form = $state(blankDomain());
	let errors = $state<Record<string, string>>({});
	let saving = $state(false);
	/** Set when a domain picker asked for the new domain: it is used there once added. */
	let afterCreate = $state<((domain: DomainDefinition) => unknown) | null>(null);

	export function openNew(then: ((domain: DomainDefinition) => unknown) | null = null) {
		afterCreate = then;
		editing = null;
		form = blankDomain();
		errors = {};
		open = true;
	}

	export function edit(domain: DomainDefinition) {
		afterCreate = null;
		editing = domain.id;
		form = { ...domain };
		errors = {};
		open = true;
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		errors = validateDomain(form);
		if (Object.keys(errors).length) return;
		saving = true;
		onstatus({ notice: '', error: '' });
		try {
			const { id, ...rest } = form;
			const domain = editing
				? await services.registry.updateDomain(id, rest)
				: await services.registry.createDomain(form);
			await invalidateAll();
			onstatus({
				notice: editing ? `Saved ${domain.label}.` : `Added the ${domain.label} domain.`,
				error: ''
			});
			const then = afterCreate;
			open = false;
			await then?.(domain);
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			if (!Object.keys(errors).length) onstatus({ notice: '', error: errorMessage(e) });
		} finally {
			saving = false;
		}
	}
</script>

<Modal
	bind:open
	eyebrow="Domains"
	title={editing ? `Edit ${form.label || 'domain'}` : 'New domain'}
	onclose={() => {
		afterCreate = null;
		onclose();
	}}
>
	<form id="domain-form" onsubmit={submit} novalidate>
		{#if afterCreate}
			<p class="mb-4 text-sm text-muted">
				It is added for every classroom, then used for this value.
			</p>
		{/if}
		<div class="grid gap-3 sm:grid-cols-[8rem_1fr]">
			<div>
				<label class="label" for="domain-code">Code</label>
				<input
					id="domain-code"
					class={['input font-mono uppercase', errors.id && 'input-error']}
					placeholder="SOIL"
					disabled={!!editing}
					bind:value={form.id}
				/>
			</div>
			<div>
				<label class="label" for="domain-label">Name</label>
				<input
					id="domain-label"
					class={['input', errors.label && 'input-error']}
					placeholder="Soil"
					bind:value={form.label}
				/>
			</div>
		</div>
		{#each ['id', 'label'] as field (field)}
			{#if errors[field]}<p class="field-error">{errors[field]}</p>{/if}
		{/each}
		<div class="mt-3">
			<label class="label" for="domain-description">Description</label>
			<input
				id="domain-description"
				class="input"
				placeholder="Moisture, temperature and chemistry of the ground."
				bind:value={form.description}
			/>
		</div>
		<div class="mt-3">
			<IconPicker name="domain-icon" bind:value={form.icon} error={errors.icon} />
		</div>
	</form>
	{#snippet footer()}
		<button type="button" class="btn btn-ghost" onclick={() => (open = false)}
			><Icon name="close" class="size-4" /> Cancel</button
		>
		<button form="domain-form" class="btn btn-primary" disabled={saving}>
			<Icon name="plus" class="size-4" />
			{saving ? 'Saving…' : editing ? 'Save domain' : afterCreate ? 'Add and use' : 'Add domain'}
		</button>
	{/snippet}
</Modal>
