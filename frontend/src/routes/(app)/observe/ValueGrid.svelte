<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { ClassValue } from 'svelte/elements';
	import type { Parameter } from '#lib/domain/types.ts';
	import { formatValue } from '#lib/features/measurements/key-values.ts';

	/**
	 * An observation's values as a grid of labelled cells. `before` and `after` add more cells
	 * around them, written as `<div class="bg-surface p-3">` with a `dt` and `dd`.
	 */
	let {
		values,
		parameters,
		class: className,
		before,
		after
	}: {
		values: Record<string, number | string>;
		parameters: Map<string, Parameter>;
		class?: ClassValue;
		before?: Snippet;
		after?: Snippet;
	} = $props();
</script>

<dl
	class={[
		'grid gap-px border border-line bg-line sm:grid-cols-2 sm:[&>:last-child:nth-child(odd)]:col-span-2',
		className
	]}
>
	{@render before?.()}
	{#each Object.entries(values) as [id, value] (id)}
		{@const parameter = parameters.get(id)}
		<div class="bg-surface p-3">
			<dt class="text-xs text-muted">{parameter?.label ?? id}</dt>
			<dd class="mt-1 text-lg text-ink">
				{formatValue(value, parameter?.unit, parameter?.valueType)}
			</dd>
		</div>
	{/each}
	{@render after?.()}
</dl>
