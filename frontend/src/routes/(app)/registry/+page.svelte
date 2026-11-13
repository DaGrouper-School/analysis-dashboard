<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import type { DomainDefinition, Parameter } from '#lib/domain/types.ts';
	import DefaultAlerts from '#lib/features/alerts/DefaultAlerts.svelte';
	import ExplanationModal from '#lib/features/measurements/ExplanationModal.svelte';
	import VerticalTabs, { type TabItem } from '#lib/components/VerticalTabs.svelte';
	import { errorMessage, services } from '#lib/services/index.ts';
	import ChangeLog from './ChangeLog.svelte';
	import DomainModal from './DomainModal.svelte';
	import DomainsPanel from './DomainsPanel.svelte';
	import TypeModal from './TypeModal.svelte';
	import TypesPanel from './TypesPanel.svelte';
	import ValuesPanel from './ValuesPanel.svelte';
	import { CHART_LABELS } from '#lib/domain/charts.ts';
	import { domainOptions, NEW_DOMAIN } from './type-form.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const parameters = $derived(data.allowed ? data.parameters : []);
	const paramsById = $derived(new Map(parameters.map((p) => [p.id, p])));
	const domainsById = $derived(new Map(data.domains.map((d) => [d.id, d])));
	const unmapped = $derived(
		parameters.filter((p) => !p.domain || !domainsById.has(p.domain)).length
	);
	const domainChoices = $derived(domainOptions(data.domains));

	/** Each tab's icon, also drawn by its section title. */
	const ICONS = {
		domains: 'tag',
		types: 'layers',
		values: 'readings',
		alerts: 'alert',
		log: 'history'
	};
	type Tab = keyof typeof ICONS;
	let tab = $state<Tab>();
	const tabs = $derived<TabItem<Tab>[]>([
		{ id: 'domains', label: 'Domains', icon: ICONS.domains, count: data.domains.length },
		{ id: 'types', label: 'Measurement types', icon: ICONS.types, count: data.deviceTypes.length },
		{
			id: 'values',
			label: 'Values & domains',
			icon: ICONS.values,
			count: parameters.length,
			badge: unmapped ? { text: String(unmapped), title: `${unmapped} not in a domain` } : undefined
		},
		{
			id: 'alerts',
			label: 'Default alerts',
			icon: ICONS.alerts,
			count: data.allowed ? data.alertDefaults.length : 0
		},
		{
			id: 'log',
			label: 'Change log',
			icon: ICONS.log,
			count: data.allowed ? data.changes.length : 0
		}
	]);

	let explaining = $state(false);
	let explainId = $state('');
	const explained = $derived(paramsById.get(explainId) ?? null);

	let notice = $state('');
	let actionError = $state('');
	const report = (status: { notice: string; error: string }) => {
		notice = status.notice;
		actionError = status.error;
	};

	async function run(action: () => Promise<unknown>, done: string) {
		report({ notice: '', error: '' });
		try {
			await action();
			await invalidateAll();
			notice = done;
		} catch (e) {
			actionError = errorMessage(e);
		}
	}

	let domainModal = $state<DomainModal>();
	let typeModal = $state<TypeModal>();
	// Bumped when the domain dialog closes, so a picker left on "New domain…" shows its value again.
	let pickers = $state(0);

	const openNewDomain = (then: ((domain: DomainDefinition) => unknown) | null = null) =>
		domainModal?.openNew(then);

	function mapTo(parameter: Parameter, value: string) {
		if (value === NEW_DOMAIN) {
			openNewDomain((domain) =>
				run(
					() => services.registry.mapParameter(parameter.id, domain.id),
					`${parameter.label} now shows under ${domain.label}.`
				)
			);
			return;
		}
		const domain = value ? domainsById.get(value) : null;
		run(
			() => services.registry.mapParameter(parameter.id, domain?.id ?? null),
			domain
				? `${parameter.label} now shows under ${domain.label}.`
				: `${parameter.label} is no longer in a domain.`
		);
	}
</script>

<svelte:head><title>Registry · DaGrouper Analysis</title></svelte:head>

<p class="eyebrow">Platform admin</p>
<h1 class="mt-1 font-display text-3xl sm:text-4xl">Registry</h1>
<p class="mt-2 max-w-2xl text-muted">
	Measurement types, the values they send, and the environmental domains those values show under.
	They are shared by every classroom: register a type here and any classroom can add a measurement
	of that type and see its domains. Every change is logged.
</p>

{#if !data.allowed}
	<div class="card mt-8">
		<h2 class="text-xl font-semibold">Platform admins only</h2>
		<p class="mt-2 text-sm text-muted">
			Only a platform admin can change measurement types and domains. Ask one if a measurement's
			values show up under the wrong domain.
		</p>
	</div>
{:else}
	<div
		class="fixed right-4 bottom-4 z-30 w-[min(24rem,calc(100vw-2rem))] space-y-2 shadow-lg max-sm:bottom-20"
		aria-live="polite"
	>
		{#if notice}
			<p
				class="rounded-md border border-lime/40 bg-surface px-4 py-2 text-sm text-lime"
				role="status"
			>
				{notice}
			</p>
		{/if}
		{#if actionError}
			<p
				class="rounded-md border border-danger/40 bg-surface px-4 py-2 text-sm text-danger"
				role="alert"
			>
				{actionError}
			</p>
		{/if}
	</div>

	<div class="mt-8">
		<VerticalTabs {tabs} bind:active={tab} label="Registry">
			{#if tab === 'domains'}
				<DomainsPanel
					domains={data.domains}
					{parameters}
					icon={ICONS.domains}
					onadd={() => openNewDomain()}
					onedit={(domain) => domainModal?.edit(domain)}
					onremove={(domain) =>
						run(
							() => services.registry.deleteDomain(domain.id),
							`Removed the ${domain.label} domain.`
						)}
				/>
			{:else if tab === 'types'}
				<TypesPanel
					deviceTypes={data.deviceTypes}
					domains={data.domains}
					parameters={paramsById}
					icon={ICONS.types}
					onnew={() => typeModal?.openNew()}
					onedit={(type) => typeModal?.edit(type)}
				/>
			{:else if tab === 'values'}
				<ValuesPanel
					{parameters}
					domains={data.domains}
					deviceTypes={data.deviceTypes}
					{domainChoices}
					{pickers}
					icon={ICONS.values}
					onmap={mapTo}
					onchart={(parameter, chart) =>
						run(
							() => services.registry.chartParameter(parameter.id, chart),
							`${parameter.label} now charts as ${CHART_LABELS[chart].toLowerCase()}.`
						)}
					onexplain={(id) => {
						explainId = id;
						explaining = true;
					}}
				/>
			{:else if tab === 'alerts'}
				<DefaultAlerts
					defaults={data.alertDefaults}
					{parameters}
					deviceTypes={data.deviceTypes}
					domains={data.domains}
					icon={ICONS.alerts}
					onchange={() => invalidateAll()}
				/>
			{:else}
				<ChangeLog changes={data.changes} icon={ICONS.log} />
			{/if}
		</VerticalTabs>
	</div>

	<TypeModal
		bind:this={typeModal}
		parameters={paramsById}
		domains={domainsById}
		{domainChoices}
		{pickers}
		onnewdomain={openNewDomain}
		onstatus={report}
	/>
	<DomainModal bind:this={domainModal} onstatus={report} onclose={() => pickers++} />
{/if}

{#if explaining && explained}
	<ExplanationModal
		bind:open={explaining}
		parameter={explained}
		{parameters}
		onsaved={() => invalidateAll()}
	/>
{/if}
