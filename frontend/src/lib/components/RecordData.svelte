<script lang="ts">
	import type { Device, GeoPoint, Parameter, Reading, ReadingInput } from '#lib/domain/types.ts';
	import { formatPoint } from '#lib/geo.ts';
	import { keySummary } from '#lib/key-values.ts';
	import { sampleValues } from '#lib/sample-readings.ts';
	import { errorMessage, services } from '#lib/services/index.ts';

	/**
	 * Record data from a registered sensor (simulated). Until hardware is wired up, this plays
	 * what a real recording does: find the sensor, pair, then stream its buffered readings to the
	 * backend, which tags them with the classroom and forwards them to SAM. GPS comes with each
	 * reading; readings without a fix fall back to the lab's location and are flagged.
	 */
	let {
		devices,
		classroomName,
		labLocation,
		lastGps,
		deviceId = $bindable(''),
		parameters = [],
		onfinish
	}: {
		devices: Device[];
		classroomName: string;
		labLocation: GeoPoint;
		/** The device's last GPS position, if it ever sent one. */
		lastGps: (deviceId: string) => GeoPoint | null;
		/** With one device the picker is hidden: the data goes to that sensor. */
		deviceId?: string;
		/** The parameter registry, for labelling values. */
		parameters?: Parameter[];
		onfinish: () => Promise<void>;
	} = $props();

	type Phase = 'idle' | 'searching' | 'pairing' | 'streaming' | 'done' | 'error';
	const COUNT = 6;
	/** Readings that arrive without a GPS fix, to show the lab-location fallback. */
	const NO_FIX = new Set([2, 4]);

	let phase = $state<Phase>('idle');
	/** Opt-in: put the recorder's name on these readings. Otherwise they carry no personal credit. */
	let credit = $state(false);
	let sent = $state<Reading[]>([]);
	let error = $state('');
	let run = 0;

	const device = $derived(
		devices.length === 1 ? devices[0] : devices.find((d) => d.id === deviceId)
	);
	const paramsById = $derived(new Map(parameters.map((p) => [p.id, p])));
	const target = (reading: Reading) => devices.find((d) => d.id === reading.deviceId) ?? device!;
	const busy = $derived(phase === 'searching' || phase === 'pairing' || phase === 'streaming');
	const time = new Intl.DateTimeFormat(undefined, { timeStyle: 'short' });

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

	async function record() {
		if (!device) return;
		const target = device;
		const id = ++run;
		const live = () => id === run;
		sent = [];
		error = '';
		phase = 'searching';
		await wait(1200);
		if (!live()) return;
		phase = 'pairing';
		await wait(1200);
		if (!live()) return;
		phase = 'streaming';
		// With no GPS history, assume the device is near the lab.
		const start = lastGps(target.id) ?? {
			lat: labLocation.lat + 0.01,
			lng: labLocation.lng + 0.01
		};
		const points = track(start);
		const now = Date.now();
		try {
			for (let i = 0; i < COUNT; i++) {
				const input: ReadingInput = {
					// Buffered readings, ten minutes apart, oldest first.
					observedAt: new Date(now - (COUNT - 1 - i) * 10 * 60_000).toISOString(),
					values: sampleValues(target.type),
					location: NO_FIX.has(i) ? null : points[i]
				};
				const [reading] = await services.labs.uploadReadings(target.id, [input], { credit });
				if (!live()) return;
				sent = [...sent, reading];
				await wait(700);
				if (!live()) return;
			}
			phase = 'done';
		} catch (e) {
			error = errorMessage(e);
			phase = 'error';
		}
		await onfinish();
	}

	async function stop() {
		const hadReadings = sent.length > 0;
		run++;
		phase = 'idle';
		sent = [];
		if (hadReadings) await onfinish();
	}
</script>

