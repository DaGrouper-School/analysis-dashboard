<script lang="ts">
	import { onMount } from 'svelte';
	import Select from '#lib/components/Select.svelte';

	/** Pilot countries first; the number is stored in E.164. */
	const COUNTRIES = [
		{ code: 'ET', name: 'Ethiopia', dial: '+251' },
		{ code: 'BS', name: 'Bahamas', dial: '+1242' },
		{ code: 'SO', name: 'Somalia', dial: '+252' },
		{ code: 'JM', name: 'Jamaica', dial: '+1876' },
		{ code: 'US', name: 'United States', dial: '+1' },
		{ code: 'GB', name: 'United Kingdom', dial: '+44' },
		{ code: 'KE', name: 'Kenya', dial: '+254' }
	];

	let {
		value = $bindable(''),
		id = 'phone',
		error = '',
		disabled = false
	}: { value?: string; id?: string; error?: string; disabled?: boolean } = $props();

	let dial = $state(COUNTRIES[0].dial);
	let national = $state('');

	onMount(() => {
		// Split an existing E.164 value back into country and number (longest prefix wins).
		const match = [...COUNTRIES]
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
		class="w-36 shrink-0"
		options={COUNTRIES.map((c) => ({ value: c.dial, label: `${c.name} ${c.dial}` }))}
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
