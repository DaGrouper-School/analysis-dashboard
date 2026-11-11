<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import IconPicker from '#lib/components/IconPicker.svelte';
	import Modal from '#lib/components/Modal.svelte';
	import type { SelectOption } from '#lib/components/Select.svelte';
	import Wizard from '#lib/components/Wizard.svelte';
	import type { DeviceTypeDefinition, DomainDefinition, Parameter } from '#lib/domain/types.ts';
	import { validateDeviceType } from '#lib/domain/validation.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';
	import { DEVICE_TYPE_PALETTE } from '#lib/utils/labels.ts';
	import TypeReview from './TypeReview.svelte';
	import TypeValueRows from './TypeValueRows.svelte';
	import {
		blankRow,
		blankType,
		firstStepWithError,
		isFilled,
		TYPE_STEPS,
		typeForm,
		typeInput,
		typeStepOf
	} from './type-form.ts';

	/**
	 * Registers a measurement type or edits one, in three steps: basics, expected values, review.
	 * Open it with `openNew` or `edit` through `bind:this`. `onstatus` reports what happened for
	 * the page's notice.
	 */
	let {
		parameters,
		domains,
		domainChoices,
		pickers,
		onnewdomain,
		onstatus
	}: {
		parameters: Map<string, Parameter>;
		domains: Map<string, DomainDefinition>;
		domainChoices: SelectOption[];
		pickers: number;
		onnewdomain: (then: (domain: DomainDefinition) => unknown) => void;
		onstatus: (status: { notice: string; error: string }) => void;
	} = $props();

	let open = $state(false);
	let editing = $state<string | null>(null);
	let form = $state(blankType());
	let errors = $state<Record<string, string>>({});
	let saving = $state(false);
	let step = $state(0);
	const title = $derived(
		editing ? `Edit ${form.label || 'measurement type'}` : 'Register a measurement type'
	);

	function reset() {
		editing = null;
		form = blankType();
		errors = {};
		step = 0;
	}

	export function openNew() {
		reset();
		open = true;
	}

	export function edit(type: DeviceTypeDefinition) {
		editing = type.id;
		form = typeForm(type, parameters);
		errors = {};
		step = 0;
		open = true;
	}

	/** Shows the errors of `at` (all of them on review) and says whether it is clear. */
	function check(at: number) {
		// Past the basics, blank rows go first, so error positions match the rows left.
		if (at > 0) form.rows = form.rows.filter(isFilled);
		const all = validateDeviceType(typeInput(form));
		errors = Object.fromEntries(
			Object.entries(all).filter(([field]) => at === 2 || typeStepOf(field) === at)
		);
		if (at === 1 && !form.rows.length) form.rows = [blankRow()];
		// On review, send the person back to the first step with a problem.
		const first = firstStepWithError(errors);
		if (at === 2 && first !== null) step = first;
		return first === null;
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		form.rows = form.rows.filter(isFilled);
		const input = typeInput(form);
		saving = true;
		onstatus({ notice: '', error: '' });
		try {
			const type = editing
				? await services.registry.updateDeviceType(editing, input)
				: await services.registry.createDeviceType(input);
			await invalidateAll();
			onstatus({
				notice: editing
					? `Saved the ${type.label} measurement type.`
					: `Registered the ${type.label} measurement type. Every classroom can now add one.`,
				error: ''
			});
			reset();
			open = false;
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			const first = firstStepWithError(errors);
			if (first !== null) step = first;
			else onstatus({ notice: '', error: errorMessage(e) });
		} finally {
			saving = false;
		}
	}
</script>

<Modal bind:open wide eyebrow="Measurement types" {title}>
	<Wizard
		steps={TYPE_STEPS}
		bind:current={step}
		{check}
		onsubmit={save}
		submitLabel={editing ? 'Save measurement type' : 'Register measurement type'}
		busy={saving}
		label={title}
	>
		{#snippet extra()}
			<button type="button" class="btn btn-ghost" onclick={() => (open = false)}>Cancel</button>
		{/snippet}
		{#if step === 0}
			<div class="grid gap-3 sm:grid-cols-2">
				<div>
					<label class="label" for="type-label">Name</label>
					<input
						id="type-label"
						class={['input', errors.label && 'input-error']}
						placeholder="Soil probe"
						bind:value={form.label}
					/>
					{#if errors.label}<p class="field-error">{errors.label}</p>{/if}
				</div>
				<div>
					<label class="label" for="type-description">Description</label>
					<input
						id="type-description"
						class="input"
						placeholder="What it is and where it goes."
						bind:value={form.description}
					/>
				</div>
			</div>
			<label class="mt-3 flex items-start gap-3 rounded-md border border-line p-3 text-sm">
				<input
					type="checkbox"
					class="mt-0.5 rounded border-line-strong bg-transparent text-teal checked:bg-current focus:ring-teal"
					bind:checked={form.manual}
				/>
				<span>
					<span class="block text-ink">Manual measurement</span>
					<span class="block text-xs text-muted">
						Entered by hand, with no sensor: added to a lab without pairing, with a generated
						identifier, and recorded with “Enter measurement” instead of a sync.
					</span>
				</span>
			</label>
			<fieldset class="mt-3">
				<legend class="label">Color on the map</legend>
				<div class="flex flex-wrap gap-2">
					{#each DEVICE_TYPE_PALETTE as color (color)}
						<label class="cursor-pointer">
							<input
								type="radio"
								class="peer sr-only"
								name="type-color"
								value={color}
								bind:group={form.color}
							/>
							<span
								class="block size-7 rounded-full ring-offset-2 ring-offset-surface peer-checked:ring-2 peer-checked:ring-ink peer-focus-visible:ring-2 peer-focus-visible:ring-teal"
								style:background={color}
							></span>
							<span class="sr-only">{color}</span>
						</label>
					{/each}
				</div>
			</fieldset>
			<div class="mt-3">
				<IconPicker
					name="type-icon"
					legend="Icon"
					color={form.color}
					bind:value={form.icon}
					error={errors.icon}
				/>
			</div>
		{:else if step === 1}
			<TypeValueRows
				bind:rows={form.rows}
				{errors}
				{parameters}
				{domainChoices}
				{pickers}
				{onnewdomain}
			/>
		{:else}
			<TypeReview {form} {domains} />
		{/if}
	</Wizard>
</Modal>
