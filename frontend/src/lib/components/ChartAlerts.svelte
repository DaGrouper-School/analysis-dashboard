<script lang="ts">
	import type {
		AlertRuleInput,
		AlertScope,
		DeviceAlerts,
		EffectiveAlertRule,
		Parameter
	} from '#lib/domain/types.ts';
	import { ruleText } from '#lib/domain/alerts.ts';
	import AlertRuleModal from '#lib/components/AlertRuleModal.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import { errorMessage, services } from '#lib/services/index.ts';

	/**
	 * The alert levels on one value of one device, under its chart (AP-08): where they come from,
	 * each level, and for heads and facilitators the controls to add, change, turn off, delete
	 * and reset them. `editable` false shows the levels only.
	 */
	let {
		alerts,
		parameter,
		typeName,
		editable = true,
		onchange
	}: {
		alerts: DeviceAlerts;
		parameter: Parameter;
		/** The device type's registry label, e.g. `Weather station`. */
		typeName: string;
		editable?: boolean;
		onchange?: () => unknown;
	} = $props();

	// `weather stations in Trinity Lab`: who a classroom-wide level applies to.
	const typeLabel = $derived(
		`${typeName.toLowerCase().replace(/s?$/, 's')} in ${alerts.classroomName}`
	);
	const rules = $derived(alerts.rules.filter((r) => r.parameterId === parameter.id));
	const source = $derived(rules[0]?.source ?? null);
	const latest = $derived(
		rules.filter((r) => r.updatedAt).toSorted((a, b) => b.updatedAt!.localeCompare(a.updatedAt!))[0]
	);
	const canEdit = $derived(editable && alerts.canEditDevice);
	const day = new Intl.DateTimeFormat(undefined, { day: 'numeric', month: 'short' });

	let open = $state(false);
	let editRule = $state<EffectiveAlertRule | null>(null);
	let error = $state('');

	/** The layer a change to `rule` lands in: the type's only for the head, else this device's. */
	function scopeFor(rule: EffectiveAlertRule) {
		return rule.source === 'type' && alerts.canEditType ? 'type' : 'device';
	}

	function edit(rule: EffectiveAlertRule | null) {
		editRule = rule;
		open = true;
	}

	async function run(action: () => Promise<unknown>) {
		error = '';
		try {
			await action();
			await onchange?.();
		} catch (e) {
			error = errorMessage(e);
		}
	}

	function toggle(rule: EffectiveAlertRule) {
		const { parameterId, direction, value, level, title } = rule;
		run(() =>
			services.labs.saveAlert(
				alerts.deviceId,
				scopeFor(rule),
				{ parameterId, direction, value, level, title, enabled: !rule.enabled },
				rule.id
			)
		);
	}

	function remove(rule: EffectiveAlertRule) {
		const last = rules.filter((r) => r.source === rule.source).length === 1;
		const then = last
			? ` It is the last ${parameter.label} level ${rule.source === 'device' ? 'on this device' : `for all ${typeLabel}`}, so the next levels down apply again.`
			: '';
		if (!confirm(`Delete “${rule.title}”?${then}`)) return;
		run(() => services.labs.deleteAlert(alerts.deviceId, rule.id));
	}

	function reset() {
		if (!source || source === 'default') return;
		const back =
			source === 'device'
				? `use the levels for all ${typeLabel}`
				: 'use the platform defaults for every measurement of this type';
		if (!confirm(`Drop these ${parameter.label} levels and ${back}?`)) return;
		run(() => services.labs.resetAlerts(alerts.deviceId, source, parameter.id));
	}

	const scopes = $derived(
		alerts.canEditType
			? [
					{
						id: 'device',
						label: 'This device only',
						hint: 'Replaces the classroom and default levels for this value on this device.'
					},
					{
						id: 'type',
						label: `All ${typeLabel}`,
						hint: 'A device with its own levels for this value keeps them.'
					}
				]
			: []
	);

	/** Saving a level from a layer below copies that layer's levels up; say so. */
	function note(scope: string) {
		if (!editRule || editRule.source === scope) return null;
		const from =
			editRule.source === 'default'
				? 'This is a platform default.'
				: `This level is set for all ${typeLabel}.`;
		const to = scope === 'device' ? 'this device' : `all ${typeLabel}`;
		const below = editRule.source === 'default' ? 'platform defaults' : 'classroom levels';
		return `${from} Saving copies the ${parameter.label} levels to ${to}, with this change; the ${below} stay as they are.`;
	}

	async function save(scope: string, input: AlertRuleInput) {
		await services.labs.saveAlert(alerts.deviceId, scope as AlertScope, input, editRule?.id);
		await onchange?.();
	}

	const canReset = $derived(
		canEdit && (source === 'device' || (source === 'type' && alerts.canEditType))
	);
