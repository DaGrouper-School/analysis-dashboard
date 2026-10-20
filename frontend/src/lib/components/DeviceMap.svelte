<script lang="ts">
	import 'leaflet/dist/leaflet.css';
	import type { CircleMarker, Map as LeafletMap } from 'leaflet';
	import { onMount } from 'svelte';
	import type { CurrentConditions } from '#lib/domain/types.ts';

	/**
	 * Devices at the position of their latest reading. A reading without GPS sits at the lab
	 * and gets a hollow, dashed marker so it never passes for the device's own position.
	 */
	let {
		items,
		selectedId = $bindable('')
	}: {
		items: CurrentConditions[];
		selectedId?: string;
	} = $props();

	let container: HTMLDivElement;
	let map: LeafletMap | null = null;
	// Leaflet objects, not UI state: a plain lookup, outside Svelte's reactivity.
	let markers: Record<string, CircleMarker> = {};

	const FOREST = '#1f3b2a';
	const LIME = '#c5f04a';

	function style(item: CurrentConditions, selected: boolean) {
		const fromDevice = item.reading.locationSource === 'device';
		return {
			radius: selected ? 12 : 9,
			color: FOREST,
			weight: selected ? 4 : 2,
			dashArray: fromDevice ? undefined : '4 4',
			fillColor: fromDevice ? (item.alerts.length ? '#f2a33a' : LIME) : '#ffffff',
			fillOpacity: fromDevice ? 0.95 : 0.7
		};
	}

	onMount(() => {
		let cancelled = false;
		(async () => {
			const L = await import('leaflet');
			if (cancelled) return;
			map = L.map(container, { scrollWheelZoom: false, worldCopyJump: true });
			L.tileLayer('https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png', {
				attribution:
					'&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>',
				maxZoom: 18
			}).addTo(map);
			for (const item of items) {
				const { lat, lng } = item.reading.location;
				const marker = L.circleMarker([lat, lng], style(item, item.device.id === selectedId))
					.addTo(map)
					.bindTooltip(
						item.reading.locationSource === 'device'
							? item.device.name
							: `${item.device.name} · lab location, not from the device`
					)
					.on('click', () => (selectedId = item.device.id));
				marker.getElement()?.setAttribute('data-device', item.device.id);
				markers[item.device.id] = marker;
			}
			const points = items.map(
				(i) => [i.reading.location.lat, i.reading.location.lng] as [number, number]
			);
			if (points.length) map.fitBounds(L.latLngBounds(points), { padding: [40, 40], maxZoom: 11 });
			else map.setView([10, 20], 2);
		})();
		return () => {
			cancelled = true;
			map?.remove();
			map = null;
			markers = {};
		};
	});

	// Restyle markers when the selection changes, and bring the selected one into view.
	$effect(() => {
		const id = selectedId;
		for (const item of items) {
			const marker = markers[item.device.id];
			if (!marker) continue;
			marker.setStyle(style(item, item.device.id === id));
			if (item.device.id === id) {
				marker.bringToFront();
				if (map && !map.getBounds().contains(marker.getLatLng())) map.panTo(marker.getLatLng());
			}
		}
	});
</script>

<div
	bind:this={container}
	class="h-[55vh] min-h-80 w-full overflow-hidden rounded-3xl border border-line bg-canvas lg:h-[36rem]"
	role="region"
	aria-label="Map of your classrooms' devices"
></div>