<section class="card" aria-labelledby="record-heading">
	<h2 id="record-heading" class="text-lg font-semibold">Record data</h2>
	<p class="mt-1 text-sm text-muted">
		Demo: pairs with {device && devices.length === 1 ? device.name : 'a registered sensor'} and records
		its readings to {classroomName}.
	</p>

	<div class="mt-4 flex flex-wrap items-end gap-3">
		{#if devices.length > 1}
			<div class="min-w-0 flex-1">
				<label class="label" for="record-device">Sensor</label>
				<select id="record-device" class="input" bind:value={deviceId} disabled={busy}>
					<option value="" disabled>Choose a sensor</option>
					{#each devices as d (d.id)}
						<option value={d.id}>{d.name}</option>
					{/each}
				</select>
			</div>
		{/if}
		{#if busy || phase === 'done'}
			<button type="button" class="btn btn-ghost" onclick={stop}>
				{busy ? 'Stop' : 'Done'}
			</button>
		{:else}
			<button type="button" class="btn btn-primary" disabled={!device} onclick={record}>
				Record data
			</button>
		{/if}
	</div>
	<label class="mt-3 flex items-start gap-2 text-sm">
		<input
			id="record-credit"
			type="checkbox"
			class="mt-0.5 rounded-sm border-line-strong bg-transparent text-teal focus:ring-teal"
			bind:checked={credit}
			disabled={busy}
		/>
		<span>
			Credit these readings to me
			<span class="block text-xs text-muted"
				>Your name is shown on them to people in {classroomName}. Leave it off to record them
				without your name.</span
			>
		</span>
	</label>

	{#if phase !== 'idle' && device}
		<ol class="mt-5 space-y-2 text-sm" aria-live="polite">
			<li class="flex items-center gap-2">
				<span aria-hidden="true">{phase === 'searching' ? '…' : '✓'}</span>
				{phase === 'searching' ? `Looking for ${device.identifier}…` : `Found ${device.identifier}`}
			</li>
			{#if phase !== 'searching'}
				<li class="flex items-center gap-2">
					<span aria-hidden="true">{phase === 'pairing' ? '…' : '✓'}</span>
					{phase === 'pairing' ? 'Pairing…' : 'Paired'}
				</li>
			{/if}
			{#if phase === 'streaming' || phase === 'done' || (phase === 'error' && sent.length)}
				<li class="flex items-center gap-2">
					<span aria-hidden="true">{phase === 'streaming' ? '…' : '✓'}</span>
					{phase === 'streaming' ? 'Receiving readings' : 'Readings received'}
					<span class="text-muted">{sent.length} of {COUNT}</span>
				</li>
			{/if}
		</ol>

		{#if sent.length}
			<ul class="mt-4 divide-y divide-line rounded-md border border-line text-xs">
				{#each sent as reading (reading.id)}
					<li class="space-y-1 p-3">
						<div class="flex flex-wrap items-center justify-between gap-2">
							<span class="font-medium">{time.format(new Date(reading.observedAt))}</span>
							<span
								class="rounded-full bg-lime/15 px-2 py-0.5 font-medium text-lime ring-1 ring-lime/40"
								>Recorded</span
							>
						</div>
						<p class="text-muted">{keySummary(target(reading).type, reading.values, paramsById)}</p>
						<p class="text-muted">
							{reading.creditedTo ? `Credited to ${reading.creditedTo}` : 'Not credited to anyone'}
						</p>
						<p>
							{formatPoint(reading.location)}
							{#if reading.locationSource === 'device'}
								<span class="text-muted">· device GPS</span>
							{:else}
								<span class="rounded-full bg-raised px-2 py-0.5 font-medium ring-1 ring-line"
									>No GPS fix: lab location, not from the device</span
								>
							{/if}
						</p>
					</li>
				{/each}
			</ul>
		{/if}

		{#if phase === 'done'}
			<p class="mt-3 text-sm" role="status">
				{COUNT} readings recorded. {NO_FIX.size} had no GPS and use the lab's location.
			</p>
		{/if}
		{#if error}<p class="field-error" role="alert">{error}</p>{/if}
	{/if}
</section>
