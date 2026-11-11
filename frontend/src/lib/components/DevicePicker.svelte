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

	/**
	 * The classroom's devices as a row of choices (AP-45), each with its registry icon and colour.
	 * The picked one sets what "What's happening now?" and the domains show, and the page's
	 * colour. The same on the public classroom page and the dashboard.
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
</script>

<section class={['card min-w-0 overflow-hidden p-3', className]} aria-labelledby="devices-heading">
	<h2 id="devices-heading" class="sr-only">Devices</h2>
	<div class="-m-1 flex gap-2 overflow-x-auto p-1" role="group" aria-labelledby="devices-heading">
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
</section>
