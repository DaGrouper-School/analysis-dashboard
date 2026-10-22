<script lang="ts">
	import type {
		AlertDirection,
		AlertLevel,
		AlertRule,
		AlertRuleInput,
		Parameter
	} from '#lib/domain/types.ts';
	import {
		ALERT_LEVELS,
		ALERT_DIRECTIONS,
		ALERT_TITLE_MAX,
		DIRECTION_LABELS,
		LEVEL_LABELS,
		checkAlertRule
	} from '#lib/domain/alerts.ts';
	import Modal from '#lib/components/Modal.svelte';
	import { rangeText, readEntries, unitsFor } from '#lib/observations.ts';
	import { errorMessage, ServiceError } from '#lib/services/index.ts';

	/**
	 * Adds an alert level on one value, or changes one (AP-08): on a device, on every sensor of
	 * a type in a classroom, or as a platform default. With more than one `scopes`, the person
	 * picks where it applies. The value can be typed in any of the parameter's units and gets the
	 * same checks as a manual observation, wrong-unit hint included.
	 */
	let {
		open = $bindable(false),
		parameter,
		rule = null,
		scopes = [],
		initialScope = '',
		note,
		save: send
	}: {
		open?: boolean;
		parameter: Parameter;
		/** The rule to change, or null to add one. */
		rule?: AlertRule | null;
		/** Where the level can apply, when there is a choice. */
		scopes?: { id: string; label: string; hint: string }[];
		initialScope?: string;
		/** A line on what saving to `scope` does to the levels below it, if anything. */
		note?: (scope: string) => string | null;
		/** Saves the level. Resolves once the page shows it, so nothing flickers back. */
		save: (scope: string, input: AlertRuleInput) => Promise<unknown>;
	} = $props();

	let scope = $state('');
	let direction = $state<AlertDirection>('above');
	let level = $state<AlertLevel>('caution');
	let value = $state('');
	let unit = $state('');
	let title = $state('');
	let errors = $state<Record<string, string>>({});
	let error = $state('');
	let saving = $state(false);

	// Fill the form each time it opens, from the rule being changed or blank.
	$effect(() => {
		if (!open) return;
		scope = initialScope;
		direction = rule?.direction ?? 'above';
		level = rule?.level ?? 'caution';
		value = rule ? String(rule.value) : '';
		unit = parameter.unit;
		title = rule?.title ?? '';
		errors = {};
		error = '';
	});

	async function save(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		const parameterId = parameter.id;
		const read = readEntries(
			{
				id: 'alert',
				label: 'Alert',
				description: '',
				domain: null,
				deviceType: null,
				fields: [{ parameterId, required: true }]
			},
			[parameter],
			{ [parameterId]: { value, unit } }
		);
		const input = {
			parameterId,
			direction,
			level,
			value: read.values[parameterId] as number,
			title,
			enabled: rule?.enabled ?? true
		};
		errors = {
			...checkAlertRule(input, [parameter]),
			...(read.errors[parameterId] ? { value: read.errors[parameterId] } : {})
		};
		if (Object.keys(errors).length) return;
		saving = true;
		try {
			await send(scope, input);
			open = false;
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			error = errorMessage(e);
		} finally {
			saving = false;
		}
	}
</script>

<Modal bind:open title={rule ? 'Change alert' : 'Add alert'} eyebrow={parameter.label}>
	<form id="alert-rule" class="space-y-5" onsubmit={save} novalidate>
		{#if scopes.length > 1}
			<fieldset>
				<legend class="label">Applies to</legend>
				<div class="space-y-2">
					{#each scopes as option (option.id)}
						<label
							class={[
								'flex cursor-pointer items-start gap-3 rounded-md border px-3 py-2 text-sm transition',
								scope === option.id
									? 'border-ink bg-ink/[0.05]'
									: 'border-line hover:border-line-strong'
							]}
						>
							<input
								type="radio"
								name="alert-scope"
								class="mt-0.5 border-line-strong bg-transparent text-teal checked:bg-current focus:ring-teal"
								value={option.id}
								bind:group={scope}
							/>
							<span>
								<span class="block text-ink">{option.label}</span>
								<span class="block text-xs text-muted">{option.hint}</span>
							</span>
						</label>
					{/each}
				</div>
			</fieldset>
		{/if}

		<fieldset>
			<legend class="label">Level</legend>
			<div class="flex flex-wrap gap-2">
				{#each ALERT_LEVELS as option (option)}
					<label
						class={[
							'flex cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm transition',
							level === option ? 'border-ink bg-ink/[0.05]' : 'border-line hover:border-line-strong'
						]}
					>
						<input
							type="radio"
							name="alert-level"
							class={[
								'border-line-strong bg-transparent checked:bg-current focus:ring-teal',
								option === 'warning' ? 'text-danger' : 'text-orange'
							]}
							value={option}
							bind:group={level}
						/>
						{LEVEL_LABELS[option]}
					</label>
				{/each}
			</div>
		</fieldset>

		<div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
			<div>
				<label class="label" for="alert-direction">When the value is</label>
				<select id="alert-direction" class="input" bind:value={direction}>
					{#each ALERT_DIRECTIONS as option (option)}
						<option value={option}>{DIRECTION_LABELS[option]}</option>
					{/each}
				</select>
			</div>
			<div>
				<label class="label" for="alert-value">Threshold</label>
				<input
					id="alert-value"
					class={['input', errors.value && 'input-error']}
					inputmode="decimal"
					bind:value
				/>
			</div>
			{#if unitsFor(parameter).length > 1}
				<div>
					<label class="label" for="alert-unit">Unit</label>
					<select id="alert-unit" class="input w-auto pr-9" bind:value={unit}>
						{#each unitsFor(parameter) as option (option)}
							<option value={option}>{option}</option>
						{/each}
					</select>
				</div>
			{:else if parameter.unit}
				<p class="self-end pb-2.5 text-sm text-muted">{parameter.unit}</p>
			{/if}
		</div>
		{#if errors.value}
			<p class="field-error -mt-3">{errors.value}</p>
		{:else if rangeText(parameter)}
			<p class="-mt-3 text-xs text-muted">Expected range {rangeText(parameter)}.</p>
		{/if}

		<div>
			<label class="label" for="alert-title">Short name</label>
			<input
				id="alert-title"
				class={['input', errors.title && 'input-error']}
				maxlength={ALERT_TITLE_MAX}
				placeholder="Very high UV"
				bind:value={title}
			/>
			{#if errors.title}
				<p class="field-error">{errors.title}</p>
			{:else}
				<p class="mt-1.5 text-xs text-muted">
					Shown on the alert. Say what was measured; the dashboard adds the reading and level.
				</p>
			{/if}
		</div>

		{#if note?.(scope)}
			<p class="rounded-md border border-line p-3 text-sm text-muted">{note(scope)}</p>
		{/if}
		{#if error}<p class="field-error" role="alert">{error}</p>{/if}
	</form>

	{#snippet footer()}
		<button type="button" class="btn btn-ghost" onclick={() => (open = false)}>Cancel</button>
		<button form="alert-rule" class="btn btn-primary" disabled={saving}>
			{saving ? 'Saving…' : rule ? 'Save changes' : 'Add alert'}
		</button>
	{/snippet}
</Modal>