</script>

<div class="mt-3 border-t border-line pt-3" aria-label="{parameter.label} alert levels">
	<div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
		<p class="text-xs text-muted">
			{#if !source}
				No alert levels.
			{:else if source === 'device'}
				Alert levels for this device
			{:else if source === 'type'}
				Alert levels for all {typeLabel}
			{:else}
				Platform default alert levels
			{/if}
			{#if latest}· {latest.updatedBy}, {day.format(new Date(latest.updatedAt!))}{/if}
			{#if canReset}
				·
				<button type="button" class="underline hover:text-ink" onclick={reset}>
					{source === 'device' ? 'Use classroom levels' : 'Reset to default'}
				</button>
			{/if}
		</p>
		{#if canEdit}
			<button
				type="button"
				class="btn btn-ghost px-2.5 py-1 text-xs"
				aria-haspopup="dialog"
				onclick={() => edit(null)}
				><Icon name="plus" class="size-3.5" /> Add alert<span class="sr-only">
					on {parameter.label}</span
				></button
			>
		{/if}
	</div>
	{#if rules.length}
		<ul class="mt-2 space-y-1.5">
			{#each rules as rule (rule.id)}
				<li class="flex flex-wrap items-center gap-x-2 gap-y-1">
					<span
						class={[
							'rounded-full px-2 py-0.5 text-xs font-medium ring-1',
							!rule.enabled
								? 'text-muted line-through ring-line'
								: rule.level === 'warning'
									? 'bg-danger/15 text-danger ring-danger/40'
									: 'bg-orange/15 text-orange ring-orange/40'
						]}
					>
						<span aria-hidden="true">{rule.level === 'warning' ? '▲▲' : '▲'}</span>
						{ruleText(rule, parameter.unit)}
					</span>
					<span class={['text-xs', rule.enabled ? 'text-ink' : 'text-muted']}>
						{rule.title}{#if !rule.enabled}&nbsp;· off{/if}
					</span>
					{#if canEdit}
						<span class="ml-auto flex gap-1">
							<button
								type="button"
								class="btn btn-ghost px-2 py-0.5 text-xs"
								aria-label="Change {rule.title}"
								onclick={() => edit(rule)}>Edit</button
							>
							<button
								type="button"
								class="btn btn-ghost px-2 py-0.5 text-xs"
								aria-label="{rule.enabled ? 'Turn off' : 'Turn on'} {rule.title}"
								onclick={() => toggle(rule)}>{rule.enabled ? 'Turn off' : 'Turn on'}</button
							>
							{#if rule.source === 'device' || (rule.source === 'type' && alerts.canEditType)}
								<button
									type="button"
									class="btn btn-ghost px-2 py-0.5 text-xs"
									aria-label="Delete {rule.title}"
									onclick={() => remove(rule)}>Delete</button
								>
							{/if}
						</span>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}
	{#if error}<p class="field-error" role="alert">{error}</p>{/if}
</div>

<!-- Mounted only while open: a domain view has a chart per value. -->
{#if canEdit && open}
	<AlertRuleModal
		bind:open
		{parameter}
		source={alerts.deviceName}
		rule={editRule}
		{scopes}
		initialScope={editRule ? scopeFor(editRule) : 'device'}
		{note}
		{save}
	/>
{/if}
