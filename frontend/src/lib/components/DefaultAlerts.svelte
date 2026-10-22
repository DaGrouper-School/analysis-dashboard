<script lang="ts">
	import type {
		AlertRule,
		AlertRuleInput,
		DeviceTypeDefinition,
		DomainDefinition,
		Parameter
	} from '#lib/domain/types.ts';
	import { bySeverity, ruleText } from '#lib/domain/alerts.ts';
	import AlertRuleModal from '#lib/components/AlertRuleModal.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import { errorMessage, services } from '#lib/services/index.ts';

	/**
	 * The platform default alert levels (AP-08), for platform admins: pick a device type, then
	 * add, change, turn off and delete the defaults on each of its values. Values only people
	 * record sit under "Field observations". A default is in force on every device whose
	 * classroom and device have no levels of their own for that value.
	 */
	let {
		defaults,
		parameters,
		deviceTypes,
		domains,
		icon,
		onchange
	}: {
		defaults: AlertRule[];
		parameters: Parameter[];
		deviceTypes: DeviceTypeDefinition[];
		domains: DomainDefinition[];
		/** The section title's icon, as on its tab. */
		icon: string;
		onchange: () => unknown;
	} = $props();

	const numeric = $derived(parameters.filter((p) => (p.valueType ?? 'number') === 'number'));
	const byId = $derived(new Map(numeric.map((p) => [p.id, p])));
	const domainLabel = (id: string | null) => domains.find((d) => d.id === id)?.label ?? 'No domain';
	const FIELD = '__field__';
	/** Device types with a number value, then field observations: values no device type sends. */
	const groups = $derived([
		...deviceTypes
			.map((t) => ({
				id: t.id as string,
				label: t.label,
				icon: t.icon,
				values: t.parameterIds.map((id) => byId.get(id)).filter((p): p is Parameter => !!p)
			}))
			.filter((g) => g.values.length),
		{
			id: FIELD,
			label: 'Field observations',
			icon: 'clipboard',
			values: numeric.filter((p) => !deviceTypes.some((t) => t.parameterIds.includes(p.id)))
		}
	]);
	const count = (values: Parameter[]) =>
		defaults.filter((r) => values.some((p) => p.id === r.parameterId)).length;
	let groupId = $state('');
	const group = $derived(groups.find((g) => g.id === groupId) ?? groups[0]);

	let open = $state(false);
	let parameter = $state<Parameter | null>(null);
	let editRule = $state<AlertRule | null>(null);
	let error = $state('');

	function edit(target: Parameter, rule: AlertRule | null) {
		parameter = target;
		editRule = rule;
		open = true;
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

	async function save(_scope: string, input: AlertRuleInput) {
		await services.registry.saveAlertDefault(input, editRule?.id);
		await onchange();
	}

	function toggle(rule: AlertRule) {
		const { parameterId, direction, value, level, title } = rule;
		run(() =>
			services.registry.saveAlertDefault(
				{ parameterId, direction, value, level, title, enabled: !rule.enabled },
				rule.id
			)
		);
	}

	function remove(target: Parameter, rule: AlertRule) {
		if (
			!confirm(
				`Delete the default “${rule.title}” on ${target.label}? Every device using the defaults loses it.`
			)
		)
			return;
		run(() => services.registry.deleteAlertDefault(rule.id));
	}
</script>

<div>
	<h2 class="flex items-center gap-2 text-xl font-semibold">
		<Icon name={icon} class="size-5 text-muted" /> Default alerts
	</h2>
	<p class="mt-1 max-w-prose text-sm text-muted">
		In force on every device whose classroom and device have no alert levels of their own for that
		value. Classrooms that already set their own levels keep them.
	</p>

	<div class="mt-4 flex flex-wrap gap-2" role="group" aria-label="Measurement type">
		{#each groups as option (option.id)}
			{@const n = count(option.values)}
			<button
				type="button"
				class={[
					'flex items-center gap-2 rounded-full border px-3 py-1.5 text-sm transition',
					option.id === group.id
						? 'border-ink bg-ink text-canvas'
						: 'border-line text-muted hover:border-line-strong hover:text-ink'
				]}
				aria-pressed={option.id === group.id}
				onclick={() => (groupId = option.id)}
			>
				<Icon name={option.icon} fallback="sensor" class="size-4" />
				{option.label}
				{#if n}<span class="text-xs opacity-70">{n}</span>{/if}
			</button>
		{/each}
	</div>
	{#if error}<p class="field-error mt-3" role="alert">{error}</p>{/if}

	{#if group.values.length}
		<ul class="mt-4 space-y-3" aria-label="{group.label} values">
			{#each group.values as target (target.id)}
				{@const rules = defaults.filter((r) => r.parameterId === target.id).sort(bySeverity)}
				<li class="card" aria-label="{target.label} default alerts">
					<div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
						<p>
							<span class="font-medium text-ink">{target.label}</span>
							<span class="text-xs text-muted">· {domainLabel(target.domain)}</span>
						</p>
						<button
							type="button"
							class="btn btn-ghost px-2.5 py-1 text-xs"
							aria-haspopup="dialog"
							onclick={() => edit(target, null)}
							><Icon name="plus" class="size-3.5" /> Add alert<span class="sr-only">
								on {target.label}</span
							></button
						>
					</div>
					{#if !rules.length}
						<p class="mt-1 text-xs text-muted">No default levels.</p>
					{/if}
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
									{ruleText(rule, target.unit)}
								</span>
								<span class={['text-xs', rule.enabled ? 'text-ink' : 'text-muted']}>
									{rule.title}{#if !rule.enabled}&nbsp;· off{/if}
								</span>
								<span class="ml-auto flex gap-1">
									<button
										type="button"
										class="btn btn-ghost px-2 py-0.5 text-xs"
										aria-label="Change {rule.title}"
										onclick={() => edit(target, rule)}>Edit</button
									>
									<button
										type="button"
										class="btn btn-ghost px-2 py-0.5 text-xs"
										aria-label="{rule.enabled ? 'Turn off' : 'Turn on'} {rule.title}"
										onclick={() => toggle(rule)}>{rule.enabled ? 'Turn off' : 'Turn on'}</button
									>
									<button
										type="button"
										class="btn btn-ghost px-2 py-0.5 text-xs"
										aria-label="Delete {rule.title}"
										onclick={() => remove(target, rule)}>Delete</button
									>
								</span>
							</li>
						{/each}
					</ul>
				</li>
			{/each}
		</ul>
	{:else}
		<p class="mt-4 text-sm text-muted">{group.label} has no number values to set levels on.</p>
	{/if}
</div>

<!-- Mounted only while open, for the value picked. -->
{#if open && parameter}
	<AlertRuleModal bind:open {parameter} rule={editRule} {save} />
{/if}
