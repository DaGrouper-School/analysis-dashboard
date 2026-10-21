<script lang="ts">
	import type {
		Device,
		DeviceTypeDefinition,
		GeoPoint,
		Parameter,
		Reading,
		ReadingInput
	} from '#lib/domain/types.ts';
	import Icon from '#lib/components/Icon.svelte';
	import Modal from '#lib/components/Modal.svelte';
	import PairingVisual, { type PairingState } from '#lib/components/PairingVisual.svelte';
	import StepTrack from '#lib/components/StepTrack.svelte';
	import { formatPoint } from '#lib/geo.ts';
	import { keySummary } from '#lib/key-values.ts';
	import { deviceTypeColor, deviceTypeIcon, deviceTypeLabel } from '#lib/labels.ts';
	import { sampleValues } from '#lib/sample-readings.ts';
	import { errorMessage, services } from '#lib/services/index.ts';

	/**
	 * Sync a registered sensor (simulated), as a step-by-step modal. Until hardware is wired up,
	 * this plays what a real sync does: scan for the sensor, handshake, then stream its buffered
	 * readings to the backend, which tags them with the classroom and forwards them to SAM. GPS
	 * comes with each reading; readings without a fix fall back to the lab's location and are
	 * flagged. `onclose` runs when the modal closes, so the page can show the new readings.
	 */
	let {
		open = $bindable(false),
		devices,
		deviceId = $bindable(''),
		deviceTypes,
		classroomName,
		labLocation,
		lastGps,
		parameters = [],
		onclose
	}: {
		open?: boolean;
		devices: Device[];
		/** The sensor to sync. With more than one device the modal lets the user change it. */
		deviceId?: string;
		deviceTypes: DeviceTypeDefinition[];
		classroomName: string;
		labLocation: GeoPoint;
		/** The device's last GPS position, if it ever sent one. */
		lastGps: (deviceId: string) => GeoPoint | null;
		/** The parameter registry, for labelling values. */
		parameters?: Parameter[];
		onclose: () => unknown;
	} = $props();

	type Phase = 'ready' | 'scanning' | 'handshake' | 'streaming' | 'done' | 'error';
	const COUNT = 6;
	/** Readings that arrive without a GPS fix, to show the lab-location fallback. */
	const NO_FIX = new Set([2, 4]);
	const STEPS = [
		{ label: 'Sensor', icon: 'sensor' },
		{ label: 'Scan', icon: 'radar' },
		{ label: 'Handshake', icon: 'link' },
		{ label: 'Sync', icon: 'sync' },
		{ label: 'Done', icon: 'check' }
	];
	const STEP_OF: Record<Phase, number> = {
		ready: 0,
		scanning: 1,
		handshake: 2,
		streaming: 3,
		done: 5,
		error: 3
	};
	const VISUAL: Record<Phase, PairingState> = {
		ready: 'idle',
		scanning: 'scan',
		handshake: 'handshake',
		streaming: 'stream',
		done: 'done',
		error: 'error'
	};

	let phase = $state<Phase>('ready');
	/** Opt-in: put the recorder's name on these readings. Otherwise they carry no personal credit. */
	let credit = $state(false);
	let sent = $state<Reading[]>([]);
	let error = $state('');
	let run = 0;

	const device = $derived(
		devices.length === 1 ? devices[0] : devices.find((d) => d.id === deviceId)
	);
	const paramsById = $derived(new Map(parameters.map((p) => [p.id, p])));
	const busy = $derived(phase === 'scanning' || phase === 'handshake' || phase === 'streaming');
	const color = $derived(device ? deviceTypeColor(deviceTypes, device.type) : undefined);
	const time = new Intl.DateTimeFormat(undefined, { timeStyle: 'short' });
	const status = $derived(
		{
			ready: 'Standby',
			scanning: 'Scanning',
			handshake: 'Handshake',
			streaming: `Receiving ${sent.length}/${COUNT}`,
			done: 'Link locked',
			error: 'Link lost'
		}[phase]
	);

	const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));

	/** A wandering track: devices can move between readings, e.g. a station that was relocated. */
	function track(start: GeoPoint): GeoPoint[] {
		const points: GeoPoint[] = [];
		let { lat, lng } = start;
		for (let i = 0; i < COUNT; i++) {
			lat += (Math.random() - 0.5) * 0.004;
			lng += (Math.random() - 0.5) * 0.004;
			points.push({ lat: Math.round(lat * 1e5) / 1e5, lng: Math.round(lng * 1e5) / 1e5 });
		}
		return points;
	}

	async function sync() {
		if (!device) return;
		const target = device;
		const id = ++run;
		const live = () => id === run;
		sent = [];
		error = '';
		phase = 'scanning';
		await wait(1600);
		if (!live()) return;
		phase = 'handshake';
		await wait(1400);
		if (!live()) return;
		phase = 'streaming';
		// With no GPS history, assume the device is near the lab.
		const start = lastGps(target.id) ?? {
			lat: labLocation.lat + 0.01,
			lng: labLocation.lng + 0.01
		};
		const points = track(start);
		const now = Date.now();
		const expected = deviceTypes.find((t) => t.id === target.type)?.parameterIds;
		try {
			for (let i = 0; i < COUNT; i++) {
				const input: ReadingInput = {
					// Buffered readings, ten minutes apart, oldest first.
					observedAt: new Date(now - (COUNT - 1 - i) * 10 * 60_000).toISOString(),
					values: sampleValues(target.type, expected),
					location: NO_FIX.has(i) ? null : points[i]
				};
				const [reading] = await services.labs.uploadReadings(target.id, [input], { credit });
				if (!live()) return;
				sent = [reading, ...sent];
				await wait(600);
				if (!live()) return;
			}
			phase = 'done';
		} catch (e) {
			error = errorMessage(e);
			phase = 'error';
		}
	}

	function stop() {
		run++;
		phase = 'ready';
	}

	function closed() {
		// Closing mid-sync stops it; whatever arrived is already saved.
		run++;
		phase = 'ready';
		sent = [];
		error = '';
		onclose();
	}
