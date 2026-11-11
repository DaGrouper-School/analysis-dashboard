<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import type { DomainDefinition } from '#lib/domain/types.ts';
	import type { TypeForm } from './type-form.ts';

	/** The measurement type form's last step: the type and its values, to check before saving. */
	let { form, domains }: { form: TypeForm; domains: Map<string, DomainDefinition> } = $props();
</script>

<h4 class="text-sm font-semibold text-ink">Check and save</h4>
<dl class="mt-3 grid gap-4 text-sm sm:grid-cols-[10rem_1fr]">
	<dt class="text-muted">Measurement type</dt>
	<dd class="flex items-center gap-2 text-ink">
		<span
			class="grid size-8 place-items-center rounded-md"
			style:color={form.color}
			style:background="color-mix(in oklab, {form.color} 15%, transparent)"
		>
			<Icon name={form.icon} />
		</span>
		<span>
			{form.label}
			{#if form.description}<span class="block text-xs text-muted">{form.description}</span>{/if}
		</span>
	</dd>
	<dt class="text-muted">Expected values</dt>
	<dd>
		<ul class="divide-y divide-line rounded-md border border-line">
			{#each form.rows as row (row.key)}
				{@const domain = row.domain ? domains.get(row.domain) : null}
				<li class="flex flex-wrap items-center justify-between gap-2 px-3 py-2">
					<span class="text-ink"
						>{row.label}{row.unit ? ` (${row.unit})` : ''}
						<span class="font-mono text-xs text-muted">{row.id}</span></span
					>
					{#if row.tile}<span class="chip">Tile</span>{/if}
					{#if row.card}<span class="chip">Card</span>{/if}
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
