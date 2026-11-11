<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import Select, { toOptions } from '#lib/components/Select.svelte';
	import type {
		GeoPoint,
		ObservationTarget,
		ObservationTemplate,
		Parameter
	} from '#lib/domain/types.ts';
	import ParticipantIdNote from '#lib/features/consent/ParticipantIdNote.svelte';
	import { formatPoint } from '#lib/features/map/geo.ts';
	import { rangeText, unitsFor, type FieldEntry } from '#lib/features/measurements/observations.ts';
	import { localNow, STEP_INFO, type GpsState } from './observe-form.ts';

	/** The observation form's values step: each field in its unit, then when and where. */
	let {
		template,
		target,
		parameters,
		entries = $bindable(),
		touched = $bindable(),
		observedAt = $bindable(),
		location = $bindable(),
		gps = $bindable(),
		errorOf,
		timeError,
		serverErrors,
		tried
	}: {
		template: ObservationTemplate;
		target: ObservationTarget;
		parameters: Map<string, Parameter>;
		entries: Record<string, FieldEntry>;
		/** Fields the person has left, so their errors show. */
		touched: Record<string, boolean>;
		observedAt: string;
		location: GeoPoint | null;
		gps: GpsState;
		/** The error to show for a field, if any. */
		errorOf: (parameterId: string) => string | undefined;
		timeError: string;
		serverErrors: Record<string, string>;
		/** Next was tried: show every error. */
		tried: boolean;
	} = $props();

	function locate() {
		if (!('geolocation' in navigator)) {
			gps = 'none';
			return;
		}
		gps = 'locating';
		navigator.geolocation.getCurrentPosition(
			(position) => {
				location = {
					lat: Math.round(position.coords.latitude * 1e5) / 1e5,
					lng: Math.round(position.coords.longitude * 1e5) / 1e5
				};
				gps = 'fix';
			},
			() => {
				location = null;
				gps = 'none';
			},
			{ enableHighAccuracy: true, timeout: 10_000 }
		);
	}
</script>

<div class="space-y-8">
	<fieldset>
		<legend class="flex items-center gap-2 text-lg font-semibold text-ink">
			<Icon name={STEP_INFO.values.icon} class="size-5 text-muted" /> Your values
		</legend>
		<div class="mt-3 grid gap-5 sm:grid-cols-2">
			{#each template.fields as field (field.parameterId)}
				{@const param = parameters.get(field.parameterId)}
				{@const error = errorOf(field.parameterId)}
				{@const entry = entries[field.parameterId]}
				{#if param && entry}
					{@const units = unitsFor(param)}
					{@const range = rangeText(param)}
					<div>
						<label class="label" for="field-{param.id}">
							{param.label}
							{#if !field.required}<span class="font-normal text-muted">(optional)</span>{/if}
						</label>
						<div class="flex gap-2">
							{#if param.options}
								<Select
									id="field-{param.id}"
									options={toOptions(param.options)}
									bind:value={entry.value}
									onblur={() => (touched[param.id] = true)}
									error={!!error}
									aria-describedby="help-{param.id}"
								/>
							{:else}
								<input
									id="field-{param.id}"
									class={['input min-w-0', error && 'input-error']}
									inputmode={(param.valueType ?? 'number') === 'number' ? 'decimal' : 'text'}
									autocomplete="off"
									bind:value={entry.value}
									onblur={() => (touched[param.id] = true)}
									aria-invalid={!!error}
									aria-describedby="help-{param.id}"
								/>
							{/if}
							{#if units.length > 1}
								<Select
									size="mini"
									align="end"
									class="w-24 shrink-0"
									label="{param.label} unit"
									options={toOptions(units)}
									bind:value={entry.unit}
								/>
							{:else if param.unit}
								<span class="self-center text-sm text-muted">{param.unit}</span>
							{/if}
						</div>
						<p id="help-{param.id}" class="mt-1.5 text-xs text-muted">
							{param.definition}{range ? ` Expected ${range}.` : ''}
						</p>
						{#if error}<p class="field-error">{error}</p>{/if}
					</div>
				{/if}
			{/each}
		</div>
	</fieldset>

	<div class="grid gap-5 sm:grid-cols-2">
		<div>
			<label class="label" for="observe-time">When</label>
			<input
				id="observe-time"
				type="datetime-local"
				class={['input', tried && timeError && 'input-error']}
				max={localNow()}
				bind:value={observedAt}
			/>
			{#if (tried && timeError) || serverErrors.observedAt}
				<p class="field-error">{serverErrors.observedAt ?? timeError}</p>
			{/if}
		</div>
		<div>
			<p class="label">Where</p>
			<div class="flex flex-wrap items-center gap-2">
				<button type="button" class="btn btn-ghost" onclick={locate} disabled={gps === 'locating'}>
					{gps === 'locating' ? 'Finding you…' : gps === 'fix' ? 'Update GPS' : 'Use my GPS'}
				</button>
				{#if gps === 'fix' && location}
					<span class="text-sm text-ink">{formatPoint(location)}</span>
				{/if}
			</div>
			<p class="mt-1.5 text-xs text-muted" aria-live="polite">
				{gps === 'fix'
					? 'Sent with the observation.'
					: `${gps === 'none' ? 'No GPS fix. ' : ''}Without GPS, ${target.lab.name}'s location is used and flagged as not from the device.`}
			</p>
		</div>
	</div>

	<ParticipantIdNote />
</div>
