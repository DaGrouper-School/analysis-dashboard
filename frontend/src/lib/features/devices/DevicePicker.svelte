<script lang="ts" module>
	/** A device to pick, with its registry colour and icon. */
	export interface PickableDevice {
		id: string;
		name: string;
		color: string;
		icon: string;
	}
</script>

<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import Select from '#lib/components/Select.svelte';
	import { MOBILE_TABS_MAX } from '#lib/utils/tabs.ts';

	/**
	 * The classroom's devices as a row of choices (AP-45), each with its registry icon and colour.
	 * The picked one sets what "What's happening now?" and the domains show, and the page's
	 * colour. The same on the public classroom page and the dashboard. More than three devices
	 * become a dropdown, so the card stays one row.
	 */
	let {
		devices,
		deviceId,
		onpick,
		class: className = ''
	}: {
		devices: PickableDevice[];
		deviceId: string | undefined;
		onpick: (id: string) => void;
		class?: string;
	} = $props();

	const options = $derived(
		devices.map((d) => ({ value: d.id, label: d.name, icon: d.icon, color: d.color }))
	);
</script>

<!-- The "Devices" label sits in the card, beside the choices, as part of the selector. -->
<section
	class={['card flex min-w-0 items-center gap-3 overflow-hidden p-3', className]}
	aria-labelledby="devices-heading"
>
	<h2
		id="devices-heading"
		class="shrink-0 self-stretch border-r border-line pr-3 pl-1 text-sm font-medium text-muted"
	>
		<span class="flex h-full items-center">Devices</span>
	</h2>
	{#if devices.length > MOBILE_TABS_MAX}
		<Select
			{options}
			value={deviceId}
			label="Device"
			labelHidden
			class="min-w-0 flex-1 sm:w-96 sm:flex-none"
			onchange={onpick}
		/>
	{:else}
		<div
			class="-m-1 flex min-w-0 flex-1 gap-2 overflow-x-auto p-1"
			role="group"
			aria-labelledby="devices-heading"
		>
			{#each devices as device (device.id)}
				{@const picked = device.id === deviceId}
				<button
					type="button"
					aria-pressed={picked}
					class={[
						'flex shrink-0 items-center gap-2.5 rounded-md border px-3.5 py-2.5 text-sm transition-colors focus-visible:ring-2 focus-visible:ring-teal focus-visible:outline-none',
						picked ? 'text-ink' : 'border-line text-muted hover:border-line-strong hover:text-ink'
					]}
					style:--accent={device.color}
					style:border-color={picked ? device.color : undefined}
					style:background-color={picked
						? `color-mix(in oklab, ${device.color} 14%, transparent)`
						: undefined}
					onclick={() => onpick(device.id)}
				>
					<span
						class="grid size-7 place-items-center rounded-full"
						style:color={device.color}
						style:background-color="color-mix(in oklab, {device.color} 16%, transparent)"
					>
						<Icon name={device.icon} class="size-4" />
					</span>
					{device.name}
				</button>
			{/each}
		</div>
	{/if}
</section>
