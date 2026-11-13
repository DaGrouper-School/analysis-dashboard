<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import Select, { type SelectOption } from '#lib/components/Select.svelte';
	import type { DomainDefinition, Parameter } from '#lib/domain/types.ts';
	import { blankRow, fillRow, NEW_DOMAIN, VALUE_KINDS, type ValueRow } from './type-form.ts';

	/** The measurement type form's second step: the values a type is expected to send. */
	let {
		rows = $bindable(),
		errors,
		parameters,
		domainChoices,
		pickers,
		onnewdomain
	}: {
		rows: ValueRow[];
		errors: Record<string, string>;
		parameters: Map<string, Parameter>;
		domainChoices: SelectOption[];
		/** Bumped when the domain dialog closes, so a picker left on "New domain…" resets. */
		pickers: number;
		/** Opens the domain dialog; `then` gets the new domain once it is added. */
		onnewdomain: (then: (domain: DomainDefinition) => unknown) => void;
	} = $props();

	function rowDomain(row: ValueRow, value: string) {
		if (value === NEW_DOMAIN) {
			onnewdomain((domain) => (row.domain = domain.id));
			return;
		}
		row.domain = value || null;
	}
</script>

<h4 class="text-sm font-semibold text-ink">Expected values</h4>
<p class="text-xs text-muted">
	Use an ID from the registry to share an existing value, or a new ID to add one. Station values
	can't be edited here. When this device is picked on a classroom page, "Tile" values show in
	"What's happening now?" and "Card" numbers are charted under it, in this order.
</p>
{#if errors.values}<p class="field-error">{errors.values}</p>{/if}
<datalist id="parameter-ids">
	{#each [...parameters.values()] as parameter (parameter.id)}
		<option value={parameter.id}>{parameter.label}</option>
	{/each}
</datalist>
{#key pickers}
	<ol class="mt-3 space-y-3">
		{#each rows as row, i (row.key)}
			{@const existing = parameters.get(row.id.trim())}
			{@const locked = existing && existing.source !== 'device'}
			<li
				class="grid gap-2 rounded-md border border-line p-3 sm:grid-cols-[1.2fr_1.2fr_5rem_6.5rem_1.2fr_auto]"
			>
				<div>
					<label class="label text-xs" for="row-{row.key}-id">ID</label>
					<input
						id="row-{row.key}-id"
						class={['input font-mono text-sm', errors[`values.${i}.id`] && 'input-error']}
						list="parameter-ids"
						placeholder="soil_ph"
						value={row.id}
						oninput={(e) => fillRow(row, e.currentTarget.value, parameters)}
					/>
				</div>
				<div>
					<label class="label text-xs" for="row-{row.key}-label">Label</label>
					<input
						id="row-{row.key}-label"
						class={['input text-sm', errors[`values.${i}.label`] && 'input-error']}
						placeholder="Soil pH"
						disabled={locked}
						bind:value={row.label}
					/>
				</div>
				<div>
					<label class="label text-xs" for="row-{row.key}-unit">Unit</label>
					<input
						id="row-{row.key}-unit"
						class="input text-sm"
						placeholder="%"
						disabled={locked}
						bind:value={row.unit}
					/>
				</div>
				<div>
					<label class="label text-xs" for="row-{row.key}-kind">Type</label>
					<Select
						id="row-{row.key}-kind"
						class="text-sm"
						disabled={locked}
						options={VALUE_KINDS}
						bind:value={row.valueType}
					/>
				</div>
				<div>
					<label class="label text-xs" for="row-{row.key}-domain">Domain</label>
					<Select
						id="row-{row.key}-domain"
						class="text-sm"
						options={domainChoices}
						value={row.domain ?? ''}
						onchange={(value) => rowDomain(row, value)}
					/>
				</div>
				<div class="flex items-end gap-2">
					<label
						class="flex h-10 items-center gap-1.5 text-xs whitespace-nowrap text-muted"
						title="A tile in &quot;What's happening now?&quot; when this device is picked"
					>
						<input type="checkbox" bind:checked={row.tile} />
						Tile
					</label>
					<label
						class="flex h-10 items-center gap-1.5 text-xs whitespace-nowrap text-muted"
						title="A card under &quot;What's happening now?&quot; when this device is picked"
					>
						<input type="checkbox" bind:checked={row.card} />
						Card
					</label>
					<button
						type="button"
						class="btn btn-ghost px-3"
						aria-label="Remove value {i + 1}"
						onclick={() => (rows = rows.filter((r) => r !== row))}
						><Icon name="close" class="size-4" /></button
					>
				</div>
				{#if errors[`values.${i}.id`] || errors[`values.${i}.label`]}
					<p class="field-error sm:col-span-6">
						{errors[`values.${i}.id`] ?? errors[`values.${i}.label`]}
					</p>
				{:else if existing}
					<p class="text-xs text-muted sm:col-span-6">
						{locked
							? 'Shared station value: the registry keeps its label and unit.'
							: 'Already in the registry: this type shares it.'}
						A new domain applies everywhere this value is used.
					</p>
				{/if}
			</li>
		{/each}
	</ol>
{/key}
<button type="button" class="btn btn-ghost mt-3" onclick={() => (rows = [...rows, blankRow()])}
	><Icon name="plus" class="size-4" />Add a value</button
>
