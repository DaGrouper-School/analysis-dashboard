<script lang="ts">
	import type {
		Device,
		DeviceType,
		DeviceTypeDefinition,
		DomainDefinition,
		Parameter,
		RegisterDeviceInput
	} from '#lib/domain/types.ts';
	import { validateDevice } from '#lib/domain/validation.ts';
	import Icon from '#lib/components/Icon.svelte';
	import Modal from '#lib/components/Modal.svelte';
	import PairingVisual, { type PairingState } from '#lib/components/PairingVisual.svelte';
	import StepTrack from '#lib/components/StepTrack.svelte';
	import { deviceTypeColor, deviceTypeIcon } from '#lib/labels.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';

	/**
	 * Register a sensor to a lab as a step-by-step modal: pick its type, identify it, pair (the
	 * device is registered in SAM as a source), done. `onclose` runs when the modal closes, so
	 * the page can show the new devices.
	 */
	let {
		open = $bindable(false),
		labId,
		labName,
		deviceTypes,
		domains,
		parameters,
		onregistered,
		onclose
	}: {
		open?: boolean;
		labId: string;
		labName: string;
		deviceTypes: DeviceTypeDefinition[];
		domains: DomainDefinition[];
		parameters: Parameter[];
		onregistered?: (device: Device) => unknown;
		onclose: () => unknown;
	} = $props();

	type Phase = 'type' | 'identify' | 'scanning' | 'handshake' | 'paired' | 'error';
	const STEPS = [
		{ label: 'Type', icon: 'sensor' },
		{ label: 'Identify', icon: 'tag' },
		{ label: 'Pair', icon: 'radar' },
		{ label: 'Paired', icon: 'check' }
	];
	const STEP_OF: Record<Phase, number> = {
		type: 0,
		identify: 1,
		scanning: 2,
		handshake: 2,
		paired: 4,
		error: 2
	};
	const VISUAL: Record<Phase, PairingState> = {
		type: 'idle',
		identify: 'idle',
		scanning: 'scan',
		handshake: 'handshake',
		paired: 'done',
		error: 'error'
	};

	let phase = $state<Phase>('type');
	// svelte-ignore state_referenced_locally
	let type = $state<DeviceType>(deviceTypes[0]?.id ?? '');
	let name = $state('');
	let identifier = $state('');
	let errors = $state<Record<string, string>>({});
	let error = $state('');
	let registered = $state<Device | null>(null);
	let run = 0;

	const busy = $derived(phase === 'scanning' || phase === 'handshake');
	const color = $derived(deviceTypeColor(deviceTypes, type));
	const icon = $derived(deviceTypeIcon(deviceTypes, type));
	const status = $derived(
		{
			type: 'Standby',
			identify: 'Standby',
			scanning: 'Scanning',
			handshake: 'Handshake',
			paired: 'Paired',
			error: 'No link'
		}[phase]
	);

	// What the picked type sends, and the domains those values show under (SAM-23).
	const typeValues = $derived.by(() => {
		const ids = deviceTypes.find((t) => t.id === type)?.parameterIds ?? [];
		return ids.map((id) => parameters.find((p) => p.id === id) ?? { id, label: id, domain: null });
	});
	const typeDomains = $derived(domains.filter((d) => typeValues.some((v) => v.domain === d.id)));
	const typeUnmapped = $derived(
		typeValues.filter((v) => !domains.some((d) => d.id === v.domain)).length
	);

	const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));

	function next(event: SubmitEvent) {
		event.preventDefault();
		if (phase === 'type') {
			phase = 'identify';
			return;
		}
		pair();
	}

	async function pair() {
		const input: RegisterDeviceInput = { name, type, identifier };
		errors = validateDevice(input);
		if (Object.keys(errors).length) return;
		const id = ++run;
		const live = () => id === run;
		error = '';
		phase = 'scanning';
		await wait(1500);
		if (!live()) return;
		phase = 'handshake';
		try {
			// Registering is the handshake: the device becomes a SAM source for this lab.
			const [device] = await Promise.all([services.labs.registerDevice(labId, input), wait(1300)]);
			if (!live()) return;
			registered = device;
			phase = 'paired';
			await onregistered?.(device);
		} catch (e) {
			if (!live()) return;
			if (e instanceof ServiceError && Object.keys(e.fields).length) {
				// A field problem (e.g. the identifier is taken): back to the form to fix it.
				errors = e.fields;
				phase = e.fields.type ? 'type' : 'identify';
				return;
			}
			error = errorMessage(e);
			phase = 'error';
		}
	}

	function another() {
		run++;
		name = identifier = '';
		errors = {};
		error = '';
		registered = null;
		phase = 'type';
	}

	function closed() {
		another();
		onclose();
	}
</script>

