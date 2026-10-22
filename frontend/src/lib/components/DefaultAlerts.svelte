<script lang="ts">
	import type {
		AlertRule,
		AlertRuleInput,
		DomainDefinition,
		Parameter
	} from '#lib/domain/types.ts';
	import { bySeverity, ruleText } from '#lib/domain/alerts.ts';
	import AlertRuleModal from '#lib/components/AlertRuleModal.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import { errorMessage, services } from '#lib/services/index.ts';

	/**
	 * The platform default alert levels (AP-08), for platform admins: each value's defaults, with
	 * controls to add, change, turn off and delete them. A default is in force on every device
	 * whose classroom and device have no levels of their own for that value.
	 */
	let {
		defaults,
		parameters,
		domains,
		icon,
		onchange
	}: {
		defaults: AlertRule[];
		parameters: Parameter[];
		domains: DomainDefinition[];
		/** The section title's icon, as on its tab. */
		icon: string;
		onchange: () => unknown;
	} = $props();

	const numeric = $derived(parameters.filter((p) => (p.valueType ?? 'number') === 'number'));
	const byId = $derived(new Map(numeric.map((p) => [p.id, p])));
	const domainLabel = (id: string | null) => domains.find((d) => d.id === id)?.label ?? 'No domain';
	/** Values with defaults, by domain then label. */
	const groups = $derived(
		[...new Set(defaults.map((r) => r.parameterId))]
			.map((id) => byId.get(id))
			.filter((p): p is Parameter => !!p)
			.sort(
				(a, b) =>
					domainLabel(a.domain).localeCompare(domainLabel(b.domain)) ||
					a.label.localeCompare(b.label)
			)
	);
	const withoutDefaults = $derived(
		numeric
			.filter((p) => !defaults.some((r) => r.parameterId === p.id))
			.sort((a, b) => a.label.localeCompare(b.label))
	);

	let open = $state(false);
	let parameter = $state<Parameter | null>(null);
	let editRule = $state<AlertRule | null>(null);
	let pick = $state('');
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

	function addFor(event: Event) {
		const target = byId.get((event.currentTarget as HTMLSelectElement).value);
		pick = '';
		if (target) edit(target, null);
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

	<div class="mt-4 max-w-sm">
		<label class="label" for="default-alert-value">Add a default for another value</label>
		<select id="default-alert-value" class="input" bind:value={pick} onchange={addFor}>
			<option value="" disabled>Pick a value…</option>
			{#each withoutDefaults as option (option.id)}
				<option value={option.id}>{option.label}{option.unit ? ` (${option.unit})` : ''}</option>
			{/each}
		</select>
	</div>
	{#if error}<p class="field-error mt-3" role="alert">{error}</p>{/if}

	{#if groups.length}
		<ul class="mt-4 space-y-3">
			{#each groups as target (target.id)}
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
		<p class="mt-4 text-sm text-muted">No default alert levels yet.</p>
	{/if}
</div>

<!-- Mounted only while open, for the value picked. -->
{#if open && parameter}
	<AlertRuleModal bind:open {parameter} rule={editRule} {save} />
{/if}
