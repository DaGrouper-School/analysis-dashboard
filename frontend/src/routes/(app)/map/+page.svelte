<script lang="ts">
	import { resolve } from '$app/paths';
	import Icon from '#lib/components/Icon.svelte';
	import Globe, { type GlobeMarker } from '#lib/components/Globe.svelte';
	import RollingNumber from '#lib/components/RollingNumber.svelte';
	import type { DeviceType } from '#lib/domain/types.ts';
	import { domainOf } from '#lib/domains.ts';
	import { formatPoint } from '#lib/geo.ts';
	import { formatValue, keyValues } from '#lib/key-values.ts';
	import { deviceTypeColor, deviceTypeIcon, deviceTypeLabel } from '#lib/labels.ts';
	import { reveal } from '#lib/reveal.ts';
	import { domainQuery } from '#lib/sensor-selection.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// Opened from the dashboard: show that classroom's devices, with its picked sensor selected.
	// svelte-ignore state_referenced_locally
	let classroomId = $state(
		data.conditions.some((c) => c.reading.classroomId === data.classroom) ? data.classroom : null
	);
	// svelte-ignore state_referenced_locally
	let selectedId = $state(
		data.conditions.find((c) => c.device.id === data.selected)?.device.id ??
			data.conditions.find((c) => c.reading.classroomId === classroomId)?.device.id ??
			data.conditions.find((c) => c.device.type === 'weather_station')?.device.id ??
			data.conditions[0]?.device.id ??
			''
	);
	let query = $state('');
	let hiddenTypes = $state<DeviceType[]>([]);
	let globe = $state<ReturnType<typeof Globe>>();

	const selected = $derived(data.conditions.find((c) => c.device.id === selectedId) ?? null);

	// Picking a sensor, in the list or on the globe, brings its values into view: below the
	// globe on small screens, and back to the top of the floating panel on wide ones.
	let panel = $state<HTMLElement>();
	// svelte-ignore state_referenced_locally
	let shownId = selectedId;
	$effect(() => {
		if (!panel || selectedId === shownId) return;
		shownId = selectedId;
		const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
		panel.scrollTo({ top: 0, behavior: reduce ? 'auto' : 'smooth' });
		reveal(panel);
	});
	const paramsById = $derived(new Map(data.parameters.map((p) => [p.id, p])));
	const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
	const classroomName = $derived(
		data.conditions.find((c) => c.reading.classroomId === classroomId)?.classroomName ?? null
	);
	const classroomCount = $derived(new Set(data.conditions.map((c) => c.classroomName)).size);
	const types = $derived([...new Set(data.conditions.map((c) => c.device.type))]);

	const visible = $derived(
		data.conditions.filter((c) => {
			if (classroomId && c.reading.classroomId !== classroomId) return false;
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
			color: deviceTypeColor(data.deviceTypes, c.device.type),
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

	/** The selected sensor's domain view, opened on one alert's value when given. */
	function domainsHref(parameterId?: string) {
		const { device, reading } = selected!;
		const label = parameterId ? (paramsById.get(parameterId)?.label ?? parameterId) : '';
		return (
			resolve('/(app)/dashboard/domains') +
			domainQuery(reading.classroomId, device.id, {
				domain: parameterId ? domainOf(parameterId, paramsById, data.domains) : null,
				search: label
			})
		);
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
					<li
						class="flex items-center gap-2"
						style:--type={deviceTypeColor(data.deviceTypes, type)}
					>
						<span class="size-2 rounded-full bg-(--type)"></span>
						<Icon name={deviceTypeIcon(data.deviceTypes, type)} class="size-4 text-(--type)" />
						{deviceTypeLabel(data.deviceTypes, type)}
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
				{#if classroomName}
					<button
						type="button"
						class="chip mt-3 border-line-strong text-ink"
						aria-label="Show every classroom"
						onclick={() => (classroomId = null)}
					>
						{classroomName}
						<span aria-hidden="true" class="text-muted">×</span>
					</button>
				{/if}
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
							<span style:color={on ? deviceTypeColor(data.deviceTypes, type) : undefined}
								><Icon name={deviceTypeIcon(data.deviceTypes, type)} class="size-3.5" /></span
							>
							{deviceTypeLabel(data.deviceTypes, type)}
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
								class="grid size-8 shrink-0 place-items-center rounded-md"
								style:color={deviceTypeColor(data.deviceTypes, item.device.type)}
								style:background="color-mix(in oklab, {deviceTypeColor(
									data.deviceTypes,
									item.device.type
								)} 15%, transparent)"
							>
								<Icon name={deviceTypeIcon(data.deviceTypes, item.device.type)} class="size-4" />
							</span>
							<span class="min-w-0 flex-1">
								<span class="block truncate text-ink">{item.device.name}</span>
								<span class="block text-xs text-muted">
									{item.device.labName} · {deviceTypeLabel(data.deviceTypes, item.device.type)}
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
				bind:this={panel}
				class="accent-panel mx-4 mt-4 mb-8 scroll-mt-24 border p-5 lg:absolute lg:top-6 lg:right-6 lg:m-0 lg:max-h-[calc(100%-6rem)] lg:w-[20rem] lg:overflow-y-auto"
				style:--accent={deviceTypeColor(data.deviceTypes, selected.device.type)}
				aria-live="polite"
				aria-label="Current conditions"
			>
				<div class="flex items-start justify-between gap-3">
					<div class="min-w-0">
						<p class="text-xs text-muted">Current conditions</p>
						<h2 class="mt-1 flex items-center gap-2 text-xl">
							<Icon
								name={deviceTypeIcon(data.deviceTypes, selected.device.type)}
								class="size-6 shrink-0 text-(--accent)"
							/>
							<span class="min-w-0 break-words">{selected.device.name}</span>
						</h2>
					</div>
					<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
					<a href={domainsHref()} class="btn btn-ghost shrink-0 px-3 py-1.5 text-sm">View domains</a
					>
				</div>
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
				<!-- One value fills the row; an odd last value spans both columns. No empty cells. -->
				<dl
					class={[
						'accent-tiles mt-5 grid gap-px border',
						tiles.length > 1 && 'grid-cols-2 [&>:last-child:nth-child(odd)]:col-span-2'
					]}
				>
					{#each tiles as item, i (i)}
						<div class="accent-tile min-w-0 p-3">
							<dt class="text-xs text-muted">{item.label}</dt>
							<dd class="mt-1 text-2xl break-words text-ink">
								<RollingNumber value={formatValue(item.value, item.unit, item.valueType)} />
							</dd>
						</div>
					{/each}
				</dl>

				<h3 class="mt-6 text-sm font-medium">Active alerts</h3>
				{#if selected.alerts.length}
					<ul class="mt-2 space-y-2">
						{#each selected.alerts as alert (alert.parameterId)}
							<li>
								<!-- eslint-disable svelte/no-navigation-without-resolve -->
								<a
									href={domainsHref(alert.parameterId)}
									class={[
										'block border-l-2 bg-ink/[0.03] py-2 pr-2 pl-3 text-sm hover:bg-ink/[0.08]',
										alert.level === 'warning' ? 'border-danger' : 'border-orange'
									]}
								>
									<p class="text-ink">
										<span aria-hidden="true">{alert.level === 'warning' ? '▲▲' : '▲'}</span>
										{alert.title}
										<span class="sr-only">({alert.level})</span>
									</p>
									<p class="text-muted">{alert.message}</p>
								</a>
								<!-- eslint-enable svelte/no-navigation-without-resolve -->
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
