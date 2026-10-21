<script lang="ts">
	import type {
		ClassroomAlerts,
		Device,
		DeviceTypeDefinition,
		DomainDefinition,
		EffectiveAlertRule,
		Parameter
	} from '#lib/domain/types.ts';
	import { ruleText } from '#lib/domain/alerts.ts';
	import AlertRuleModal from '#lib/components/AlertRuleModal.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import { errorMessage, services } from '#lib/services/index.ts';

	/**
	 * A classroom's alert levels per value (AP-08), grouped by domain. Heads and facilitators
	 * add, change, turn off and delete them; everyone else reads them.
	 */
	let {
		classroomId,
		alerts,
		devices,
		deviceTypes,
		domains,
		parameters,
		onchange
	}: {
		classroomId: string;
		alerts: ClassroomAlerts;
		devices: Device[];
		deviceTypes: DeviceTypeDefinition[];
		domains: DomainDefinition[];
		parameters: Parameter[];
		onchange: () => unknown;
	} = $props();

	const paramsById = $derived(new Map(parameters.map((p) => [p.id, p])));

	// The numeric values the classroom's devices send: the ones an alert can be set on.
	const sent = $derived.by(() => {
		const types = new Set(devices.map((d) => d.type));
		const ids = new Set(deviceTypes.filter((t) => types.has(t.id)).flatMap((t) => t.parameterIds));
		return parameters.filter((p) => ids.has(p.id) && (p.valueType ?? 'number') === 'number');
	});

	/** Values with levels, by domain in the registry's order. The classroom's own always show. */
	const groups = $derived.by(() => {
		const sentIds = new Set(sent.map((p) => p.id));
		const shown: Record<string, EffectiveAlertRule[]> = {};
		for (const rule of alerts.rules) {
			if (rule.source === 'default' && !sentIds.has(rule.parameterId)) continue;
			(shown[rule.parameterId] ??= []).push(rule);
		}
		const sections = [
			...domains.map((d) => ({ id: d.id, label: d.label, icon: d.icon })),
			{ id: null, label: 'Other values', icon: 'tag' }
		];
		return sections
			.map((section) => ({
				...section,
				items: Object.entries(shown)
					.filter(([id]) => (paramsById.get(id)?.domain ?? null) === section.id)
					.map(([id, rules]) => ({ id, parameter: paramsById.get(id), rules }))
			}))
			.filter((section) => section.items.length);
	});

	const count = $derived(groups.reduce((n, g) => n + g.items.length, 0));
	const day = new Intl.DateTimeFormat(undefined, { day: 'numeric', month: 'short' });

	let editing = $state(false);
	let editRule = $state<EffectiveAlertRule | null>(null);
	let error = $state('');

	function add() {
		editRule = null;
		editing = true;
	}

	function edit(rule: EffectiveAlertRule) {
		editRule = rule;
		editing = true;
	}

	async function run(action: () => Promise<unknown>) {
		error = '';
		try {
			await action();
			await onchange();
		} catch (e) {
			error = errorMessage(e);
		}
	}

	function toggle(rule: EffectiveAlertRule) {
		const { parameterId, direction, value, level, title } = rule;
		run(() =>
			services.classrooms.saveAlert(
				classroomId,
				{ parameterId, direction, value, level, title, enabled: !rule.enabled },
				rule.id
			)
		);
	}

	function remove(rule: EffectiveAlertRule, label: string, last: boolean) {
		const then = last
			? ` It is the classroom's last ${label} level, so the platform defaults apply again.`
			: '';
		if (!confirm(`Delete “${rule.title}”?${then}`)) return;
		run(() => services.classrooms.deleteAlert(classroomId, rule.id));
	}

	function reset(parameterId: string, label: string) {
		if (!confirm(`Drop the classroom's own ${label} levels and use the platform defaults?`)) return;
		run(() => services.classrooms.resetAlerts(classroomId, parameterId));
	}
</script>

