<script lang="ts">
	import type {
		AlertDirection,
		AlertLevel,
		ClassroomAlerts,
		EffectiveAlertRule,
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
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';

	/**
	 * Adds an alert level to a classroom, or changes one (AP-08). The value can be typed in any
	 * of the parameter's units and gets the same checks as a manual observation, wrong-unit hint
	 * included. Editing a platform default saves it as the classroom's own.
	 */
	let {
		open = $bindable(false),
		classroomId,
		parameters,
		rule = null,
		onsaved
	}: {
		open?: boolean;
		classroomId: string;
		/** The numeric values the classroom's devices send, to pick from when adding. */
		parameters: Parameter[];
		/** The rule to change, or null to add one. */
		rule?: EffectiveAlertRule | null;
		onsaved: (alerts: ClassroomAlerts) => unknown;
	} = $props();

	let parameterId = $state('');
	let direction = $state<AlertDirection>('above');
	let level = $state<AlertLevel>('caution');
	let value = $state('');
	let unit = $state('');
	let title = $state('');
	let errors = $state<Record<string, string>>({});
	let error = $state('');
	let saving = $state(false);

	const parameter = $derived(parameters.find((p) => p.id === parameterId));

	// Fill the form each time it opens, from the rule being changed or blank.
	$effect(() => {
		if (!open) return;
		parameterId = rule?.parameterId ?? parameters[0]?.id ?? '';
		direction = rule?.direction ?? 'above';
		level = rule?.level ?? 'caution';
		value = rule ? String(rule.value) : '';
		unit = parameters.find((p) => p.id === parameterId)?.unit ?? '';
		title = rule?.title ?? '';
		errors = {};
		error = '';
	});

	function pickParameter(id: string) {
		parameterId = id;
		unit = parameters.find((p) => p.id === id)?.unit ?? '';
		errors = {};
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		if (!parameter) return;
		error = '';
		const field = { parameterId, required: true };
		const read = readEntries(
			{
				id: 'alert',
				label: 'Alert',
				description: '',
				domain: null,
				deviceType: null,
				fields: [field]
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
			const saved = await services.classrooms.saveAlert(classroomId, input, rule?.id);
			open = false;
			await onsaved(saved);
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			error = errorMessage(e);
		} finally {
			saving = false;
		}
	}
</script>

<Modal bind:open title={rule ? 'Change alert' : 'Add alert'} eyebrow="Classroom alerts">
	<form id="alert-rule" class="space-y-5" onsubmit={save} novalidate>
		<div>
			<label class="label" for="alert-parameter">Value</label>
			<select
				id="alert-parameter"
				class={['input', errors.parameterId && 'input-error']}
				value={parameterId}
				disabled={!!rule}
				onchange={(e) => pickParameter(e.currentTarget.value)}
			>
				{#each parameters as option (option.id)}
					<option value={option.id}>{option.label}</option>
				{/each}
			</select>
			{#if errors.parameterId}
				<p class="field-error">{errors.parameterId}</p>
			{:else if rule}
				<p class="mt-1.5 text-xs text-muted">An alert stays on its value.</p>
			{/if}
		</div>

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
			{#if parameter && unitsFor(parameter).length > 1}
				<div>
					<label class="label" for="alert-unit">Unit</label>
					<select id="alert-unit" class="input w-auto pr-9" bind:value={unit}>
						{#each unitsFor(parameter) as option (option)}
							<option value={option}>{option}</option>
						{/each}
					</select>
				</div>
			{:else if parameter?.unit}
				<p class="self-end pb-2.5 text-sm text-muted">{parameter.unit}</p>
			{/if}
		</div>
		{#if errors.value}
			<p class="field-error -mt-3">{errors.value}</p>
		{:else if parameter && rangeText(parameter)}
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

		{#if rule?.source === 'default'}
			<p class="rounded-md border border-line p-3 text-sm text-muted">
				This is a platform default. Saving makes the classroom's own copy of the {parameter?.label}
				levels; the platform defaults stay as they are.
			</p>
		{/if}
		{#if error}<p class="field-error" role="alert">{error}</p>{/if}
	</form>

	{#snippet footer()}
		<button type="button" class="btn btn-ghost" onclick={() => (open = false)}>Cancel</button>
		<button form="alert-rule" class="btn btn-primary" disabled={saving || !parameter}>
			{saving ? 'Saving…' : rule ? 'Save changes' : 'Add alert'}
		</button>
	{/snippet}
</Modal>
