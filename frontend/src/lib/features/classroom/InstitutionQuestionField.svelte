<script lang="ts">
	import PhotoPicker from '#lib/components/PhotoPicker.svelte';
	import YesNoField from '#lib/components/YesNoField.svelte';
	import type { InstitutionAnswers, InstitutionQuestion } from '#lib/domain/types.ts';

	/**
	 * One question of the institution setup (AP-40), drawn for its kind: a photo, text, a year, a
	 * web address, yes or no, or several choices with answers of the head's own.
	 */
	let {
		question,
		answers = $bindable(),
		error = '',
		onchange
	}: {
		question: InstitutionQuestion;
		answers: InstitutionAnswers;
		error?: string;
		/** After the answer changes, with an error when a picked photo could not be read. */
		onchange: (error?: string) => void;
	} = $props();

	const id = $derived(`institution-${question.field}`);

	/** The text answers, by field. */
	type TextField = 'photoAlt' | 'foundedYear' | 'history' | 'hub' | 'website';
	const textField = $derived(question.field as TextField);

	// Multiple choice: the configured options, and whatever else the head named.
	const options = $derived(question.options ?? []);
	// Seeded once from the saved answers; the head edits it from then on.
	const initialOthers = answers.ecosystems.filter((e) => !question.options?.includes(e));
	let otherText = $state(initialOthers.join(', '));

	function toggle(option: string, on: boolean) {
		const chosen = answers.ecosystems.filter((e) => e !== option);
		answers.ecosystems = on ? [...chosen, option] : chosen;
		onchange();
	}

	function setOthers(text: string) {
		otherText = text;
		const named = text
			.split(/[,;\n]/)
			.map((e) => e.trim())
			.filter(Boolean);
		answers.ecosystems = [...answers.ecosystems.filter((e) => options.includes(e)), ...named];
		onchange();
	}

	const count = $derived(answers[textField]?.length ?? 0);
</script>

{#snippet labelFor(forId: string)}
	<label class="label" for={forId}>
		{question.label}
		{#if !question.required}<span class="font-normal text-muted">(optional)</span>{/if}
	</label>
{/snippet}

{#snippet hintAndError()}
	{#if question.help}<p class="mt-1 text-xs text-muted">{question.help}</p>{/if}
	{#if error}<p class="field-error">{error}</p>{/if}
{/snippet}

{#if question.kind === 'photo'}
	<PhotoPicker
		{id}
		label={question.label}
		help={question.help}
		required={question.required}
		bind:image={answers.photo}
		alt={answers.photoAlt}
		{error}
		onpick={(pickError) => onchange(pickError)}
	/>
{:else if question.kind === 'yes_no'}
	<YesNoField
		name={id}
		label={question.label}
		help={question.help}
		value={answers.showPublicly}
		{error}
		onchange={(value) => {
			answers.showPublicly = value;
			onchange();
		}}
	/>
{:else if question.kind === 'multi_choice'}
	<fieldset>
		<legend class="label">
			{question.label}
			{#if !question.required}<span class="font-normal text-muted">(optional)</span>{/if}
		</legend>
		{#if question.help}<p class="-mt-1 mb-2 text-xs text-muted">{question.help}</p>{/if}
		<div class="flex flex-wrap gap-2">
			{#each options as option (option)}
				{@const on = answers.ecosystems.includes(option)}
				<label
					class={[
						'flex cursor-pointer items-center gap-2 rounded-full border px-3 py-1.5 text-sm transition-colors',
						on ? 'border-teal bg-teal/10 text-ink' : 'border-line-strong text-muted hover:text-ink'
					]}
				>
					<input
						type="checkbox"
						class="rounded-sm border-line-strong bg-transparent text-teal focus:ring-teal"
						checked={on}
						onchange={(e) => toggle(option, e.currentTarget.checked)}
					/>
					{option}
				</label>
			{/each}
		</div>
		{#if question.allowOther}
			<label class="label mt-3" for="{id}-other">
				Other <span class="font-normal text-muted">(separate with commas)</span>
			</label>
			<input
				id="{id}-other"
				class="input"
				value={otherText}
				oninput={(e) => setOthers(e.currentTarget.value)}
			/>
		{/if}
		{#if error}<p class="field-error">{error}</p>{/if}
	</fieldset>
{:else if question.kind === 'long_text'}
	<div>
		{@render labelFor(id)}
		<textarea
			{id}
			rows="5"
			maxlength={question.maxLength}
			class={['input', error && 'input-error']}
			bind:value={answers[textField]}
			oninput={() => onchange()}></textarea>
		{#if question.maxLength}
			<p class="mt-1 text-right text-xs text-muted tabular-nums">
				{count} / {question.maxLength}
			</p>
		{/if}
		{@render hintAndError()}
	</div>
{:else}
	<div>
		{@render labelFor(id)}
		<input
			{id}
			type={question.kind === 'url' ? 'url' : 'text'}
			inputmode={question.kind === 'year' ? 'numeric' : undefined}
			maxlength={question.maxLength}
			placeholder={question.kind === 'url' ? 'https://' : undefined}
			class={['input', error && 'input-error']}
			bind:value={answers[textField]}
			oninput={() => onchange()}
		/>
		{@render hintAndError()}
	</div>
{/if}
