<script lang="ts">
	/** A yes or no question: its label, help, two radio buttons and its error. */
	let {
		name,
		label,
		value,
		help = '',
		error = '',
		onchange
	}: {
		/** Groups the two buttons; also names the field. */
		name: string;
		label: string;
		/** `yes`, `no`, or anything else for no answer yet. */
		value: string | undefined;
		help?: string;
		error?: string;
		onchange: (value: 'yes' | 'no') => void;
	} = $props();

	const CHOICES = [
		['yes', 'Yes'],
		['no', 'No']
	] as const;
</script>

<fieldset>
	<legend class="label">{label}</legend>
	{#if help}<p class="-mt-1 mb-2 text-xs text-muted">{help}</p>{/if}
	<div class="flex gap-4">
		{#each CHOICES as [choice, text] (choice)}
			<label class="flex items-center gap-2 text-sm">
				<input
					type="radio"
					{name}
					class="border-line-strong bg-transparent text-teal checked:bg-current focus:ring-teal"
					value={choice}
					checked={value === choice}
					onchange={() => onchange(choice)}
				/>
				{text}
			</label>
		{/each}
	</div>
	{#if error}<p class="field-error">{error}</p>{/if}
</fieldset>
