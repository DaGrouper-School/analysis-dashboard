<script lang="ts">
	import { resolve } from '$app/paths';
	import Globe, { type GlobeMarker } from '#lib/components/Globe.svelte';
	import type { DeviceType } from '#lib/domain/types.ts';
	import { formatPoint } from '#lib/geo.ts';
	import { formatValue, keyValues } from '#lib/key-values.ts';
	import { DEVICE_TYPE_COLORS, DEVICE_TYPE_LABELS } from '#lib/labels.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// svelte-ignore state_referenced_locally
	let selectedId = $state(
		data.conditions.find((c) => c.device.id === data.selected)?.device.id ??
			data.conditions.find((c) => c.device.type === 'weather_station')?.device.id ??
			data.conditions[0]?.device.id ??
			''
	);
	let query = $state('');
	let hiddenTypes = $state<DeviceType[]>([]);
	let globe = $state<ReturnType<typeof Globe>>();

	const selected = $derived(data.conditions.find((c) => c.device.id === selectedId) ?? null);
	const paramsById = $derived(new Map(data.parameters.map((p) => [p.id, p])));
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const classroomCount = $derived(new Set(data.conditions.map((c) => c.classroomName)).size);
	const types = $derived([...new Set(data.conditions.map((c) => c.device.type))]);

	const visible = $derived(
		data.conditions.filter((c) => {
			if (hiddenTypes.includes(c.device.type)) return false;
			const q = query.trim().toLowerCase();
			return (
				!q ||
				[c.device.name, c.device.labName, c.classroomName].some((t) => t.toLowerCase().includes(q))
			);
		})
	);

	const markers = $derived<GlobeMarker[]>(
		visible.map((c) => ({
			id: c.device.id,
			lat: c.reading.location.lat,
			lng: c.reading.location.lng,
			color: DEVICE_TYPE_COLORS[c.device.type],
			hollow: c.reading.locationSource === 'lab',
			label:
				c.reading.locationSource === 'lab'
					? `${c.device.name} · lab location, not from the device`
					: c.device.name
		}))
	);

	function toggleType(type: DeviceType) {
		hiddenTypes = hiddenTypes.includes(type)
			? hiddenTypes.filter((t) => t !== type)
			: [...hiddenTypes, type];
	}

	// Only the values that matter for the selected sensor's type: no rain on a soil sensor.
	const tiles = $derived(
		selected ? keyValues(selected.device.type, selected.reading.values, paramsById) : []
	);
</script>

<svelte:head><title>Map · DaGrouper Analysis</title></svelte:head>

