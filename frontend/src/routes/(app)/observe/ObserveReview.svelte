<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import type {
		Device,
		GeoPoint,
		ObservationTarget,
		ObservationTemplate,
		Parameter
	} from '#lib/domain/types.ts';
	import ParticipantIdNote from '#lib/features/consent/ParticipantIdNote.svelte';
	import { formatPoint } from '#lib/features/map/geo.ts';
	import { STEP_INFO } from './observe-form.ts';
	import ValueGrid from './ValueGrid.svelte';

	/** The observation form's last step: everything that will be sent, to check first. */
	let {
		template,
		target,
		device,
		values,
		parameters,
		observedAt,
		location,
		error
	}: {
		template: ObservationTemplate;
		target: ObservationTarget;
		device: Device | null;
		values: Record<string, number | string>;
		parameters: Map<string, Parameter>;
		observedAt: string;
		/** The person's GPS fix, or null to use the lab's location. */
		location: GeoPoint | null;
		error: string;
	} = $props();

	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
</script>

<h2 class="flex items-center gap-2 text-lg font-semibold">
	<Icon name={STEP_INFO.review.icon} class="size-5 text-muted" /> Check and send
</h2>
<ValueGrid class="mt-4" {values} {parameters}>
	{#snippet before()}
		<div class="bg-surface p-3">
			<dt class="text-xs text-muted">Recording</dt>
			<dd class="mt-1 text-ink">{template.label}</dd>
		</div>
		<div class="bg-surface p-3">
			<dt class="text-xs text-muted">Lab</dt>
			<dd class="mt-1 text-ink">{target.lab.name} · {target.classroomName}</dd>
		</div>
		{#if device}
			<div class="bg-surface p-3">
				<dt class="text-xs text-muted">Device</dt>
				<dd class="mt-1 text-ink">{device.name} · {device.identifier}</dd>
			</div>
		{/if}
	{/snippet}
	{#snippet after()}
		<div class="bg-surface p-3">
			<dt class="text-xs text-muted">When</dt>
			<dd class="mt-1 text-ink">{observedAt ? dateTime.format(new Date(observedAt)) : '—'}</dd>
		</div>
		<div class="bg-surface p-3">
			<dt class="text-xs text-muted">Where</dt>
			<dd class="mt-1 text-ink">
				{#if location}
					{formatPoint(location)} <span class="text-muted">· your GPS</span>
				{:else}
					{target.lab.name}'s location <span class="text-muted">· no GPS</span>
				{/if}
			</dd>
		</div>
	{/snippet}
</ValueGrid>
<ParticipantIdNote class="mt-3" />
{#if error}<p class="field-error mt-3" role="alert">{error}</p>{/if}
