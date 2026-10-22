<script lang="ts">
	import Modal from '#lib/components/Modal.svelte';
	import type { Calculation, Parameter } from '#lib/domain/types.ts';
	import { PLAIN_MAX, checkExplanation } from '#lib/domain/validation.ts';
	import { SOURCE_CALCULATION } from '#lib/data/seed.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';

	/**
	 * A platform admin sets what a value means and how it is calculated (AP-07): the plain
	 * sentence under its chart, and the method, formula and inputs behind "How is this
	 * calculated?". Inputs are picked by searching, a few matches at a time.
	 */
	let {
		open = $bindable(false),
		parameter,
		parameters,
		onsaved
	}: {
		open?: boolean;
		parameter: Parameter;
		parameters: Parameter[];
		onsaved: () => unknown;
	} = $props();

	const METHODS: [Calculation['method'], string][] = [
		['measured', 'Measured'],
		['derived', 'Calculated from other values'],
		['manual', 'Entered by hand']
	];

	let plain = $state('');
	let method = $state<Calculation['method']>('measured');
	let formula = $state('');
	let inputs = $state<string[]>([]);
	let find = $state('');
	let errors = $state<Record<string, string>>({});
	let error = $state('');
	let saving = $state(false);

	// Fill the form each time it opens, from what the registry has now.
	$effect(() => {
		if (!open) return;
		const calculation = parameter.calculation ?? {
			...SOURCE_CALCULATION[parameter.source],
			inputs: []
		};
		plain = parameter.plain ?? '';
		method = calculation.method;
		formula = calculation.formula;
		inputs = [...calculation.inputs];
		find = '';
		errors = {};
		error = '';
	});

	const labelOf = (id: string) => parameters.find((p) => p.id === id)?.label ?? id;
	const matches = $derived.by(() => {
		const q = find.trim().toLowerCase();
		if (!q) return [];
		return parameters
			.filter(
				(p) =>
					p.id !== parameter.id &&
					!inputs.includes(p.id) &&
					(p.label.toLowerCase().includes(q) || p.id.includes(q))
			)
			.slice(0, 6);
	});

	async function save(event: SubmitEvent) {
		event.preventDefault();
		const input = { plain, method, formula, inputs };
		errors = checkExplanation(input, parameter.id, parameters);
		if (Object.keys(errors).length) return;
		saving = true;
		error = '';
		try {
			await services.registry.explainParameter(parameter.id, input);
			await onsaved();
			open = false;
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			error = errorMessage(e);
		} finally {
			saving = false;
		}
	}
</script>

<Modal bind:open title="What it means and how it is calculated" eyebrow={parameter.label}>
	<form id="explanation" class="space-y-5" onsubmit={save} novalidate>
		<div>
			<label class="label" for="explain-plain">What it means</label>
			<textarea
				id="explain-plain"
				class={['input min-h-20', errors.plain && 'input-error']}
				maxlength={PLAIN_MAX}
				placeholder="How warm the air is in the shade, measured about 2 m above the ground."
				bind:value={plain}></textarea>
			{#if errors.plain}
				<p class="field-error">{errors.plain}</p>
			{:else}
				<p class="mt-1.5 text-xs text-muted">
					One plain sentence on what is measured. No advice, and no cause and effect.
				</p>
			{/if}
		</div>

		<fieldset>
			<legend class="label">Method</legend>
			<div class="flex flex-wrap gap-2">
				{#each METHODS as [option, label] (option)}
					<label
						class={[
							'flex items-center gap-2 rounded-md border px-3 py-2 text-sm transition',
							method === option
								? 'border-ink bg-ink/[0.05]'
								: 'border-line hover:border-line-strong'
						]}
					>
						<input
							type="radio"
							name="explain-method"
							class="border-line-strong bg-transparent text-teal checked:bg-current focus:ring-teal"
							value={option}
							bind:group={method}
						/>
						{label}
					</label>
				{/each}
			</div>
		</fieldset>

		<div>
			<label class="label" for="explain-formula"
				>{method === 'derived' ? 'Formula' : 'How it is taken'}</label
			>
			<textarea
				id="explain-formula"
				class={[
					'input min-h-20',
					method === 'derived' && 'font-mono text-xs',
					errors.formula && 'input-error'
				]}
				bind:value={formula}></textarea>
			{#if errors.formula}<p class="field-error">{errors.formula}</p>{/if}
		</div>

		<div>
			<label class="label" for="explain-find">Inputs</label>
			{#if inputs.length}
				<ul class="mb-2 flex flex-wrap gap-2" aria-label="Inputs">
					{#each inputs as id (id)}
						<li class="chip gap-1.5">
							{labelOf(id)}
							<button
								type="button"
								class="text-muted hover:text-ink"
								aria-label="Remove {labelOf(id)}"
								onclick={() => (inputs = inputs.filter((i) => i !== id))}>×</button
							>
						</li>
					{/each}
				</ul>
			{/if}
			<input
				id="explain-find"
				type="search"
				class={['input', errors.inputs && 'input-error']}
				placeholder="Find a value to add"
				autocomplete="off"
				bind:value={find}
			/>
			{#if matches.length}
				<ul class="mt-2 flex flex-wrap gap-2" aria-label="Matching values">
					{#each matches as option (option.id)}
						<li>
							<button
								type="button"
								class="chip hover:border-line-strong"
								onclick={() => {
									inputs = [...inputs, option.id];
									find = '';
								}}>+ {option.label}</button
							>
						</li>
					{/each}
				</ul>
			{/if}
			{#if errors.inputs}
				<p class="field-error">{errors.inputs}</p>
			{:else}
				<p class="mt-1.5 text-xs text-muted">The values the formula uses, if any.</p>
			{/if}
		</div>
		{#if error}<p class="field-error" role="alert">{error}</p>{/if}
	</form>

	{#snippet footer()}
		<button type="button" class="btn btn-ghost" onclick={() => (open = false)}>Cancel</button>
		<button form="explanation" class="btn btn-primary" disabled={saving}>
			{saving ? 'Saving…' : 'Save'}
		</button>
	{/snippet}
</Modal>