{#if data.conditions.length}
	<!-- Full-bleed explorer: the globe fills the space, panels float over it on wide screens. -->
	<div
		class="relative left-1/2 -my-8 w-screen -translate-x-1/2 lg:h-[calc(100dvh-86px)] lg:min-h-[40rem]"
	>
		<div class="relative h-[62vh] min-h-96 lg:absolute lg:inset-0 lg:h-auto">
			<Globe
				bind:this={globe}
				{markers}
				bind:selectedId
				ariaLabel="Globe of your classrooms' devices"
			/>
			<div class="absolute right-4 bottom-4 flex flex-col border border-line-strong bg-canvas/80">
				<button
					type="button"
					class="grid size-10 place-items-center text-lg text-ink hover:bg-ink/10"
					aria-label="Zoom in"
					onclick={() => globe?.zoomBy(1.5)}>+</button
				>
				<button
					type="button"
					class="grid size-10 place-items-center border-t border-line-strong text-lg text-ink hover:bg-ink/10"
					aria-label="Zoom out"
					onclick={() => globe?.zoomBy(1 / 1.5)}>−</button
				>
			</div>
			<ul
				class="absolute bottom-4 left-1/2 hidden -translate-x-1/2 flex-wrap items-center gap-x-5 gap-y-1 border border-line bg-canvas/80 px-5 py-3 text-xs text-text md:flex"
				aria-label="Legend"
			>
				{#each types as type (type)}
					<li class="flex items-center gap-2">
						<span class="size-2 rounded-full" style:background={DEVICE_TYPE_COLORS[type]}></span>
						{DEVICE_TYPE_LABELS[type]}
					</li>
				{/each}
				<li class="flex items-center gap-2">
					<span class="size-2.5 rounded-full border border-dashed border-ink"></span>
					Lab location, not from the device
				</li>
			</ul>
		</div>

		<!-- Explorer panel -->
		<section
			class="mx-4 mt-4 flex flex-col border border-line bg-surface/95 lg:absolute lg:top-6 lg:bottom-6 lg:left-6 lg:m-0 lg:w-[22rem]"
			aria-label="Devices"
		>
			<div class="border-b border-line p-5">
				<p class="text-sm font-medium">
					<span class="text-cyan">DaGrouper</span> <span class="ml-1.5 text-ink">Map</span>
				</p>
				<h1 class="mt-2 text-2xl text-ink">Explore your devices</h1>
				<p class="mt-1 text-sm text-muted">
					{data.conditions.length} device{data.conditions.length === 1 ? '' : 's'} · {classroomCount}
					classroom{classroomCount === 1 ? '' : 's'}
				</p>
				<label class="sr-only" for="device-search">Search devices</label>
				<input
					id="device-search"
					class="input mt-4"
					type="search"
					placeholder="Device, lab or classroom"
					bind:value={query}
				/>
				<p class="mt-4 text-xs text-muted">Device type</p>
				<div class="mt-2 flex flex-wrap gap-2">
					{#each types as type (type)}
						{@const on = !hiddenTypes.includes(type)}
						<button
							type="button"
							class={['chip transition', on ? 'border-line-strong text-ink' : 'opacity-50']}
							aria-pressed={on}
							onclick={() => toggleType(type)}
						>
							<span
								class="size-2 rounded-full"
								style:background={on ? DEVICE_TYPE_COLORS[type] : 'transparent'}
								style:box-shadow="inset 0 0 0 1.5px {DEVICE_TYPE_COLORS[type]}"
							></span>
							{DEVICE_TYPE_LABELS[type]}
						</button>
					{/each}
				</div>
			</div>
			<ul class="min-h-0 flex-1 overflow-y-auto py-2">
				{#each visible as item (item.device.id)}
					<li>
						<button
							type="button"
							class={[
								'flex w-full items-start gap-3 px-5 py-3 text-left transition',
								item.device.id === selectedId ? 'bg-ink/[0.07]' : 'hover:bg-ink/[0.04]'
							]}
							aria-pressed={item.device.id === selectedId}
							onclick={() => (selectedId = item.device.id)}
						>
							<span
								class="mt-1.5 size-2 shrink-0 rounded-full"
								style:background={DEVICE_TYPE_COLORS[item.device.type]}
							></span>
							<span class="min-w-0 flex-1">
								<span class="block truncate text-ink">{item.device.name}</span>
								<span class="block text-xs text-muted">
									{item.device.labName} · {DEVICE_TYPE_LABELS[item.device.type]}
									{#if item.reading.locationSource === 'lab'}· lab location{/if}
								</span>
							</span>
							{#if item.alerts.length}
								<span class="mt-0.5 text-xs whitespace-nowrap text-orange">
									▲ {item.alerts.length} alert{item.alerts.length > 1 ? 's' : ''}
								</span>
							{/if}
						</button>
					</li>
				{:else}
					<li class="px-5 py-3 text-sm text-muted">No devices match.</li>
				{/each}
			</ul>
		</section>

		<!-- Selected device -->
		{#if selected}
			<section
				class="mx-4 mt-4 mb-8 border border-line bg-surface/95 p-5 lg:absolute lg:top-6 lg:right-6 lg:m-0 lg:max-h-[calc(100%-6rem)] lg:w-[20rem] lg:overflow-y-auto"
				aria-live="polite"
				aria-label="Current conditions"
			>
				<p class="text-xs text-muted">Current conditions</p>
				<h2 class="mt-1 text-xl">{selected.device.name}</h2>
				<p class="mt-1 text-sm text-muted">
					{selected.classroomName} ·
					<a
						class="underline-offset-2 hover:text-ink hover:underline"
						href={resolve('/(app)/labs/[labId]', { labId: selected.device.labId })}
						>{selected.device.labName}</a
					>
				</p>
				<p class="mt-3 text-xs text-muted">
					Observed {dateTime.format(new Date(selected.reading.observedAt))}
				</p>
				<p class="mt-1 text-xs">
					{formatPoint(selected.reading.location)}
					{#if selected.reading.locationSource === 'lab'}
						<span class="chip mt-1 border-dashed">Lab location, not from the device</span>
					{:else}
						<span class="text-muted">· device GPS</span>
					{/if}
				</p>
				<dl class="mt-5 grid grid-cols-2 gap-px border border-line bg-line">
					{#each tiles as item (item.id)}
						<div class="bg-surface p-3">
							<dt class="text-xs text-muted">{item.label}</dt>
							<dd class="mt-1 text-2xl text-ink">{formatValue(item.value, item.unit)}</dd>
						</div>
					{/each}
				</dl>

				<h3 class="mt-6 text-sm font-medium">Active alerts</h3>
				{#if selected.alerts.length}
					<ul class="mt-2 space-y-2">
						{#each selected.alerts as alert (alert.parameterId)}
							<li
								class={[
									'border-l-2 bg-ink/[0.03] py-2 pr-2 pl-3 text-sm',
									alert.level === 'warning' ? 'border-danger' : 'border-orange'
								]}
							>
								<p class="text-ink">
									<span aria-hidden="true">{alert.level === 'warning' ? '▲▲' : '▲'}</span>
									{alert.title}
									<span class="sr-only">({alert.level})</span>
								</p>
								<p class="text-muted">{alert.message}</p>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="mt-2 text-sm text-muted">No active alerts for this device.</p>
				{/if}
			</section>
		{/if}
	</div>
{:else}
	<div class="card mt-6 max-w-2xl">
		<h1 class="text-2xl">No devices to show</h1>
		<p class="mt-2 text-sm text-muted">
			The map shows devices in your classrooms' labs once they have sent a reading. Join a classroom
			with an invite link, or ask your facilitator to connect a device.
		</p>
		<a class="btn btn-ghost mt-4" href={resolve('/(app)/classrooms')}>Your classrooms</a>
	</div>
{/if}
