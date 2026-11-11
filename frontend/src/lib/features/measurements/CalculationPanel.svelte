<script lang="ts">
	import Modal from '#lib/components/Modal.svelte';
	import type { CalculationDetails, Parameter } from '#lib/domain/types.ts';
	import { errorMessage, services } from '#lib/services/index.ts';

	/**
	 * "How is this calculated?" (AP-07): the formula, inputs, source and SAM version of one value,
	 * fetched from the registry when it opens. `onpick` opens another value's calculation, so an
	 * input can be followed to how it is measured.
	 */
	let {
		open = $bindable(false),
		parameterId,
		onpick
	}: {
		open?: boolean;
		parameterId: string;
		onpick: (parameterId: string) => unknown;
	} = $props();

	const METHOD = {
		measured: 'Measured',
		derived: 'Calculated from other values',
		manual: 'Entered by hand'
	};
	const SOURCE: Record<Parameter['source'], string> = {
		weather_station: 'Weather station',
		device: 'Sensor',
		manual: 'People, on a guided form'
	};

	let details = $state<CalculationDetails | null>(null);
	let error = $state('');

	$effect(() => {
		if (!open) return;
		const id = parameterId;
		error = '';
		services.registry
			.getCalculation(id)
			.then((result) => {
				if (id === parameterId) details = result;
			})
			.catch((e) => (error = errorMessage(e)));
	});

	const shown = $derived(details?.parameter.id === parameterId ? details : null);
	const published = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' });
</script>

<Modal
	bind:open
	title={shown?.parameter.label ?? 'How is this calculated?'}
	eyebrow="How is this calculated?"
>
	{#if error}
		<p class="field-error" role="alert">{error}</p>
	{:else if !shown}
		<p class="text-sm text-muted" aria-live="polite">Loading…</p>
	{:else}
		<dl class="space-y-4 text-sm">
			<div>
				<dt class="text-muted">What it means</dt>
				<dd class="mt-0.5 text-ink">{shown.parameter.plain ?? 'No plain-language note yet.'}</dd>
			</div>
			<div>
				<dt class="text-muted">Method</dt>
				<dd class="mt-0.5 text-ink">{METHOD[shown.method]}</dd>
			</div>
			<div>
				<dt class="text-muted">{shown.method === 'derived' ? 'Formula' : 'How it is taken'}</dt>
				<dd
					class={[
						'mt-0.5 text-ink',
						shown.method === 'derived' &&
							'rounded-md bg-raised p-3 font-mono text-xs leading-relaxed'
					]}
				>
					{shown.formula}
				</dd>
			</div>
			{#if shown.inputDetails.length}
				<div>
					<dt class="text-muted">Inputs</dt>
					<dd class="mt-1">
						<ul class="flex flex-wrap gap-2">
							{#each shown.inputDetails as input (input.id)}
								<li>
									<button
										type="button"
										class="chip hover:border-line-strong"
										aria-label="How {input.label.toLowerCase()} is calculated"
										onclick={() => onpick(input.id)}
										>{input.label}{#if input.unit}<span class="text-muted">&nbsp;{input.unit}</span
											>{/if}</button
									>
								</li>
							{/each}
						</ul>
					</dd>
				</div>
			{/if}
			<div class="grid gap-4 sm:grid-cols-3">
				<div>
					<dt class="text-muted">Unit</dt>
					<dd class="mt-0.5 text-ink">{shown.parameter.unit || 'None'}</dd>
				</div>
				<div>
					<dt class="text-muted">Source</dt>
					<dd class="mt-0.5 text-ink">{SOURCE[shown.parameter.source]}</dd>
				</div>
				<div>
					<dt class="text-muted">Version</dt>
					<dd class="mt-0.5 text-ink">
						{shown.version}, {published.format(new Date(shown.publishedAt))}
					</dd>
				</div>
			</div>
		</dl>
	{/if}
</Modal>
