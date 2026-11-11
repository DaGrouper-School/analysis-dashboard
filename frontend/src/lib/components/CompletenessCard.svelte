<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import type { DomainCompleteness } from '#lib/domain/types.ts';
	import { FALLBACK_DOMAIN_ICON } from '#lib/icons.ts';

	/**
	 * "How complete is this month?" (AP-22): each domain's share of what the research protocol
	 * expects, with a square per day. Missing days stay empty; they are never filled in.
	 */
	let { domains, class: className = '' }: { domains: DomainCompleteness[]; class?: string } =
		$props();

	const dayMonth = new Intl.DateTimeFormat(undefined, {
		day: 'numeric',
		month: 'short',
		timeZone: 'UTC'
	});
	const domainKey = (domain: { id: string } | null) => domain?.id ?? 'other';
	const percent = (d: DomainCompleteness) =>
		d.expected ? Math.round((d.received / d.expected) * 100) : 0;
	const missingDays = (d: DomainCompleteness) =>
		d.days.filter((day) => day.status === 'missing').map((day) => day.day);
	const DAY_TONE = {
		complete: 'bg-teal',
		partial: 'bg-teal/40',
		missing: 'border border-dashed border-line-strong'
	} as const;
	const DAY_WORD = { complete: 'complete', partial: 'partly received', missing: 'no readings' };
</script>

<div class={['card divide-y divide-line p-0', className]}>
	{#each domains as d (domainKey(d.domain))}
		{@const missing = missingDays(d)}
		<div class="flex flex-col gap-3 px-5 py-4 md:flex-row md:items-center">
			<p class="flex w-40 shrink-0 items-center gap-2 font-medium text-ink">
				<Icon
					name={d.domain?.icon ?? FALLBACK_DOMAIN_ICON}
					fallback={FALLBACK_DOMAIN_ICON}
					class="size-4 text-muted"
				/>
				{d.domain?.label ?? 'Other'}
			</p>
			<p class="w-36 shrink-0 text-sm">
				<span class="font-mono text-ink tabular-nums">{percent(d)}%</span>
				<span class="text-muted">· {d.received} of {d.expected}</span>
			</p>
			{#if d.days.length}
				<div class="min-w-0 flex-1">
					<ol class="flex flex-wrap gap-1" aria-label="Days of the month">
						{#each d.days as day (day.day)}
							<li
								class={['size-3.5 rounded-[3px]', DAY_TONE[day.status]]}
								title="{dayMonth.format(new Date(`${day.day}T00:00:00Z`))}: {DAY_WORD[day.status]}"
							>
								<span class="sr-only"
									>{dayMonth.format(new Date(`${day.day}T00:00:00Z`))}: {DAY_WORD[day.status]}</span
								>
							</li>
						{/each}
					</ol>
					{#if missing.length}
						<p class="mt-1.5 text-xs text-muted">
							No readings on {missing
								.map((m) => dayMonth.format(new Date(`${m}T00:00:00Z`)))
								.join(', ')}
						</p>
					{/if}
				</div>
			{:else}
				<p class="text-sm text-muted">Measured weekly</p>
			{/if}
		</div>
	{/each}
</div>
<p class="mt-2 flex flex-wrap gap-4 text-xs text-muted" aria-hidden="true">
	<span class="flex items-center gap-1.5"
		><span class="size-3 rounded-[3px] bg-teal"></span>Complete</span
	>
	<span class="flex items-center gap-1.5"
		><span class="size-3 rounded-[3px] bg-teal/40"></span>Partly received</span
	>
	<span class="flex items-center gap-1.5"
		><span class="size-3 rounded-[3px] border border-dashed border-line-strong"></span>No readings</span
	>
</p>
