<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import Select, { type SelectOption } from '#lib/components/Select.svelte';
	import type { DeviceTypeDefinition, DomainDefinition, Parameter } from '#lib/domain/types.ts';

	/**
	 * Values and domains (AP-09): every value with the types that send it and the domain it shows
	 * under. Values in no domain are listed first, to place.
	 */
	let {
		parameters,
		domains,
		deviceTypes,
		domainChoices,
		pickers,
		icon,
		onmap,
		onexplain
	}: {
		parameters: Parameter[];
		domains: DomainDefinition[];
		deviceTypes: DeviceTypeDefinition[];
		domainChoices: SelectOption[];
		/** Bumped when the domain dialog closes, so a picker left on "New domain…" resets. */
		pickers: number;
		icon: string;
		onmap: (parameter: Parameter, value: string) => void;
		onexplain: (parameterId: string) => void;
	} = $props();

	const domainsById = $derived(new Map(domains.map((d) => [d.id, d])));
	const unmapped = $derived(parameters.filter((p) => !p.domain || !domainsById.has(p.domain)));
	const typesFor = (parameterId: string) =>
		deviceTypes.filter((t) => t.parameterIds.includes(parameterId));

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
</script>

<div>
	{#if unmapped.length}
		<section class="card mb-6 border-orange/50" aria-labelledby="unmapped-title">
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
							<Select
								id="unmapped-{parameter.id}"
								label="Domain for {parameter.label}"
								class="w-full sm:w-56"
								options={domainChoices}
								value=""
								onchange={(value) => onmap(parameter, value)}
							/>
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
					<Icon name={icon} class="size-5 text-muted" /> Values and domains
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
					placeholder="Value or measurement type"
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
						<th class="px-4 py-3 font-medium"><span class="sr-only">Meaning</span></th>
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
									<Select
										id="map-{parameter.id}"
										label="Domain for {parameter.label}"
										size="compact"
										warn={loose}
										options={domainChoices}
										value={loose ? '' : (parameter.domain ?? '')}
										onchange={(value) => onmap(parameter, value)}
									/>
								</td>
								<td class="px-4 py-2 text-right">
									<button
										type="button"
										class={[
											'btn btn-ghost px-2.5 py-1 text-xs',
											!parameter.plain && 'border-orange'
										]}
										aria-haspopup="dialog"
										title={parameter.plain ?? 'No plain-language note yet'}
										onclick={() => onexplain(parameter.id)}
										>Meaning<span class="sr-only"> of {parameter.label}</span></button
									>
								</td>
							</tr>
						{:else}
							<tr><td colspan="4" class="px-4 py-6 text-center text-muted">No values match.</td></tr
							>
						{/each}
					</tbody>
				{/key}
			</table>
		</div>
	</section>
</div>
