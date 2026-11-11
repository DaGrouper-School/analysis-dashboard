<script lang="ts">
	import { untrack } from 'svelte';
	import { replaceState } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import Icon from '#lib/components/Icon.svelte';
	import Sparkline from '#lib/components/Sparkline.svelte';
	import type {
		Parameter,
		PublicDomainCard,
		PublicSource,
		PublicValue
	} from '#lib/domain/types.ts';
	import { FALLBACK_DOMAIN_ICON } from '#lib/icons.ts';
	import { formatValue } from '#lib/key-values.ts';

	/**
	 * "What this classroom measures" (AP-22), the same on the public classroom page and the
	 * dashboard: a card per domain with its first readings, or one domain with all of them. The
	 * open domain is kept in the URL (`?domain=`), so a shared link opens the same one.
	 */
	let {
		domains,
		parameters,
		classroomId,
		class: className = ''
	}: {
		domains: PublicDomainCard[];
		parameters: Parameter[];
		classroomId: string;
		class?: string;
	} = $props();

	const paramsById = $derived(new Map(parameters.map((p) => [p.id, p])));
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	/** A card shows this many readings; its domain tab shows them all. */
	const CARD_READINGS = 3;
	const sourceName = (source: PublicSource) => source.name ?? 'Field observations';
	const domainKey = (domain: { id: string } | null) => domain?.id ?? 'other';
	const valueKey = (value: PublicValue) => `${value.source.id}|${value.parameterId}`;

	let active = $state('all');
	untrack(() => {
		const wanted = page.url.searchParams.get('domain');
		if (wanted && domains.some((c) => domainKey(c.domain) === wanted)) active = wanted;
	});
	const tabs = $derived([
		{ id: 'all', label: 'All domains', icon: 'layers', count: undefined as number | undefined },
		...domains.map((card) => ({
			id: domainKey(card.domain),
			label: card.domain?.label ?? 'Other',
			icon: card.domain?.icon ?? FALLBACK_DOMAIN_ICON,
			count: card.values.length
		}))
	]);
	const openCard = $derived(domains.find((c) => domainKey(c.domain) === active));

	function open(id: string) {
		active = id;
		const url = new URL(page.url.href);
		if (id === 'all') url.searchParams.delete('domain');
		else url.searchParams.set('domain', id);
		// The current page's own URL, with only the domain changed.
		// eslint-disable-next-line svelte/no-navigation-without-resolve
		replaceState(url, page.state);
	}

	function keys(event: KeyboardEvent) {
		const step = { ArrowRight: 1, ArrowLeft: -1 }[event.key];
		if (!step) return;
		event.preventDefault();
		const i = tabs.findIndex((t) => t.id === active);
		open(tabs[(i + step + tabs.length) % tabs.length].id);
		document.getElementById(`domain-tab-${active}`)?.focus();
	}
</script>

{#snippet reading(value: PublicValue, compact: boolean)}
	{@const param = paramsById.get(value.parameterId)}
	<li class="flex items-center justify-between gap-3 py-2.5">
		<span class="min-w-0">
			<a
				class="link"
				href={resolve('/explore/[classroom]/[parameter]', {
					classroom: classroomId,
					parameter: value.parameterId
				})}>{param?.label ?? value.parameterId}</a
			>
			<span class="block truncate text-xs text-muted">
				{sourceName(value.source)}{compact
					? ''
					: ` · ${dateTime.format(new Date(value.observedAt))}`}
			</span>
		</span>
		<span class="flex shrink-0 items-center gap-3">
			<Sparkline values={value.last7Days} />
			<span
				class={[
					'max-w-40 min-w-20 text-right font-mono text-ink tabular-nums',
					typeof value.value === 'number' && 'whitespace-nowrap'
				]}
			>
				{formatValue(value.value, param?.unit, param?.valueType)}
			</span>
		</span>
	</li>
{/snippet}

{#snippet domainTitle(card: PublicDomainCard)}
	<Icon
		name={card.domain?.icon ?? FALLBACK_DOMAIN_ICON}
		fallback={FALLBACK_DOMAIN_ICON}
		class="size-5 text-muted"
	/>
	{card.domain?.label ?? 'Other'}
{/snippet}

<section class={className} aria-labelledby="domains-heading">
	<h2 id="domains-heading" class="font-display text-2xl">What this classroom measures</h2>
	{#if domains.length}
		<div
			class="mt-4 flex gap-1 overflow-x-auto border-b border-line"
			role="tablist"
			aria-label="Domains"
			tabindex="-1"
			onkeydown={keys}
		>
			{#each tabs as tab (tab.id)}
				{@const selected = tab.id === active}
				<button
					type="button"
					role="tab"
					id="domain-tab-{tab.id}"
					aria-selected={selected}
					aria-controls="domain-panel"
					tabindex={selected ? 0 : -1}
					class={[
						'-mb-px flex shrink-0 items-center gap-2 border-b-2 px-3 py-2 text-sm whitespace-nowrap transition',
						selected
							? 'border-ink font-medium text-ink'
							: 'border-transparent text-muted hover:text-ink'
					]}
					onclick={() => open(tab.id)}
				>
					<Icon name={tab.icon} fallback={FALLBACK_DOMAIN_ICON} class="size-4" />
					{tab.label}
					{#if tab.count !== undefined}<span class="text-xs text-muted">{tab.count}</span>{/if}
				</button>
			{/each}
		</div>

		<div
			id="domain-panel"
			class="mt-4"
			role="tabpanel"
			aria-labelledby="domain-tab-{active}"
			tabindex="-1"
		>
			{#if openCard}
				<article class="card">
					<h3 class="flex items-center gap-2 text-lg font-semibold">
						{@render domainTitle(openCard)}
					</h3>
					{#if openCard.domain?.description}
						<p class="mt-1 text-sm text-muted">{openCard.domain.description}</p>
					{/if}
					<p class="mt-3 text-xs text-muted">Sparklines show the last reading of each of 7 days.</p>
					<ul class="mt-2 divide-y divide-line">
						{#each openCard.values as value (valueKey(value))}
							{@render reading(value, false)}
						{/each}
					</ul>
				</article>
			{:else}
				<div class="grid gap-4 md:grid-cols-2">
					{#each domains as card (domainKey(card.domain))}
						<article class="card flex flex-col">
							<h3 class="text-lg font-semibold">
								<button
									type="button"
									class="flex items-center gap-2 hover:text-teal"
									onclick={() => open(domainKey(card.domain))}
								>
									{@render domainTitle(card)}
								</button>
							</h3>
							<ul class="mt-3 divide-y divide-line">
								{#each card.values.slice(0, CARD_READINGS) as value (valueKey(value))}
									{@render reading(value, true)}
								{/each}
							</ul>
							<button
								type="button"
								class="link mt-auto self-start pt-3 text-sm"
								onclick={() => open(domainKey(card.domain))}
							>
								{card.values.length > CARD_READINGS
									? `All ${card.values.length} readings`
									: `Open ${card.domain?.label ?? 'Other'}`}
							</button>
						</article>
					{/each}
				</div>
			{/if}
		</div>
	{:else}
		<p class="mt-3 text-muted">Nothing from this classroom has been released yet.</p>
	{/if}
</section>
