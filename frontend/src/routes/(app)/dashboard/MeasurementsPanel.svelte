<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import Select from '#lib/components/Select.svelte';
	import type { Parameter } from '#lib/domain/types.ts';
	import RegisterSensorModal from '#lib/features/devices/RegisterSensorModal.svelte';
	import { keySummary } from '#lib/features/measurements/key-values.ts';
	import {
		deviceTypeColor,
		deviceTypeIcon,
		deviceTypeLabel,
		deviceTypeOf,
		isManualType
	} from '#lib/utils/labels.ts';
	import { sensorQuery } from '#lib/utils/sensor-selection.ts';
	import PanelHead from './PanelHead.svelte';
	import type { PageData } from './$types';

	/**
	 * The Measurements section (AP-45): the classroom's devices, each with its latest reading, a way
	 * to sync or enter one, and its raw data. Heads and facilitators add a device to a lab here.
	 */
	let {
		data,
		classroomId,
		classroomName,
		parameters,
		canAddDevice,
		onsync
	}: {
		data: Pick<
			PageData,
			| 'devices'
			| 'deviceTypes'
			| 'classroomConditions'
			| 'canRecord'
			| 'labs'
			| 'domains'
			| 'parameters'
		>;
		classroomId: string;
		classroomName: string;
		parameters: ReadonlyMap<string, Parameter>;
		canAddDevice: boolean;
		onsync: (deviceId: string) => void;
	} = $props();

	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const conditionOf = (id: string) => data.classroomConditions.find((c) => c.device.id === id);

	let adding = $state(false);
	let addLabId = $state('');
	const addLab = $derived(data.labs.find((l) => l.id === addLabId) ?? data.labs[0]);
	const labOptions = $derived(data.labs.map((l) => ({ value: l.id, label: l.name })));
</script>

<div class="flex flex-wrap items-end justify-between gap-3">
	<div>
		<PanelHead
			title="Measurements"
			sub="The devices in {classroomName}'s labs, each with its latest reading."
		/>
	</div>
	{#if canAddDevice}
		<div class="flex flex-wrap items-center gap-2">
			{#if data.labs.length > 1}
				<Select
					options={labOptions}
					value={addLab?.id}
					label="Lab to add it to"
					size="compact"
					onchange={(id) => (addLabId = id)}
				/>
			{/if}
			<button
				type="button"
				class="btn btn-primary"
				aria-haspopup="dialog"
				onclick={() => (adding = true)}
			>
				<Icon name="plus" class="size-4" /> Add measurement
			</button>
		</div>
	{/if}
</div>
<ul class="mt-5 space-y-2.5">
	{#each data.devices as option (option.id)}
		{@const item = conditionOf(option.id)}
		<li class="card flex flex-wrap items-center gap-x-5 gap-y-3 p-4 sm:px-5">
			<span
				class="grid size-11 shrink-0 place-items-center rounded-md border border-line bg-raised"
				style:color={deviceTypeColor(data.deviceTypes, option.type)}
			>
				<Icon name={deviceTypeIcon(data.deviceTypes, option.type)} class="size-5" />
			</span>
			<div class="min-w-0 flex-[2_1_14rem]">
				<p class="font-medium text-ink">{option.name}</p>
				<p class="text-xs text-muted">
					{deviceTypeLabel(data.deviceTypes, option.type)} · {option.labName}
				</p>
			</div>
			<div class="min-w-0 flex-[2_1_12rem] text-sm">
				<p class="text-ink">
					{item
						? keySummary(
								deviceTypeOf(data.deviceTypes, option.type)?.tileParameterIds,
								item.reading.values,
								parameters
							)
						: 'No readings yet'}
				</p>
				{#if item}
					<p class="mt-0.5 flex items-center gap-1.5 text-xs">
						<span class="size-1.5 rounded-full bg-lime" aria-hidden="true"></span>
						Seen {dateTime.format(new Date(item.reading.observedAt))}
					</p>
				{/if}
			</div>
			<div class="flex flex-wrap gap-2">
				{#if data.canRecord && !isManualType(data.deviceTypes, option.type)}
					<button
						type="button"
						class="btn btn-ghost"
						aria-haspopup="dialog"
						onclick={() => onsync(option.id)}><Icon name="sync" class="size-4" /> Sync</button
					>
				{:else if data.canRecord}
					<!-- eslint-disable svelte/no-navigation-without-resolve -->
					<a
						class="btn btn-ghost"
						href={`${resolve('/(app)/observe')}?lab=${option.labId}&device=${option.id}`}
						>Enter a reading</a
					>
					<!-- eslint-enable svelte/no-navigation-without-resolve -->
				{/if}
				<!-- eslint-disable svelte/no-navigation-without-resolve -->
				<a
					class="btn btn-ghost"
					href={resolve('/(app)/dashboard/readings') + sensorQuery(classroomId, option.id)}
					>Raw data</a
				>
				<!-- eslint-enable svelte/no-navigation-without-resolve -->
			</div>
		</li>
	{:else}
		<li class="text-muted">No devices yet. Ask your facilitator to add one in one of the labs.</li>
	{/each}
</ul>

{#if canAddDevice && addLab}
	<RegisterSensorModal
		bind:open={adding}
		labId={addLab.id}
		labName={addLab.name}
		deviceTypes={data.deviceTypes}
		domains={data.domains}
		parameters={data.parameters}
		onclose={() => invalidateAll()}
	/>
{/if}
