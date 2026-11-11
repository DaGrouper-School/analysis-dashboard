<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import Select from '#lib/components/Select.svelte';
	import type { Device, DeviceTypeDefinition, ObservationTarget } from '#lib/domain/types.ts';
	import { deviceOptions } from '#lib/utils/labels.ts';
	import { STEP_INFO } from './observe-form.ts';

	/** The observation form's device step: which of the lab's fitting devices was read. */
	let {
		target,
		fitting,
		device,
		deviceId = $bindable(),
		deviceTypes,
		error,
		tried
	}: {
		target: ObservationTarget;
		/** The lab's devices of the template's type. Nothing else can be picked. */
		fitting: Device[];
		device: Device | null;
		deviceId: string;
		deviceTypes: DeviceTypeDefinition[];
		error: string;
		/** Next was tried: show the error. */
		tried: boolean;
	} = $props();
</script>

<div>
	<h2 class="flex items-center gap-2 text-lg font-semibold">
		<Icon name={STEP_INFO.device.icon} class="size-5 text-muted" /> Which device did you read?
	</h2>
	{#if fitting.length > 1}
		<div class="mt-3 max-w-sm">
			<label class="label" for="observe-device">Device</label>
			<Select
				id="observe-device"
				placeholder="Choose a device"
				options={deviceOptions(fitting, deviceTypes, 'identifier')}
				bind:value={deviceId}
				error={tried && !!error}
				aria-describedby="observe-device-help"
			/>
		</div>
	{:else if device}
		<p id="observe-device" tabindex="-1" class="mt-3 text-ink">
			{device.name} <span class="text-muted">· {device.identifier}</span>
		</p>
	{:else}
		<p id="observe-device" tabindex="-1" class="field-error">{error}</p>
	{/if}
	{#if tried && error && fitting.length > 1}
		<p class="field-error">{error}</p>
	{/if}
	<p id="observe-device-help" class="mt-2 text-sm text-muted">
		Only devices registered to {target.lab.name} are listed. A facilitator registers new ones on the lab
		page.
	</p>
</div>
