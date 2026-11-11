<script lang="ts">
	import { onMount } from 'svelte';
	import Select from '#lib/components/Select.svelte';
	import type { DialCode } from '#lib/domain/types.ts';

	/** A phone number, stored in E.164, with its country picked from `countries` (first is the default). */
	let {
		value = $bindable(''),
		countries,
		id = 'phone',
		error = '',
		disabled = false
	}: {
		value?: string;
		countries: DialCode[];
		id?: string;
		error?: string;
		disabled?: boolean;
	} = $props();

	// svelte-ignore state_referenced_locally
	let dial = $state(countries[0]?.dial ?? '');
	let national = $state('');

	onMount(() => {
		// Split an existing E.164 value back into country and number (longest prefix wins).
		const match = [...countries]
			.sort((a, b) => b.dial.length - a.dial.length)
			.find((c) => value.startsWith(c.dial));
		if (match) {
			dial = match.dial;
			national = value.slice(match.dial.length);
		}
	});

	function sync() {
		const digits = national.replace(/[^\d]/g, '').replace(/^0+/, '');
		value = digits ? `${dial}${digits}` : '';
	}
</script>

<div class="flex gap-2">
	<Select
		id="{id}-country"
		label="Country code"
		labelHidden
		class="w-36 shrink-0"
		options={countries.map((c) => ({ value: c.dial, label: `${c.name} ${c.dial}` }))}
		bind:value={dial}
		onchange={sync}
		error={Boolean(error)}
		{disabled}
	/>
	<input
		{id}
		class={['input min-w-0 flex-1', error && 'input-error']}
		type="tel"
		inputmode="tel"
		autocomplete="tel-national"
		placeholder="911 000 001"
		bind:value={national}
		oninput={sync}
		aria-invalid={Boolean(error)}
		aria-describedby={error ? `${id}-error` : undefined}
		{disabled}
	/>
</div>
{#if error}<p id="{id}-error" class="field-error">{error}</p>{/if}
