<script lang="ts">
	import { iconShapes, type RegistryIconName } from '#lib/icons.ts';

	/**
	 * A line icon in the current text color. `name` is a registry icon (stored on a device type
	 * or domain) or a UI icon; an unknown name draws `fallback`. Decorative unless `label` is set.
	 */
	let {
		name,
		fallback = 'sensor',
		label,
		class: className = 'size-5'
	}: {
		name: string;
		fallback?: RegistryIconName;
		label?: string;
		class?: string;
	} = $props();

	const shapes = $derived(iconShapes(name, fallback));
</script>

<svg
	viewBox="0 0 24 24"
	class={['shrink-0', className]}
	fill="none"
	stroke="currentColor"
	stroke-width="1.75"
	stroke-linecap="round"
	stroke-linejoin="round"
	role={label ? 'img' : undefined}
	aria-label={label}
	aria-hidden={label ? undefined : 'true'}
>
	{#each shapes as shape, i (i)}
		{#if 'd' in shape}
			<path d={shape.d} />
		{:else if 'r' in shape}
			<circle cx={shape.cx} cy={shape.cy} r={shape.r} />
		{:else}
			<rect x={shape.x} y={shape.y} width={shape.width} height={shape.height} rx={shape.rx} />
		{/if}
	{/each}
</svg>
