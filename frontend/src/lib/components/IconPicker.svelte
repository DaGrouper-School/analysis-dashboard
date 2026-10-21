<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import { iconLabel, REGISTRY_ICON_NAMES } from '#lib/icons.ts';

	/** Picks a registry icon for a device type or domain. Tinted with `color` when given. */
	let {
		value = $bindable(''),
		name,
		legend = 'Icon',
		color,
		error
	}: {
		value?: string;
		/** Radio group name, unique on the page. */
		name: string;
		legend?: string;
		color?: string;
		error?: string;
	} = $props();
</script>

<fieldset>
	<legend class="label">{legend}</legend>
	<div class="flex flex-wrap gap-1.5" style:--pick={color ?? 'var(--color-ink)'}>
		{#each REGISTRY_ICON_NAMES as icon (icon)}
			<label class="cursor-pointer" title={iconLabel(icon)}>
				<input type="radio" class="peer sr-only" {name} value={icon} bind:group={value} />
				<span
					class="grid size-9 place-items-center rounded-md border border-line text-muted transition peer-checked:border-(--pick) peer-checked:bg-(--pick)/15 peer-checked:text-(--pick) peer-focus-visible:ring-2 peer-focus-visible:ring-teal hover:border-line-strong hover:text-ink"
				>
					<Icon name={icon} />
				</span>
				<span class="sr-only">{iconLabel(icon)}</span>
			</label>
		{/each}
	</div>
	{#if error}<p class="field-error">{error}</p>{/if}
</fieldset>
