<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import type {
		DeviceTypeDefinition,
		DomainDefinition,
		DomainInput,
		ExpectedValueInput,
		Parameter
	} from '#lib/domain/types.ts';
	import { validateDeviceType, validateDomain } from '#lib/domain/validation.ts';
	import DefaultAlerts from '#lib/components/DefaultAlerts.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import Modal from '#lib/components/Modal.svelte';
	import IconPicker from '#lib/components/IconPicker.svelte';
	import VerticalTabs, { type TabItem } from '#lib/components/VerticalTabs.svelte';
	import Wizard from '#lib/components/Wizard.svelte';
	import { FALLBACK_TYPE_ICON } from '#lib/icons.ts';
	import { DEVICE_TYPE_PALETTE } from '#lib/labels.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const NEW_DOMAIN = '__new__';
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

	const parameters = $derived(data.allowed ? data.parameters : []);
	const paramsById = $derived(new Map(parameters.map((p) => [p.id, p])));
	const domainsById = $derived(new Map(data.domains.map((d) => [d.id, d])));
	const mappedCount = (domainId: string) => parameters.filter((p) => p.domain === domainId).length;
	/** Device types that send a value, for the mapping table. */
	const typesFor = (parameterId: string) =>
		data.deviceTypes.filter((t) => t.parameterIds.includes(parameterId));
	const unmapped = $derived(parameters.filter((p) => !p.domain || !domainsById.has(p.domain)));

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
		{ id: 'types', label: 'Device types', icon: ICONS.types, count: data.deviceTypes.length },
		{
			id: 'values',
			label: 'Values & domains',
			icon: ICONS.values,
			count: parameters.length,
			badge: unmapped.length
				? { text: String(unmapped.length), title: `${unmapped.length} not in a domain` }
				: undefined
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

	let notice = $state('');
	let actionError = $state('');

	async function run(action: () => Promise<unknown>, done: string) {
		actionError = '';
		notice = '';
		try {
			await action();
			await invalidateAll();
			notice = done;
			return true;
		} catch (e) {
			actionError = errorMessage(e);
			return false;
		}
	}

	// ---- Mapping values to domains -------------------------------------------------------

	let query = $state('');
	const shownParameters = $derived.by(() => {
		const q = query.trim().toLowerCase();
		if (!q) return parameters;
		return parameters.filter((p) =>
			[p.id, p.label, ...typesFor(p.id).map((t) => t.label)].some((t) =>
				t.toLowerCase().includes(q)
			)
		);
	});

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

	// ---- Domains ---------------------------------------------------------------------------

	const blankDomain = (): DomainInput => ({ id: '', label: '', icon: 'tag', description: '' });
	let domainErrors = $state<Record<string, string>>({});
	let savingDomain = $state(false);

	// One dialog adds a domain (from the Add button or a value's domain picker) or edits one.
	let domainOpen = $state(false);
	let editingDomain = $state<string | null>(null);
	let domainForm = $state(blankDomain());
	/** Set when a domain picker asked for the new domain: it is used there once added. */
	let afterCreate = $state<((domain: DomainDefinition) => unknown) | null>(null);
	// Bumped when the dialog closes, so a picker left on "New domain…" shows its value again.
	let pickers = $state(0);

	function openNewDomain(then: ((domain: DomainDefinition) => unknown) | null = null) {
		afterCreate = then;
		editingDomain = null;
		domainForm = blankDomain();
		domainErrors = {};
		domainOpen = true;
	}

	function startEdit(domain: DomainDefinition) {
		afterCreate = null;
		editingDomain = domain.id;
		domainForm = { ...domain };
		domainErrors = {};
		domainOpen = true;
	}

	async function submitDomain(event: SubmitEvent) {
		event.preventDefault();
		domainErrors = validateDomain(domainForm);
		if (Object.keys(domainErrors).length) return;
		savingDomain = true;
		actionError = notice = '';
		try {
			const { id, ...rest } = domainForm;
			const domain = editingDomain
				? await services.registry.updateDomain(id, rest)
				: await services.registry.createDomain(domainForm);
			await invalidateAll();
			notice = editingDomain ? `Saved ${domain.label}.` : `Added the ${domain.label} domain.`;
			const then = afterCreate;
			domainOpen = false;
			await then?.(domain);
		} catch (e) {
			if (e instanceof ServiceError) domainErrors = e.fields;
			if (!Object.keys(domainErrors).length) actionError = errorMessage(e);
		} finally {
			savingDomain = false;
		}
	}

	function removeDomain(domain: DomainDefinition) {
		run(() => services.registry.deleteDomain(domain.id), `Removed the ${domain.label} domain.`);
	}

	// ---- Device types ----------------------------------------------------------------------

	type ValueRow = ExpectedValueInput & { key: number };
	let nextKey = 0;
	const blankRow = (): ValueRow => ({
		key: nextKey++,
		id: '',
		label: '',
		unit: '',
		valueType: 'number',
		domain: null
	});

	let editingType = $state<string | null>(null);
	let typeLabel = $state('');
	let typeDescription = $state('');
	let typeColor = $state(DEVICE_TYPE_PALETTE[5]);
	let typeIcon = $state<string>(FALLBACK_TYPE_ICON);
	let rows = $state<ValueRow[]>([blankRow()]);
	let typeErrors = $state<Record<string, string>>({});
	let savingType = $state(false);
	let typeOpen = $state(false);
	const TYPE_STEPS = [
		{ label: 'Basics', icon: 'tag' },
		{ label: 'Expected values', icon: 'readings' },
		{ label: 'Review', icon: 'review' }
	];
	let typeStep = $state(0);
	/** Which step holds a field's error: values on step two, the rest on step one. */
	const typeStepOf = (field: string) => (field.startsWith('values') ? 1 : 0);

	function resetType() {
		editingType = null;
		typeLabel = typeDescription = '';
		typeColor = DEVICE_TYPE_PALETTE[5];
		typeIcon = FALLBACK_TYPE_ICON;
		rows = [blankRow()];
		typeErrors = {};
		typeStep = 0;
	}

	function newType() {
		resetType();
		typeOpen = true;
	}

	function editType(type: DeviceTypeDefinition) {
		editingType = type.id;
		typeLabel = type.label;
		typeDescription = type.description;
		typeColor = type.color;
		typeIcon = type.icon;
		rows = type.parameterIds.map((id) => {
			const p = paramsById.get(id);
			return {
				key: nextKey++,
				id,
				label: p?.label ?? id,
				unit: p?.unit ?? '',
				valueType: p?.valueType ?? 'number',
				domain: p?.domain ?? null
			};
		});
		if (!rows.length) rows = [blankRow()];
		typeErrors = {};
		typeStep = 0;
		typeOpen = true;
	}

	/** Typing an ID the registry has fills in that value: the type will share it. */
	function rowId(row: ValueRow, id: string) {
		row.id = id;
		const existing = paramsById.get(id.trim());
		if (existing) {
			row.label = existing.label;
			row.unit = existing.unit;
			row.valueType = existing.valueType ?? 'number';
			row.domain = existing.domain;
		}
	}

	function rowDomain(row: ValueRow, value: string) {
		if (value === NEW_DOMAIN) {
			openNewDomain((domain) => (row.domain = domain.id));
			return;
		}
		row.domain = value || null;
	}

	/** The form as input. `prune` drops blank rows first, so error positions match the rows left. */
	function typeInput(prune: boolean) {
		if (prune) rows = rows.filter((r) => r.id.trim() || r.label.trim());
		const filled = rows.filter((r) => r.id.trim() || r.label.trim());
		return {
			label: typeLabel,
			description: typeDescription,
			color: typeColor,
			icon: typeIcon,
			values: filled.map((r) => ({
				id: r.id,
				label: r.label,
				unit: r.unit,
				valueType: r.valueType,
				domain: r.domain
			}))
		};
	}

	/** Shows the errors of `step` (all of them on review) and says whether it is clear. */
	function checkTypeStep(step: number) {
		const all = validateDeviceType(typeInput(step > 0));
		typeErrors = Object.fromEntries(
			Object.entries(all).filter(([field]) => step === 2 || typeStepOf(field) === step)
		);
		if (step === 1 && !rows.length) rows = [blankRow()];
		const fields = Object.keys(typeErrors);
		// On review, send the person back to the first step with a problem.
		if (step === 2 && fields.length) typeStep = Math.min(...fields.map(typeStepOf));
		return !fields.length;
	}

	async function saveType(event: SubmitEvent) {
		event.preventDefault();
		const input = typeInput(true);
		savingType = true;
		actionError = notice = '';
		try {
			const type = editingType
				? await services.registry.updateDeviceType(editingType, input)
				: await services.registry.createDeviceType(input);
			await invalidateAll();
			notice = editingType
				? `Saved the ${type.label} device type.`
				: `Registered the ${type.label} device type. Every classroom can now register one.`;
			resetType();
			typeOpen = false;
		} catch (e) {
			if (e instanceof ServiceError) typeErrors = e.fields;
			const fields = Object.keys(typeErrors);
			if (fields.length) typeStep = Math.min(...fields.map(typeStepOf));
			else actionError = errorMessage(e);
		} finally {
			savingType = false;
		}
	}

	/** A type's values that land in each domain, for its summary. */
	function typeDomains(type: DeviceTypeDefinition) {
		const domains = data.domains.filter((d) =>
			type.parameterIds.some((id) => paramsById.get(id)?.domain === d.id)
		);
		const loose = type.parameterIds.filter((id) => {
			const domain = paramsById.get(id)?.domain;
			return !domain || !domainsById.has(domain);
		}).length;
		return { domains, loose };
	}
</script>

<svelte:head><title>Registry · DaGrouper Analysis</title></svelte:head>

{#snippet domainOptions(selected: string | null)}
	<option value="" selected={!selected}>Not in a domain</option>
	{#each data.domains as domain (domain.id)}
		<option value={domain.id} selected={domain.id === selected}>{domain.label}</option>
	{/each}
	<option value={NEW_DOMAIN}>+ New domain…</option>
{/snippet}

{#snippet typeReview()}
	<h4 class="text-sm font-semibold text-ink">Check and save</h4>
	<dl class="mt-3 grid gap-4 text-sm sm:grid-cols-[10rem_1fr]">
		<dt class="text-muted">Device type</dt>
		<dd class="flex items-center gap-2 text-ink">
			<span
				class="grid size-8 place-items-center rounded-md"
				style:color={typeColor}
				style:background="color-mix(in oklab, {typeColor} 15%, transparent)"
			>
				<Icon name={typeIcon} />
			</span>
			<span>
				{typeLabel}
				{#if typeDescription}<span class="block text-xs text-muted">{typeDescription}</span>{/if}
			</span>
		</dd>
		<dt class="text-muted">Expected values</dt>
		<dd>
			<ul class="divide-y divide-line rounded-md border border-line">
				{#each rows as row (row.key)}
					{@const domain = row.domain ? domainsById.get(row.domain) : null}
					<li class="flex flex-wrap items-center justify-between gap-2 px-3 py-2">
						<span class="text-ink"
							>{row.label}{row.unit ? ` (${row.unit})` : ''}
							<span class="font-mono text-xs text-muted">{row.id}</span></span
						>
						{#if domain}
							<span class="chip"
								><Icon name={domain.icon} fallback="tag" class="size-3.5" /> {domain.label}</span
							>
						{:else}
							<span class="chip border-orange/60 text-orange">Not in a domain</span>
						{/if}
					</li>
				{/each}
			</ul>
		</dd>
	</dl>
{/snippet}

{#snippet domainFields(form: DomainInput, codeLocked: boolean, prefix: string)}
	<div class="grid gap-3 sm:grid-cols-[8rem_1fr]">
		<div>
			<label class="label" for="{prefix}-code">Code</label>
			<input
				id="{prefix}-code"
				class={['input font-mono uppercase', domainErrors.id && 'input-error']}
				placeholder="SOIL"
				disabled={codeLocked}
				bind:value={form.id}
			/>
		</div>
		<div>
			<label class="label" for="{prefix}-label">Name</label>
			<input
				id="{prefix}-label"
				class={['input', domainErrors.label && 'input-error']}
				placeholder="Soil"
				bind:value={form.label}
			/>
		</div>
	</div>
	{#each ['id', 'label'] as field (field)}
		{#if domainErrors[field]}<p class="field-error">{domainErrors[field]}</p>{/if}
	{/each}
	<div class="mt-3">
		<label class="label" for="{prefix}-description">Description</label>
		<input
			id="{prefix}-description"
			class="input"
			placeholder="Moisture, temperature and chemistry of the ground."
			bind:value={form.description}
		/>
	</div>
	<div class="mt-3">
		<IconPicker name="{prefix}-icon" bind:value={form.icon} error={domainErrors.icon} />
	</div>
{/snippet}

<p class="eyebrow">Platform admin</p>
<h1 class="mt-1 font-display text-3xl sm:text-4xl">Registry</h1>
<p class="mt-2 max-w-2xl text-muted">
	Device types, the values they send, and the environmental domains those values show under. They
	are shared by every classroom: register a type here and any classroom can register a device of
	that type and see its domains. Every change is logged.
</p>

{#if !data.allowed}
	<div class="card mt-8">
		<h2 class="text-xl font-semibold">Platform admins only</h2>
		<p class="mt-2 text-sm text-muted">
			Only a platform admin can change device types and domains. Ask one if a sensor's values show
			up under the wrong domain.
		</p>
	</div>
{:else}
	<div
		class="fixed right-4 bottom-4 z-30 w-[min(24rem,calc(100vw-2rem))] space-y-2 shadow-lg"
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
				<!-- Domains -->
				<div>
					<div class="flex flex-wrap items-start justify-between gap-3">
						<div>
							<h2 class="flex items-center gap-2 text-xl font-semibold">
								<Icon name={ICONS.domains} class="size-5 text-muted" /> Environmental domains
							</h2>
							<p class="mt-1 text-sm text-muted">
								A domain can be edited at any time. It can only be removed once no values are in it.
							</p>
						</div>
						<button
							type="button"
							class="btn btn-primary"
							aria-haspopup="dialog"
							onclick={() => openNewDomain()}><Icon name="plus" class="size-4" /> Add domain</button
						>
					</div>
					<ul class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
						{#each data.domains as domain (domain.id)}
							{@const count = mappedCount(domain.id)}
							<li class="card flex flex-col p-4">
								<div class="flex items-start justify-between gap-3">
									<span
										class="grid size-10 place-items-center rounded-md border border-line-strong text-ink"
									>
										<Icon name={domain.icon} fallback="tag" class="size-6" />
									</span>
									<span class="font-mono text-xs text-muted">{domain.id}</span>
								</div>
								<p class="mt-2 font-semibold text-ink">{domain.label}</p>
								<p class="text-sm text-muted">{domain.description || 'No description.'}</p>
								<p class="mt-2 text-xs text-muted">{count} value{count === 1 ? '' : 's'}</p>
								<div class="mt-auto flex flex-wrap gap-2 pt-3">
									<button
										type="button"
										class="btn btn-ghost px-3 py-1.5"
										aria-haspopup="dialog"
										onclick={() => startEdit(domain)}>Edit</button
									>
									<button
										type="button"
										class="btn btn-ghost px-3 py-1.5"
										disabled={count > 0}
										title={count ? 'Move its values to another domain first' : undefined}
										onclick={() => removeDomain(domain)}>Remove</button
									>
								</div>
							</li>
						{/each}
					</ul>
				</div>
			{:else if tab === 'types'}
				<!-- Device types -->
				<div>
					<div class="flex flex-wrap items-start justify-between gap-3">
						<div>
							<h2 class="flex items-center gap-2 text-xl font-semibold">
								<Icon name={ICONS.types} class="size-5 text-muted" /> Device types
							</h2>
							<p class="mt-1 text-sm text-muted">
								What a classroom can register, and the values each type is expected to send.
							</p>
						</div>
						<button type="button" class="btn btn-primary" aria-haspopup="dialog" onclick={newType}
							><Icon name="plus" class="size-4" /> Register device type</button
						>
					</div>
					<ul class="mt-4 grid gap-3 lg:grid-cols-2">
						{#each data.deviceTypes as type (type.id)}
							{@const summary = typeDomains(type)}
							<li class="card p-4">
								<div class="flex items-start justify-between gap-3">
									<div>
										<p class="flex items-center gap-2 font-semibold text-ink">
											<span
												class="grid size-8 place-items-center rounded-md"
												style:color={type.color}
												style:background="color-mix(in oklab, {type.color} 15%, transparent)"
											>
												<Icon name={type.icon} />
											</span>
											{type.label}
										</p>
										<p class="text-sm text-muted">{type.description}</p>
									</div>
									<button
										type="button"
										class="btn btn-ghost px-3 py-1.5"
										aria-haspopup="dialog"
										onclick={() => editType(type)}>Edit</button
									>
								</div>
								<p class="mt-3 text-xs text-muted">
									{type.parameterIds.length} value{type.parameterIds.length === 1 ? '' : 's'}
									<span class="font-mono">· {type.id}</span>
								</p>
								<p class="mt-2 flex flex-wrap gap-1.5">
									{#each summary.domains as domain (domain.id)}
										<span class="chip"
											><Icon name={domain.icon} fallback="tag" class="size-3.5" />
											{domain.label}</span
										>
									{/each}
									{#if summary.loose}
										<span class="chip border-orange/60 text-orange"
											>{summary.loose} not in a domain</span
										>
									{/if}
								</p>
							</li>
						{/each}
					</ul>
				</div>
			{:else if tab === 'values'}
				<div>
					{#if unmapped.length}
						<section class="card mb-6 border-orange/50" aria-labelledby="unmapped-title">
							<h2 id="unmapped-title" class="text-lg font-semibold">
								<span aria-hidden="true">⚠</span>
								{unmapped.length} value{unmapped.length === 1 ? '' : 's'} not in a domain
							</h2>
							<p class="mt-1 text-sm text-muted">
								These still show on the dashboard, under "Not in a domain". Pick a domain, or create
								one.
							</p>
							{#key pickers}
								<ul class="mt-4 divide-y divide-line">
									{#each unmapped as parameter (parameter.id)}
										<li class="flex flex-wrap items-center justify-between gap-3 py-3">
											<div>
												<p class="font-medium text-ink">{parameter.label}</p>
												<p class="text-xs text-muted">
													<span class="font-mono">{parameter.id}</span>
													{#if typesFor(parameter.id).length}
														· from {typesFor(parameter.id)
															.map((t) => t.label)
															.join(', ')}
													{/if}
												</p>
											</div>
											<label class="sr-only" for="unmapped-{parameter.id}"
												>Domain for {parameter.label}</label
											>
											<select
												id="unmapped-{parameter.id}"
												class="input w-full sm:w-56"
												onchange={(e) => mapTo(parameter, e.currentTarget.value)}
											>
												{@render domainOptions(null)}
											</select>
										</li>
									{/each}
								</ul>
							{/key}
						</section>
					{/if}
					<!-- Mapping -->
					<section aria-labelledby="mapping-title">
						<div class="flex flex-wrap items-end justify-between gap-3">
							<div>
								<h2 id="mapping-title" class="flex items-center gap-2 text-xl font-semibold">
									<Icon name={ICONS.values} class="size-5 text-muted" /> Values and domains
								</h2>
								<p class="mt-1 text-sm text-muted">
									Each value shows under one domain on every classroom's dashboard.
								</p>
							</div>
							<div class="w-full sm:w-72">
								<label class="sr-only" for="value-search">Search values</label>
								<input
									id="value-search"
									class="input"
									type="search"
									placeholder="Value or device type"
									bind:value={query}
								/>
							</div>
						</div>
						<div class="card mt-4 overflow-x-auto p-0">
							<table class="w-full min-w-[40rem] text-left text-sm">
								<thead class="text-xs text-muted">
									<tr class="border-b border-line">
										<th class="px-4 py-3 font-medium">Value</th>
										<th class="px-4 py-3 font-medium">Sent by</th>
										<th class="px-4 py-3 font-medium">Domain</th>
									</tr>
								</thead>
								{#key pickers}
									<tbody class="divide-y divide-line">
										{#each shownParameters as parameter (parameter.id)}
											{@const loose = !parameter.domain || !domainsById.has(parameter.domain)}
											<tr class={loose ? 'bg-orange/5' : ''}>
												<td class="px-4 py-2">
													<p class="text-ink">
														{parameter.label}{parameter.unit ? ` (${parameter.unit})` : ''}
													</p>
													<p class="font-mono text-xs text-muted">{parameter.id}</p>
												</td>
												<td class="px-4 py-2 text-xs text-muted">
													{typesFor(parameter.id)
														.map((t) => t.label)
														.join(', ') || '—'}
												</td>
												<td class="px-4 py-2">
													<label class="sr-only" for="map-{parameter.id}"
														>Domain for {parameter.label}</label
													>
													<select
														id="map-{parameter.id}"
														class={['input py-1.5 text-sm', loose && 'border-orange']}
														onchange={(e) => mapTo(parameter, e.currentTarget.value)}
													>
														{@render domainOptions(loose ? null : parameter.domain)}
													</select>
												</td>
											</tr>
										{:else}
											<tr
												><td colspan="3" class="px-4 py-6 text-center text-muted"
													>No values match.</td
												></tr
											>
										{/each}
									</tbody>
								{/key}
							</table>
						</div>
					</section>
				</div>
			{:else if tab === 'alerts'}
				<DefaultAlerts
					defaults={data.alertDefaults}
					{parameters}
					domains={data.domains}
					icon={ICONS.alerts}
					onchange={() => invalidateAll()}
				/>
			{:else}
				<!-- Change log -->
				<div>
					<h2 class="flex items-center gap-2 text-xl font-semibold">
						<Icon name={ICONS.log} class="size-5 text-muted" /> Change log
					</h2>
					{#if data.changes.length}
						<ol class="card mt-4 divide-y divide-line p-0">
							{#each data.changes.slice(0, 30) as change (change.id)}
								<li class="px-4 py-3 text-sm">
									<p class="text-ink">{change.summary}</p>
									<p class="text-xs text-muted">
										{change.byName} · {dateTime.format(new Date(change.at))}
									</p>
								</li>
							{/each}
						</ol>
					{:else}
						<p class="mt-2 text-sm text-muted">No changes yet.</p>
					{/if}
				</div>
			{/if}
		</VerticalTabs>
	</div>

	<Modal
		bind:open={typeOpen}
		wide
		eyebrow="Device types"
		title={editingType ? `Edit ${typeLabel || 'device type'}` : 'Register a device type'}
	>
		<Wizard
			steps={TYPE_STEPS}
			bind:current={typeStep}
			check={checkTypeStep}
			onsubmit={saveType}
			submitLabel={editingType ? 'Save device type' : 'Register device type'}
			busy={savingType}
			label={editingType ? `Edit ${typeLabel || 'device type'}` : 'Register a device type'}
		>
			{#snippet extra()}
				<button type="button" class="btn btn-ghost" onclick={() => (typeOpen = false)}
					>Cancel</button
				>
			{/snippet}
			{#if typeStep === 0}
				<div class="grid gap-3 sm:grid-cols-2">
					<div>
						<label class="label" for="type-label">Name</label>
						<input
							id="type-label"
							class={['input', typeErrors.label && 'input-error']}
							placeholder="Soil probe"
							bind:value={typeLabel}
						/>
						{#if typeErrors.label}<p class="field-error">{typeErrors.label}</p>{/if}
					</div>
					<div>
						<label class="label" for="type-description">Description</label>
						<input
							id="type-description"
							class="input"
							placeholder="What it is and where it goes."
							bind:value={typeDescription}
						/>
					</div>
				</div>
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
									bind:group={typeColor}
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
						color={typeColor}
						bind:value={typeIcon}
						error={typeErrors.icon}
					/>
				</div>
			{:else if typeStep === 1}
				<h4 class="text-sm font-semibold text-ink">Expected values</h4>
				<p class="text-xs text-muted">
					Use an ID from the registry to share an existing value, or a new ID to add one. Station
					values can't be edited here.
				</p>
				{#if typeErrors.values}<p class="field-error">{typeErrors.values}</p>{/if}
				<datalist id="parameter-ids">
					{#each parameters as parameter (parameter.id)}
						<option value={parameter.id}>{parameter.label}</option>
					{/each}
				</datalist>
				{#key pickers}
					<ol class="mt-3 space-y-3">
						{#each rows as row, i (row.key)}
							{@const existing = paramsById.get(row.id.trim())}
							{@const locked = existing && existing.source !== 'device'}
							<li
								class="grid gap-2 rounded-md border border-line p-3 sm:grid-cols-[1.2fr_1.2fr_5rem_6.5rem_1.2fr_auto]"
							>
								<div>
									<label class="label text-xs" for="row-{row.key}-id">ID</label>
									<input
										id="row-{row.key}-id"
										class={[
											'input font-mono text-sm',
											typeErrors[`values.${i}.id`] && 'input-error'
										]}
										list="parameter-ids"
										placeholder="soil_ph"
										value={row.id}
										oninput={(e) => rowId(row, e.currentTarget.value)}
									/>
								</div>
								<div>
									<label class="label text-xs" for="row-{row.key}-label">Label</label>
									<input
										id="row-{row.key}-label"
										class={['input text-sm', typeErrors[`values.${i}.label`] && 'input-error']}
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
									<select
										id="row-{row.key}-kind"
										class="input text-sm"
										disabled={locked}
										bind:value={row.valueType}
									>
										<option value="number">Number</option>
										<option value="text">Text</option>
										<option value="time">Time (Unix seconds)</option>
									</select>
								</div>
								<div>
									<label class="label text-xs" for="row-{row.key}-domain">Domain</label>
									<select
										id="row-{row.key}-domain"
										class="input text-sm"
										onchange={(e) => rowDomain(row, e.currentTarget.value)}
									>
										{@render domainOptions(row.domain)}
									</select>
								</div>
								<div class="flex items-end">
									<button
										type="button"
										class="btn btn-ghost px-3"
										aria-label="Remove value {i + 1}"
										onclick={() => (rows = rows.filter((r) => r !== row))}>×</button
									>
								</div>
								{#if typeErrors[`values.${i}.id`] || typeErrors[`values.${i}.label`]}
									<p class="field-error sm:col-span-6">
										{typeErrors[`values.${i}.id`] ?? typeErrors[`values.${i}.label`]}
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
				<button
					type="button"
					class="btn btn-ghost mt-3"
					onclick={() => (rows = [...rows, blankRow()])}>+ Add a value</button
				>
			{:else}
				{@render typeReview()}
			{/if}
		</Wizard>
	</Modal>

	<Modal
		bind:open={domainOpen}
		eyebrow="Domains"
		title={editingDomain ? `Edit ${domainForm.label || 'domain'}` : 'New domain'}
		onclose={() => {
			afterCreate = null;
			pickers++;
		}}
	>
		<form id="domain-form" onsubmit={submitDomain} novalidate>
			{#if afterCreate}
				<p class="mb-4 text-sm text-muted">
					It is added for every classroom, then used for this value.
				</p>
			{/if}
			{@render domainFields(domainForm, !!editingDomain, 'domain')}
		</form>
		{#snippet footer()}
			<button type="button" class="btn btn-ghost" onclick={() => (domainOpen = false)}
				>Cancel</button
			>
			<button form="domain-form" class="btn btn-primary" disabled={savingDomain}>
				{savingDomain
					? 'Saving…'
					: editingDomain
						? 'Save domain'
						: afterCreate
							? 'Add and use'
							: 'Add domain'}
			</button>
		{/snippet}
	</Modal>
{/if}