<section id="alerts" class="card mt-6 scroll-mt-24" aria-labelledby="alerts-title">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<h2 id="alerts-title" class="text-lg font-semibold">
			Alerts <span class="text-muted">{count}</span>
		</h2>
		{#if alerts.canEdit}
			<button
				type="button"
				class="btn btn-ghost px-3 py-1.5 text-xs"
				aria-haspopup="dialog"
				disabled={!sent.length}
				onclick={add}><Icon name="plus" class="size-4" /> Add alert</button
			>
		{/if}
	</div>
	<p class="mt-1 max-w-2xl text-sm text-muted">
		Levels per value for all of this classroom's sensors. They raise alerts on the dashboard and
		show as lines on its charts. Values the classroom has not set use the platform defaults.
		{#if !alerts.canEdit}The institution head and facilitators can change them.{/if}
	</p>
	{#if alerts.canEdit && !sent.length}
		<p class="mt-2 text-sm text-muted">Register a sensor to set alerts on the values it sends.</p>
	{/if}
	{#if error}<p class="field-error" role="alert">{error}</p>{/if}

	{#each groups as group (group.id)}
		<h3 class="mt-6 flex items-center gap-2 text-sm font-medium text-muted">
			<Icon name={group.icon} fallback="tag" class="size-4" />
			{group.label}
		</h3>
		<ul class="mt-2 divide-y divide-line rounded-md border border-line">
			{#each group.items as item (item.id)}
				{@const label = item.parameter?.label ?? item.id}
				{@const own = item.rules[0].source === 'classroom'}
				{@const latest = item.rules
					.filter((r) => r.updatedAt)
					.toSorted((a, b) => b.updatedAt!.localeCompare(a.updatedAt!))[0]}
				<li class="p-3 sm:p-4">
					<div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
						<p class="font-medium">{label}</p>
						<p class="text-xs text-muted">
							{#if own && latest}
								Set by {latest.updatedBy}, {day.format(new Date(latest.updatedAt!))}
							{:else}
								Platform default
							{/if}
							{#if own && alerts.canEdit}
								·
								<button
									type="button"
									class="underline hover:text-ink"
									onclick={() => reset(item.id, label)}>Reset to default</button
								>
							{/if}
						</p>
					</div>
					<ul class="mt-2 space-y-2">
						{#each item.rules as rule (rule.id)}
							<li class="flex flex-wrap items-center gap-x-3 gap-y-1.5">
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
									{ruleText(rule, item.parameter?.unit)}
								</span>
								<span class={['text-sm', rule.enabled ? 'text-ink' : 'text-muted']}>
									{rule.title}{#if !rule.enabled}<span class="text-xs">&nbsp;· off</span>{/if}
								</span>
								{#if alerts.canEdit}
									<span class="ml-auto flex gap-1">
										<button
											type="button"
											class="btn btn-ghost px-2.5 py-1 text-xs"
											aria-label="Change {rule.title}"
											onclick={() => edit(rule)}>Edit</button
										>
										<button
											type="button"
											class="btn btn-ghost px-2.5 py-1 text-xs"
											aria-label="{rule.enabled ? 'Turn off' : 'Turn on'} {rule.title}"
											onclick={() => toggle(rule)}>{rule.enabled ? 'Turn off' : 'Turn on'}</button
										>
										{#if rule.source === 'classroom'}
											<button
												type="button"
												class="btn btn-ghost px-2.5 py-1 text-xs"
												aria-label="Delete {rule.title}"
												onclick={() => remove(rule, label, item.rules.length === 1)}>Delete</button
											>
										{/if}
									</span>
								{/if}
							</li>
						{/each}
					</ul>
				</li>
			{/each}
		</ul>
	{:else}
		<p class="mt-4 text-sm text-muted">
			No alert levels for the values this classroom's sensors send.
		</p>
	{/each}
</section>

{#if alerts.canEdit}
	<AlertRuleModal
		bind:open={editing}
		{classroomId}
		parameters={editRule ? [paramsById.get(editRule.parameterId)!].filter(Boolean) : sent}
		rule={editRule}
		onsaved={onchange}
	/>
{/if}
