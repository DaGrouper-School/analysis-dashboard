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

	const blankDomain = (): DomainInput => ({ id: '', label: '', icon: '', description: '' });
	let domainForm = $state(blankDomain());
	let domainErrors = $state<Record<string, string>>({});
	let savingDomain = $state(false);
	let editingDomain = $state<string | null>(null);
	let editForm = $state(blankDomain());

	async function addDomain(event: SubmitEvent) {
		event.preventDefault();
		const created = await createDomain(domainForm);
		if (created) domainForm = blankDomain();
	}

	/** Shared by the add form and the dialog. Returns the new domain, or null on errors. */
	async function createDomain(input: DomainInput): Promise<DomainDefinition | null> {
		domainErrors = validateDomain(input);
		if (Object.keys(domainErrors).length) return null;
		savingDomain = true;
		actionError = notice = '';
		try {
			const domain = await services.registry.createDomain(input);
			await invalidateAll();
			notice = `Added the ${domain.label} domain.`;
			return domain;
		} catch (e) {
			if (e instanceof ServiceError) domainErrors = e.fields;
			if (!Object.keys(domainErrors).length) actionError = errorMessage(e);
			return null;
		} finally {
			savingDomain = false;
		}
	}

	function startEdit(domain: DomainDefinition) {
		editingDomain = domain.id;
		editForm = { ...domain };
		domainErrors = {};
	}

	async function saveEdit(event: SubmitEvent) {
		event.preventDefault();
		domainErrors = validateDomain(editForm);
		if (Object.keys(domainErrors).length) return;
		const { id, ...rest } = editForm;
		if (await run(() => services.registry.updateDomain(id, rest), `Saved ${rest.label}.`)) {
			editingDomain = null;
		}
	}

	function removeDomain(domain: DomainDefinition) {
		run(() => services.registry.deleteDomain(domain.id), `Removed the ${domain.label} domain.`);
	}

	// The "New domain…" dialog, opened from a domain picker.
	let dialog = $state<HTMLDialogElement>();
	let dialogForm = $state(blankDomain());
	let afterCreate: ((domain: DomainDefinition) => unknown) | null = null;
	// Bumped when the dialog closes, so a picker left on "New domain…" shows its value again.
	let pickers = $state(0);

	function openNewDomain(then: (domain: DomainDefinition) => unknown) {
		afterCreate = then;
		dialogForm = blankDomain();
		domainErrors = {};
		dialog?.showModal();
	}

	async function submitDialog(event: SubmitEvent) {
		event.preventDefault();
		const domain = await createDomain(dialogForm);
		if (!domain) return;
		const then = afterCreate;
		dialog?.close();
		await then?.(domain);
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
	let rows = $state<ValueRow[]>([blankRow()]);
	let typeErrors = $state<Record<string, string>>({});
	let savingType = $state(false);
	let typeSection = $state<HTMLElement>();

	function resetType() {
		editingType = null;
		typeLabel = typeDescription = '';
		typeColor = DEVICE_TYPE_PALETTE[5];
		rows = [blankRow()];
		typeErrors = {};
	}

	function editType(type: DeviceTypeDefinition) {
		editingType = type.id;
		typeLabel = type.label;
		typeDescription = type.description;
		typeColor = type.color;
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
		typeSection?.scrollIntoView({ behavior: 'smooth', block: 'start' });
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

	async function saveType(event: SubmitEvent) {
		event.preventDefault();
		// Drop blank rows first, so error positions match the rows left on screen.
		rows = rows.filter((r) => r.id.trim() || r.label.trim());
		const input = {
			label: typeLabel,
			description: typeDescription,
			color: typeColor,
			values: rows.map((r) => ({
				id: r.id,
				label: r.label,
				unit: r.unit,
				valueType: r.valueType,
				domain: r.domain
			}))
		};
		typeErrors = validateDeviceType(input);
		if (Object.keys(typeErrors).length) return;
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
		} catch (e) {
			if (e instanceof ServiceError) typeErrors = e.fields;
			if (!Object.keys(typeErrors).length) actionError = errorMessage(e);
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
		<option value={domain.id} selected={domain.id === selected}>{domain.icon} {domain.label}</option
		>
	{/each}
	<option value={NEW_DOMAIN}>+ New domain…</option>
{/snippet}

{#snippet domainFields(form: DomainInput, codeLocked: boolean, prefix: string)}
	<div class="grid gap-3 sm:grid-cols-[8rem_1fr_5rem]">
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
		<div>
			<label class="label" for="{prefix}-icon">Icon</label>
			<input
				id="{prefix}-icon"
				class={['input text-center', domainErrors.icon && 'input-error']}
				placeholder="🌱"
				bind:value={form.icon}
			/>
		</div>
	</div>
	{#each ['id', 'label', 'icon'] as field (field)}
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

	{#if unmapped.length}
		<section class="card mt-6 border-orange/50" aria-labelledby="unmapped-title">
			<h2 id="unmapped-title" class="text-lg font-semibold">
				<span aria-hidden="true">⚠</span>
				{unmapped.length} value{unmapped.length === 1 ? '' : 's'} not in a domain
			</h2>
			<p class="mt-1 text-sm text-muted">
				These still show on the dashboard, under "Not in a domain". Pick a domain, or create one.
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

	<!-- Domains -->
	<section class="mt-10" aria-labelledby="domains-title">
		<h2 id="domains-title" class="text-xl font-semibold">Environmental domains</h2>
		<p class="mt-1 text-sm text-muted">
			A domain can be edited at any time. It can only be removed once no values are in it.
		</p>
		<ul class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
			{#each data.domains as domain (domain.id)}
				{@const count = mappedCount(domain.id)}
				<li class="card flex flex-col p-4">
					{#if editingDomain === domain.id}
						<form onsubmit={saveEdit} novalidate>
							{@render domainFields(editForm, true, `edit-${domain.id}`)}
							<div class="mt-3 flex gap-2">
								<button class="btn btn-primary">Save</button>
								<button type="button" class="btn btn-ghost" onclick={() => (editingDomain = null)}
									>Cancel</button
								>
							</div>
						</form>
					{:else}
						<div class="flex items-start justify-between gap-3">
							<p class="text-2xl" aria-hidden="true">{domain.icon}</p>
							<span class="font-mono text-xs text-muted">{domain.id}</span>
						</div>
						<p class="mt-2 font-semibold text-ink">{domain.label}</p>
						<p class="text-sm text-muted">{domain.description || 'No description.'}</p>
						<p class="mt-2 text-xs text-muted">{count} value{count === 1 ? '' : 's'}</p>
						<div class="mt-auto flex flex-wrap gap-2 pt-3">
							<button
								type="button"
								class="btn btn-ghost px-3 py-1.5"
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
					{/if}
				</li>
			{/each}
		</ul>
		<form class="card mt-4" onsubmit={addDomain} novalidate aria-labelledby="add-domain-title">
			<h3 id="add-domain-title" class="font-semibold">Add a domain</h3>
			<div class="mt-3">{@render domainFields(domainForm, false, 'domain')}</div>
			<button class="btn btn-primary mt-4" disabled={savingDomain}>
				{savingDomain ? 'Adding…' : 'Add domain'}
			</button>
		</form>
	</section>

	<!-- Device types -->
	<section class="mt-10" aria-labelledby="types-title">
		<h2 id="types-title" class="text-xl font-semibold">Device types</h2>
		<p class="mt-1 text-sm text-muted">
			What a classroom can register, and the values each type is expected to send.
		</p>
		<ul class="mt-4 grid gap-3 lg:grid-cols-2">
			{#each data.deviceTypes as type (type.id)}
				{@const summary = typeDomains(type)}
				<li class="card p-4">
					<div class="flex items-start justify-between gap-3">
						<div>
							<p class="flex items-center gap-2 font-semibold text-ink">
								<span class="size-2.5 rounded-full" style:background={type.color}></span>
								{type.label}
							</p>
							<p class="text-sm text-muted">{type.description}</p>
						</div>
						<button type="button" class="btn btn-ghost px-3 py-1.5" onclick={() => editType(type)}
							>Edit</button
						>
					</div>
					<p class="mt-3 text-xs text-muted">
						{type.parameterIds.length} value{type.parameterIds.length === 1 ? '' : 's'}
						<span class="font-mono">· {type.id}</span>
					</p>
					<p class="mt-2 flex flex-wrap gap-1.5">
						{#each summary.domains as domain (domain.id)}
							<span class="chip"><span aria-hidden="true">{domain.icon}</span> {domain.label}</span>
						{/each}
						{#if summary.loose}
							<span class="chip border-orange/60 text-orange">{summary.loose} not in a domain</span>
						{/if}
					</p>
				</li>
			{/each}
		</ul>

		<form
			class="card mt-4 scroll-mt-28"
			bind:this={typeSection}
			onsubmit={saveType}
			novalidate
			aria-labelledby="type-form-title"
		>
			<h3 id="type-form-title" class="font-semibold">
				{editingType ? `Edit ${typeLabel || 'device type'}` : 'Register a device type'}
			</h3>
			<div class="mt-3 grid gap-3 sm:grid-cols-2">
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

			<h4 class="mt-5 text-sm font-semibold text-ink">Expected values</h4>
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
						{@const isNew = !existing}
						<li
							class="grid gap-2 rounded-md border border-line p-3 sm:grid-cols-[1.2fr_1.2fr_5rem_6.5rem_1.2fr_auto]"
						>
							<div>
								<label class="label text-xs" for="row-{row.key}-id">ID</label>
								<input
									id="row-{row.key}-id"
									class={['input font-mono text-sm', typeErrors[`values.${i}.id`] && 'input-error']}
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
								</select>
							</div>
							<div>
								<label class="label text-xs" for="row-{row.key}-domain">Domain</label>
								{#if isNew}
									<select
										id="row-{row.key}-domain"
										class="input text-sm"
										onchange={(e) => rowDomain(row, e.currentTarget.value)}
									>
										{@render domainOptions(row.domain)}
									</select>
								{:else}
									<p id="row-{row.key}-domain" class="py-2.5 text-sm">
										{#if existing.domain && domainsById.has(existing.domain)}
											{domainsById.get(existing.domain)!.icon}
											{domainsById.get(existing.domain)!.label}
										{:else}
											<span class="text-orange">Not in a domain</span>
										{/if}
									</p>
								{/if}
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
									{#if !isNew}Change its domain under "Values and domains" below.{/if}
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
			<div class="mt-5 flex flex-wrap gap-2">
				<button class="btn btn-primary" disabled={savingType}>
					{savingType ? 'Saving…' : editingType ? 'Save device type' : 'Register device type'}
				</button>
				{#if editingType}
					<button type="button" class="btn btn-ghost" onclick={resetType}>Cancel</button>
				{/if}
			</div>
		</form>
	</section>

	<!-- Mapping -->
	<section class="mt-10" aria-labelledby="mapping-title">
		<div class="flex flex-wrap items-end justify-between gap-3">
			<div>
				<h2 id="mapping-title" class="text-xl font-semibold">Values and domains</h2>
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
							<tr><td colspan="3" class="px-4 py-6 text-center text-muted">No values match.</td></tr
							>
						{/each}
					</tbody>
				{/key}
			</table>
		</div>
	</section>

	<!-- Change log -->
	<section class="mt-10" aria-labelledby="log-title">
		<h2 id="log-title" class="text-xl font-semibold">Change log</h2>
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
	</section>

	<dialog
		bind:this={dialog}
		class="m-auto w-[min(32rem,calc(100vw-2rem))] rounded-md border border-line-strong bg-raised p-0 text-text shadow-2xl backdrop:bg-black/70"
		aria-labelledby="new-domain-title"
		onclose={() => {
			afterCreate = null;
			pickers++;
		}}
	>
		<form class="p-6" onsubmit={submitDialog} novalidate>
			<h2 id="new-domain-title" class="text-lg font-semibold">New domain</h2>
			<p class="mb-4 text-sm text-muted">
				It is added for every classroom, then used for this value.
			</p>
			{@render domainFields(dialogForm, false, 'dialog')}
			<div class="mt-5 flex justify-end gap-2">
				<button type="button" class="btn btn-ghost" onclick={() => dialog?.close()}>Cancel</button>
				<button class="btn btn-primary" disabled={savingDomain}>
					{savingDomain ? 'Adding…' : 'Add and use'}
				</button>
			</div>
		</form>
	</dialog>
{/if}