<Modal bind:open title="Register a sensor" eyebrow={labName} onclose={closed}>
	<form id="register-sensor" class="space-y-5" style:--accent={color} onsubmit={next} novalidate>
		<StepTrack steps={STEPS} current={STEP_OF[phase]} failed={phase === 'error'} />

		{#if phase === 'type'}
			<fieldset>
				<legend class="label">What kind of sensor is it?</legend>
				<div class="grid gap-2 sm:grid-cols-2">
					{#each deviceTypes as option (option.id)}
						<label class="cursor-pointer">
							<input
								type="radio"
								class="peer sr-only"
								name="sensor-type"
								value={option.id}
								bind:group={type}
							/>
							<span
								class="flex h-full items-start gap-3 rounded-md border border-line p-3 transition peer-checked:border-(--type) peer-checked:bg-(--type)/10 peer-focus-visible:ring-2 peer-focus-visible:ring-teal hover:border-line-strong"
								style:--type={option.color}
							>
								<span
									class="grid size-9 shrink-0 place-items-center rounded-md"
									style:color={option.color}
									style:background="color-mix(in oklab, {option.color} 15%, transparent)"
								>
									<Icon name={option.icon} />
								</span>
								<span class="min-w-0">
									<span class="block text-sm font-medium text-ink">{option.label}</span>
									<span class="block text-xs text-muted">{option.description}</span>
								</span>
							</span>
						</label>
					{/each}
				</div>
				{#if errors.type}<p class="field-error">{errors.type}</p>{/if}
			</fieldset>
			<div class="rounded-md border border-line p-3 text-sm">
				{#if typeValues.length}
					<p class="text-muted">
						Sends {typeValues.length} value{typeValues.length === 1 ? '' : 's'}:
						{typeValues
							.slice(0, 6)
							.map((v) => v.label)
							.join(', ')}{typeValues.length > 6 ? ', …' : ''}
					</p>
					<p class="mt-2 flex flex-wrap items-center gap-2">
						<span class="text-muted">Shows under</span>
						{#each typeDomains as domain (domain.id)}
							<span class="chip"
								><Icon name={domain.icon} fallback="tag" class="size-3.5" /> {domain.label}</span
							>
						{/each}
						{#if typeUnmapped}
							<span class="chip text-muted"
								>{typeUnmapped} value{typeUnmapped === 1 ? '' : 's'} not in a domain yet</span
							>
						{/if}
					</p>
				{:else}
					<p class="text-muted">This type has no expected values; it shows whatever it sends.</p>
				{/if}
			</div>
		{:else if phase === 'identify'}
			<PairingVisual
				state="idle"
				{icon}
				identifier={identifier.trim().toUpperCase() || undefined}
				{status}
			/>
			<p class="text-sm text-muted">
				A device can sit far from the lab, like a weather station in another town feeding this lab.
				Its GPS position is sent with each reading, not set here.
			</p>
			<div>
				<label class="label" for="sensor-name">Device name</label>
				<input
					id="sensor-name"
					class={['input', errors.name && 'input-error']}
					placeholder="Shashamane Weather Station"
					bind:value={name}
				/>
				{#if errors.name}<p class="field-error">{errors.name}</p>{/if}
			</div>
			<div>
				<label class="label" for="sensor-identifier">Identifier</label>
				<input
					id="sensor-identifier"
					class={['input font-mono', errors.identifier && 'input-error']}
					placeholder="TWS-2041-SH"
					bind:value={identifier}
				/>
				{#if errors.identifier}
					<p class="field-error">{errors.identifier}</p>
				{:else}
					<p class="mt-1.5 text-xs text-muted">The serial printed on the sensor.</p>
				{/if}
			</div>
		{:else}
			<PairingVisual
				state={VISUAL[phase]}
				{icon}
				identifier={registered?.identifier ?? identifier.trim().toUpperCase()}
				{status}
			/>
			<div aria-live="polite">
				{#if phase === 'scanning'}
					<p class="text-sm text-ink">Scanning for {identifier.trim().toUpperCase()}…</p>
					<p class="text-sm text-muted">Switch the sensor on and hold it near this device.</p>
				{:else if phase === 'handshake'}
					<p class="text-sm text-ink">Found it. Handshaking and registering the SAM source…</p>
				{:else if phase === 'paired' && registered}
					<p class="text-sm text-ink" role="status">{registered.name} is paired with {labName}.</p>
					{#if registered.samSourceId}
						<p class="mt-1 text-sm text-muted">
							SAM source <span class="font-mono break-all text-ink">{registered.samSourceId}</span>
						</p>
					{/if}
					<p class="mt-1 text-sm text-muted">Sync it to record its first readings.</p>
				{:else if phase === 'error'}
					<p class="field-error mt-0" role="alert">{error}</p>
				{/if}
			</div>
		{/if}
	</form>

	{#snippet footer()}
		{#if phase === 'type'}
			<button type="button" class="btn btn-ghost" onclick={() => (open = false)}>Cancel</button>
			<button form="register-sensor" class="btn btn-primary" disabled={!type}>
				Next <Icon name="arrow" class="size-4" />
			</button>
		{:else if phase === 'identify'}
			<button type="button" class="btn btn-ghost" onclick={() => (phase = 'type')}>Back</button>
			<button form="register-sensor" class="btn btn-primary">
				<Icon name="radar" class="size-4" /> Pair sensor
			</button>
		{:else if busy}
			<button
				type="button"
				class="btn btn-ghost"
				onclick={() => {
					run++;
					phase = 'identify';
				}}>Stop</button
			>
		{:else if phase === 'error'}
			<button type="button" class="btn btn-ghost" onclick={() => (phase = 'identify')}>Back</button>
			<button type="button" class="btn btn-primary" onclick={pair}>Try again</button>
		{:else}
			<button type="button" class="btn btn-ghost" onclick={another}>
				<Icon name="plus" class="size-4" /> Register another
			</button>
			<button type="button" class="btn btn-primary" onclick={() => (open = false)}>Done</button>
		{/if}
	{/snippet}
</Modal>
