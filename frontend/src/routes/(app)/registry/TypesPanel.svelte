<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import type { DeviceTypeDefinition, DomainDefinition, Parameter } from '#lib/domain/types.ts';

	/** The Registry's measurement types (AP-09), each with the domains its values land in. */
	let {
		deviceTypes,
		domains,
		parameters,
		icon,
		onnew,
		onedit
	}: {
		deviceTypes: DeviceTypeDefinition[];
		domains: DomainDefinition[];
		parameters: Map<string, Parameter>;
		icon: string;
		onnew: () => void;
		onedit: (type: DeviceTypeDefinition) => void;
	} = $props();

	/** A type's values that land in each domain, for its summary. */
	function typeDomains(type: DeviceTypeDefinition) {
		const known = new Set(domains.map((d) => d.id));
		const shown = domains.filter((d) =>
			type.parameterIds.some((id) => parameters.get(id)?.domain === d.id)
		);
		const loose = type.parameterIds.filter((id) => {
			const domain = parameters.get(id)?.domain;
			return !domain || !known.has(domain);
		}).length;
		return { domains: shown, loose };
	}
</script>

<div>
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h2 class="flex items-center gap-2 text-xl font-semibold">
				<Icon name={icon} class="size-5 text-muted" /> Measurement types
			</h2>
			<p class="mt-1 text-sm text-muted">
				What a classroom can register, and the values each type is expected to send.
			</p>
		</div>
		<button type="button" class="btn btn-primary" aria-haspopup="dialog" onclick={onnew}
			><Icon name="plus" class="size-4" /> Register measurement type</button
		>
	</div>
	<ul class="mt-4 grid gap-3 lg:grid-cols-2">
		{#each deviceTypes as type (type.id)}
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
							{#if type.manual}<span class="chip py-0 text-[11px]">Manual</span>{/if}
						</p>
						<p class="text-sm text-muted">{type.description}</p>
					</div>
					<button
						type="button"
						class="btn btn-ghost px-3 py-1.5"
						aria-haspopup="dialog"
						onclick={() => onedit(type)}><Icon name="pencil" class="size-4" /> Edit</button
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
						<span class="chip border-orange/60 text-orange">{summary.loose} not in a domain</span>
					{/if}
				</p>
			</li>
		{/each}
	</ul>
</div>
