<script lang="ts">
	import { resolve } from '$app/paths';
	import type { ObservationTarget, ObservationTemplate, Parameter } from '#lib/domain/types.ts';
	import ContributorId from '#lib/features/consent/ContributorId.svelte';
	import { clockMessage, type OutboxEntry } from '#lib/features/devices/outbox.ts';
	import { formatPoint } from '#lib/features/map/geo.ts';
	import ValueGrid from './ValueGrid.svelte';

	/** The observation just saved: sent, or waiting on this device until it is back online. */
	let {
		entry,
		template,
		target,
		parameters,
		onanother
	}: {
		entry: OutboxEntry;
		template: ObservationTemplate;
		target: ObservationTarget;
		parameters: Map<string, Parameter>;
		onanother: () => void;
	} = $props();

	const stored = $derived(entry.stored);
	const device = $derived(target.devices.find((d) => d.id === entry.observation.deviceId) ?? null);
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
</script>

<section class="card mt-8" aria-labelledby="sent-heading">
	{#if stored}
		<p class="text-sm text-lime" role="status">Sent</p>
		<h2 id="sent-heading" class="mt-1 text-xl font-semibold">{template.label} recorded</h2>
	{:else}
		<p class="text-sm text-orange" role="status">Saved on this device</p>
		<h2 id="sent-heading" class="mt-1 text-xl font-semibold">
			{template.label} waiting to sync
		</h2>
	{/if}
	<p class="mt-1 text-sm text-muted">
		{dateTime.format(new Date(entry.observation.observedAt))} · {target.classroomName} · {target.lab
			.name}
		{#if device}· {device.name}{/if}
	</p>
	<ValueGrid class="mt-5" values={stored?.values ?? entry.observation.values} {parameters} />
	{#each entry.clockFlags as flag (flag)}
		<p class="mt-4 text-sm text-orange">{clockMessage(flag, entry.clockOffsetMs)}</p>
	{/each}
	<ul class="mt-4 space-y-1 text-sm">
		{#if stored}
			<li>
				Tagged with {target.classroomName}{device
					? `, as a reading from ${device.name}`
					: ', as a field observation'}.
			</li>
			<li>
				{formatPoint(stored.location)}
				{#if stored.locationSource === 'device'}
					<span class="text-muted">· your GPS</span>
				{:else}
					<span class="rounded-full bg-raised px-2 py-0.5 text-xs font-medium ring-1 ring-line"
						>No GPS: lab location, not from the device</span
					>
				{/if}
			</li>
			<li class="text-muted">
				{stored.creditedTo
					? `Credited to ${stored.creditedTo}`
					: 'Credited to your participant ID, without your name'}
				{#if stored.recordedByParticipantId !== null}<ContributorId
						participantId={stored.recordedByParticipantId}
					/>{/if}
			</li>
		{:else}
			<li>
				{entry.error ??
					'It is sent with the others when you are back online. You can keep recording.'}
			</li>
			<li class="text-muted">
				{entry.credit
					? 'It will be credited to you.'
					: 'Not credited to anyone: public outputs show it without a name.'}
			</li>
		{/if}
	</ul>
	<div class="mt-6 flex flex-wrap gap-2">
		<button type="button" class="btn btn-primary" onclick={onanother}>Record another</button>
		<a class="btn btn-ghost" href={resolve('/(app)/labs/[labId]', { labId: target.lab.id })}
			>Open {target.lab.name}</a
		>
	</div>
</section>