</script>

<Modal
	bind:open
	title={device ? `Sync ${device.name}` : 'Sync a sensor'}
	eyebrow="Sync"
	onclose={closed}
>
	<div class="space-y-5" style:--accent={color ?? 'var(--color-teal)'}>
		<StepTrack steps={STEPS} current={STEP_OF[phase]} failed={phase === 'error'} />

		{#if device}
			<PairingVisual
				state={VISUAL[phase]}
				icon={deviceTypeIcon(deviceTypes, device.type)}
				identifier={device.identifier}
				{status}
			/>
		{/if}

		<div aria-live="polite">
			{#if phase === 'ready'}
				{#if devices.length > 1}
					<label class="label" for="sync-device">Sensor</label>
					<select id="sync-device" class="input" bind:value={deviceId}>
						<option value="" disabled>Choose a sensor</option>
						{#each devices as d (d.id)}
							<option value={d.id}>{d.name}</option>
						{/each}
					</select>
				{/if}
				{#if device}
					<p class={['text-sm text-muted', devices.length > 1 && 'mt-3']}>
						{deviceTypeLabel(deviceTypes, device.type)} ·
						<span class="font-mono">{device.identifier}</span>. Its buffered readings are recorded
						to {classroomName}. Demo: the sensor is simulated.
					</p>
				{/if}
				<label class="mt-4 flex items-start gap-2 text-sm">
					<input
						type="checkbox"
						class="mt-0.5 rounded-sm border-line-strong bg-transparent text-teal focus:ring-teal"
						bind:checked={credit}
					/>
					<span>
						Credit these readings to me
						<span class="block text-xs text-muted"
							>Your name is shown on them to people in {classroomName}. Leave it off to record them
							without your name.</span
						>
					</span>
				</label>
			{:else if phase === 'scanning'}
				<p class="text-sm text-ink">Scanning for {device?.identifier}…</p>
				<p class="text-sm text-muted">Keep the sensor powered on and close by.</p>
			{:else if phase === 'handshake'}
				<p class="text-sm text-ink">Found it. Handshaking with {device?.identifier}…</p>
				<p class="text-sm text-muted">Confirming it is registered to this classroom.</p>
			{:else if phase === 'streaming'}
				<p class="text-sm text-ink">Receiving readings · {sent.length} of {COUNT}</p>
			{:else if phase === 'done'}
				<p class="text-sm text-ink" role="status">
					{COUNT} readings synced to {classroomName}.
				</p>
				<p class="text-sm text-muted">
					{NO_FIX.size} had no GPS fix and use the lab's location.
				</p>
			{:else if phase === 'error'}
				<p class="field-error mt-0" role="alert">{error}</p>
				{#if sent.length}
					<p class="text-sm text-muted">
						{sent.length} reading{sent.length === 1 ? ' was' : 's were'} saved before the link dropped.
					</p>
				{/if}
			{/if}
		</div>

		{#if sent.length && device}
			<ul
				class="max-h-56 divide-y divide-line overflow-y-auto rounded-md border border-line text-xs"
			>
				{#each sent as reading (reading.id)}
					<li class="reading space-y-1 p-3">
						<div class="flex flex-wrap items-center justify-between gap-2">
							<span class="font-medium text-ink">{time.format(new Date(reading.observedAt))}</span>
							<span
								class="inline-flex items-center gap-1 rounded-full bg-(--accent)/15 px-2 py-0.5 font-medium text-(--accent)"
								><Icon name="check" class="size-3" /> Synced</span
							>
						</div>
						<p class="text-muted">{keySummary(device.type, reading.values, paramsById)}</p>
						<p>
							{formatPoint(reading.location)}
							{#if reading.locationSource === 'device'}
								<span class="text-muted">· device GPS</span>
							{:else}
								<span class="rounded-full bg-raised px-2 py-0.5 font-medium ring-1 ring-line"
									>No GPS fix: lab location, not from the device</span
								>
							{/if}
							<span class="text-muted">
								· {reading.creditedTo ? `credited to ${reading.creditedTo}` : 'not credited'}</span
							>
						</p>
					</li>
				{/each}
			</ul>
		{/if}
	</div>

	{#snippet footer()}
		{#if busy}
			<button type="button" class="btn btn-ghost" onclick={stop}>Stop</button>
		{:else if phase === 'done' || phase === 'error'}
			<button type="button" class="btn btn-ghost" onclick={sync}>
				<Icon name="sync" class="size-4" /> Sync again
			</button>
			<button type="button" class="btn btn-primary" onclick={() => (open = false)}>Done</button>
		{:else}
			<button type="button" class="btn btn-ghost" onclick={() => (open = false)}>Cancel</button>
			<button type="button" class="btn btn-primary" disabled={!device} onclick={sync}>
				<Icon name="sync" class="size-4" /> Start sync
			</button>
		{/if}
	{/snippet}
</Modal>

<style>
	.reading {
		animation: arrive 300ms ease-out;
	}
	@keyframes arrive {
		from {
			opacity: 0;
			transform: translateY(-4px);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.reading {
			animation: none;
		}
	}
</style>
