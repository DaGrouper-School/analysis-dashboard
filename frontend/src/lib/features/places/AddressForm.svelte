<script lang="ts">
	import type { Address, PlaceSuggestion } from '#lib/domain/types.ts';
	import { validateAddress } from '#lib/domain/validation.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';

	let {
		initial,
		onSave,
		submitLabel = 'Save address'
	}: {
		initial: Address | null;
		onSave: (address: Address) => Promise<void>;
		submitLabel?: string;
	} = $props();

	const empty: Address = { street: '', suite: '', city: '', state: '', zip: '', country: '' };
	// svelte-ignore state_referenced_locally
	let address = $state<Address>({ ...empty, ...initial });

	let query = $state('');
	let suggestions = $state<PlaceSuggestion[]>([]);
	let active = $state(-1);
	let searching = $state(false);
	let errors = $state<Record<string, string>>({});
	let formError = $state('');
	let saving = $state(false);
	let debounce: ReturnType<typeof setTimeout>;
	let latestQuery = '';

	function search() {
		clearTimeout(debounce);
		active = -1;
		debounce = setTimeout(async () => {
			latestQuery = query;
			searching = true;
			const results = await services.places.autocomplete(query).catch(() => []);
			if (latestQuery === query) suggestions = results;
			searching = false;
		}, 200);
	}

	async function pick(suggestion: PlaceSuggestion) {
		const place = await services.places.getAddress(suggestion.placeId);
		address = { ...empty, ...place, suite: address.suite };
		query = '';
		suggestions = [];
		errors = {};
	}

	function onKeydown(event: KeyboardEvent) {
		if (!suggestions.length) return;
		if (event.key === 'ArrowDown') active = (active + 1) % suggestions.length;
		else if (event.key === 'ArrowUp')
			active = (active - 1 + suggestions.length) % suggestions.length;
		else if (event.key === 'Enter' && active >= 0) pick(suggestions[active]);
		else if (event.key === 'Escape') suggestions = [];
		else return;
		event.preventDefault();
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		formError = '';
		errors = validateAddress(address);
		if (Object.keys(errors).length) return;
		saving = true;
		try {
			await onSave({ ...address });
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			formError = errorMessage(e);
		} finally {
			saving = false;
		}
	}

	type TextField = 'street' | 'suite' | 'city' | 'state' | 'zip' | 'country';
	const fields: { key: TextField; label: string; autocomplete: string; half?: boolean }[] = [
		{ key: 'street', label: 'Street address', autocomplete: 'address-line1' },
		{ key: 'suite', label: 'Suite / apartment (optional)', autocomplete: 'address-line2' },
		{ key: 'city', label: 'City', autocomplete: 'address-level2', half: true },
		{ key: 'state', label: 'State / region', autocomplete: 'address-level1', half: true },
		{ key: 'zip', label: 'ZIP / postal code', autocomplete: 'postal-code', half: true },
		{ key: 'country', label: 'Country', autocomplete: 'country-name', half: true }
	];
</script>

<form class="space-y-5" onsubmit={submit} novalidate>
	<div class="relative">
		<label class="label" for="place-search">Search for your address</label>
		<input
			id="place-search"
			class="input"
			placeholder="Try Shashamane, Nassau or Hargeisa"
			autocomplete="off"
			role="combobox"
			aria-expanded={suggestions.length > 0}
			aria-controls="place-results"
			aria-activedescendant={active >= 0 ? `place-${active}` : undefined}
			bind:value={query}
			oninput={search}
			onkeydown={onKeydown}
		/>
		{#if suggestions.length}
			<ul
				id="place-results"
				role="listbox"
				class="absolute z-10 mt-2 w-full overflow-hidden rounded-md border border-line-strong bg-raised shadow-xl"
			>
				{#each suggestions as suggestion, i (suggestion.placeId)}
					<li
						id="place-{i}"
						role="option"
						aria-selected={i === active}
						class={['cursor-pointer px-4 py-2.5', i === active ? 'bg-ink/10' : 'hover:bg-raised']}
						onmousedown={(e) => {
							e.preventDefault();
							pick(suggestion);
						}}
					>
						<span class="block text-sm font-medium">{suggestion.primaryText}</span>
						<span class="block text-xs text-muted">{suggestion.secondaryText}</span>
					</li>
				{/each}
			</ul>
		{:else if searching}
			<p class="mt-1.5 text-xs text-muted">Searching…</p>
		{/if}
	</div>

	<div class="grid grid-cols-2 gap-4">
		{#each fields as field (field.key)}
			<div class={field.half ? 'col-span-2 sm:col-span-1' : 'col-span-2'}>
				<label class="label" for="addr-{field.key}">{field.label}</label>
				<input
					id="addr-{field.key}"
					class={['input', errors[field.key] && 'input-error']}
					autocomplete={field.autocomplete as AutoFill}
					bind:value={address[field.key]}
					aria-invalid={Boolean(errors[field.key])}
				/>
				{#if errors[field.key]}<p class="field-error">{errors[field.key]}</p>{/if}
			</div>
		{/each}
	</div>

	{#if formError}<p class="field-error" role="alert">{formError}</p>{/if}

	<button class="btn btn-primary w-full py-3" disabled={saving}>
		{saving ? 'Saving…' : submitLabel}
	</button>
</form>
