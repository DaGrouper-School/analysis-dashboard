<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import type { DomainDefinition, Parameter } from '#lib/domain/types.ts';

	/** The Registry's environmental domains (AP-09): add, edit, or remove one with no values in it. */
	let {
		domains,
		parameters,
		icon,
		onadd,
		onedit,
		onremove
	}: {
		domains: DomainDefinition[];
		parameters: Parameter[];
		icon: string;
		onadd: () => void;
		onedit: (domain: DomainDefinition) => void;
		onremove: (domain: DomainDefinition) => void;
	} = $props();
</script>

<div>
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h2 class="flex items-center gap-2 text-xl font-semibold">
				<Icon name={icon} class="size-5 text-muted" /> Environmental domains
			</h2>
			<p class="mt-1 text-sm text-muted">
				A domain can be edited at any time. It can only be removed once no values are in it.
			</p>
		</div>
		<button type="button" class="btn btn-primary" aria-haspopup="dialog" onclick={onadd}
			><Icon name="plus" class="size-4" /> Add domain</button
		>
	</div>
	<ul class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
		{#each domains as domain (domain.id)}
			{@const count = parameters.filter((p) => p.domain === domain.id).length}
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
						onclick={() => onedit(domain)}>Edit</button
					>
					<button
						type="button"
						class="btn btn-ghost px-3 py-1.5"
						disabled={count > 0}
						title={count ? 'Move its values to another domain first' : undefined}
						onclick={() => onremove(domain)}>Remove</button
					>
				</div>
			</li>
		{/each}
	</ul>
</div>
