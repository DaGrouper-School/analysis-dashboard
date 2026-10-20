<script lang="ts">
	import type { GeoPoint } from '#lib/domain/types.ts';

	/** Latitude/longitude as text so partial input stays editable; parse with `toGeoPoint`. */
	let {
		lat = $bindable(''),
		lng = $bindable(''),
		id,
		error = '',
		copyFrom = null,
		copyLabel = 'Same as lab'
	}: {
		lat?: string;
		lng?: string;
		id: string;
		error?: string;
		copyFrom?: GeoPoint | null;
		copyLabel?: string;
	} = $props();

	let locating = $state(false);
	let geoError = $state('');

	function useCurrentLocation() {
		geoError = '';
		if (!('geolocation' in navigator)) {
			geoError = 'This browser cannot share its location.';
			return;
		}
		locating = true;
		navigator.geolocation.getCurrentPosition(
			(position) => {
				lat = position.coords.latitude.toFixed(5);
				lng = position.coords.longitude.toFixed(5);
				locating = false;
			},
			() => {
				geoError = 'Could not get your location. Enter it by hand.';
				locating = false;
			},
			{ enableHighAccuracy: true, timeout: 10_000 }
		);
	}
</script>

<div class="grid grid-cols-2 gap-3">
	<div>
		<label class="label" for="{id}-lat">Latitude</label>
		<input
			id="{id}-lat"
			class={['input', error && 'input-error']}
			inputmode="decimal"
			placeholder="7.2000"
			bind:value={lat}
		/>
	</div>
	<div>
		<label class="label" for="{id}-lng">Longitude</label>
		<input
			id="{id}-lng"
			class={['input', error && 'input-error']}
			inputmode="decimal"
			placeholder="38.6000"
			bind:value={lng}
		/>
	</div>
</div>
<div class="mt-2 flex flex-wrap gap-2">
	<button type="button" class="btn btn-ghost px-3 py-1.5 text-xs" onclick={useCurrentLocation}>
		{locating ? 'Locating…' : 'Use my current location'}
	</button>
	{#if copyFrom}
		<button
			type="button"
			class="btn btn-ghost px-3 py-1.5 text-xs"
			onclick={() => {
				lat = String(copyFrom.lat);
				lng = String(copyFrom.lng);
			}}>{copyLabel}</button
		>
	{/if}
</div>
{#if error || geoError}<p class="field-error">{error || geoError}</p>{/if}
