<script lang="ts" module>
	export interface SelectOption<T extends string = string> {
		value: T;
		label: string;
		/** A second, quieter line under the label. */
		hint?: string;
		/** A registry or UI icon, drawn on a tile tinted with `color`. */
		icon?: string;
		/** A registry color (device type, stage). Alone it draws a dot. */
		color?: string;
		/** Options that share a group are listed under its heading. */
		group?: string;
		disabled?: boolean;
		/** A small chip after the label, such as "Coming soon". */
		badge?: string;
	}

	/** Plain choices from a list of values, labelled from a lookup where one is given. */
	export function toOptions<T extends string>(
		values: readonly T[],
		labels?: Partial<Record<T, string>>
	): SelectOption<T>[] {
		return values.map((value) => ({ value, label: labels?.[value] ?? value }));
	}
</script>

<script lang="ts" generics="T extends string">
	import { tick } from 'svelte';
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * The app's dropdown. It looks like our inputs, opens a list under (or over) itself, and on
	 * phones rises from the bottom as a sheet. It is used like a native select: `bind:value`,
	 * `onchange`, and `name` for a form. `label` is shown above it, so a dropdown always says what
	 * it picks; `labelHidden` keeps it for screen readers only, where the label already in view
	 * names it (a unit beside its field, a row of a table). Or pass `id` and write your own
	 * `<label for>`, as a form does. Choices with an empty value
	 * ("All sources", "Leave blank") lead the list, set apart by a line. Long lists get a filter; on a phone it sits at the bottom of the sheet and the keyboard
	 * only opens when it is tapped.
	 */
	let {
		options,
		value = $bindable(),
		onchange,
		onblur,
		id,
		name,
		label,
		labelHidden = false,
		placeholder = 'Choose',
		size = 'field',
		align = 'start',
		search = 'auto',
		error = false,
		warn = false,
		disabled = false,
		class: className = '',
		'aria-describedby': describedBy
	}: {
		options: readonly SelectOption<T>[];
		value?: T | null;
		onchange?: (value: T) => void;
		/** Runs when focus leaves the select and its list. */
		onblur?: () => void;
		/** Goes on the button, so `<label for={id}>` names it. */
		id?: string;
		name?: string;
		/** Shown above the select, unless a `<label for={id}>` of the caller's names it. */
		label?: string;
		/** Keeps `label` for screen readers only, where something in view already names it. */
		labelHidden?: boolean;
		/** Shown while the value matches no option. */
		placeholder?: string;
		/** `field` for forms, `compact` (a pill) for toolbars, `mini` for a unit beside an input. */
		size?: 'field' | 'compact' | 'mini';
		/** Which edge the list lines up with when it is wider than the button. */
		align?: 'start' | 'end';
		/** Show a filter box: always, never, or for lists longer than eight. */
		search?: boolean | 'auto';
		error?: boolean;
		/** An orange edge: the value needs a look, but nothing is wrong. */
		warn?: boolean;
		disabled?: boolean;
		class?: string;
		'aria-describedby'?: string;
	} = $props();

	const uid = $props.id();
	const buttonId = $derived(id ?? `${uid}-button`);
	let root = $state<HTMLDivElement>();
	let button = $state<HTMLButtonElement>();
	let panel = $state<HTMLDivElement>();
	let list = $state<HTMLUListElement>();
	let filterInput = $state<HTMLInputElement>();

	let open = $state(false);
	let up = $state(false);
	let sheet = $state(false);
	let active = $state(-1);
	let term = $state('');
	let place = $state('');

	const selected = $derived(options.find((o) => o.value === value));
	const filterable = $derived(search === 'auto' ? options.length > 8 : search);
	const shown = $derived.by(() => {
		const t = term.trim().toLowerCase();
		return t ? options.filter((o) => o.label.toLowerCase().includes(t)) : options;
	});
	const hits = $derived(shown.filter((o) => o.value !== '').length);

	/** A line after the empty-value choices that lead the list; a heading where a group starts. */
	function lineAfter(i: number) {
		return (
			!term && shown[i].value === '' && shown[i + 1] !== undefined && shown[i + 1].value !== ''
		);
	}
	function groupStarts(i: number) {
		return !term && !!shown[i].group && shown[i].group !== shown[i - 1]?.group;
	}
	function parts(text: string) {
		const t = term.trim();
		const at = t ? text.toLowerCase().indexOf(t.toLowerCase()) : -1;
		return at < 0
			? null
			: [text.slice(0, at), text.slice(at, at + t.length), text.slice(at + t.length)];
	}

	function step(from: number, dir: 1 | -1) {
		let n = from;
		for (let k = 0; k < shown.length; k++) {
			n = (n + dir + shown.length) % shown.length;
			if (!shown[n].disabled) return n;
		}
		return from;
	}

	function setActive(n: number, scroll = true) {
		active = n;
		if (scroll)
			tick().then(() =>
				list?.querySelector(`#${CSS.escape(`${uid}-o${n}`)}`)?.scrollIntoView({ block: 'nearest' })
			);
	}

	/** Puts the list under the button, or over it when there is more room above. */
	function position() {
		if (!button || !panel) return;
		sheet = matchMedia('(max-width: 639px)').matches;
		if (sheet) {
			place = '';
			return;
		}
		const r = button.getBoundingClientRect();
		const below = innerHeight - r.bottom;
		up = below < 280 && r.top > below;
		const width = Math.max(r.width, panel.offsetWidth);
		const left =
			align === 'end'
				? Math.max(8, r.right - width)
				: Math.min(r.left, Math.max(8, innerWidth - width - 8));
		place = [
			`left:${left}px`,
			`min-width:${r.width}px`,
			up ? `bottom:${innerHeight - r.top + 6}px` : `top:${r.bottom + 6}px`,
			`max-height:${Math.max(160, (up ? r.top : below) - 16)}px`
		].join(';');
	}

	async function show() {
		if (disabled || open) return;
		term = '';
		open = true;
		await tick();
		panel?.showPopover?.();
		position();
		const current = shown.findIndex((o) => o.value === value);
		setActive(current >= 0 && !shown[current].disabled ? current : step(-1, 1));
		// On a phone the list takes focus, not the filter, so the keyboard only opens when the
		// person taps the filter; the current choice is highlighted either way.
		(sheet ? list : (filterInput ?? list))?.focus({ preventScroll: true });
	}

	function hide(refocus = true) {
		if (!open) return;
		if (panel?.matches(':popover-open')) panel.hidePopover();
		open = false;
		if (refocus) button?.focus({ preventScroll: true });
	}

	function pick(n: number) {
		const option = shown[n];
		if (!option || option.disabled) return;
		hide();
		if (option.value === value) return;
		value = option.value;
		onchange?.(option.value);
	}

	let typed = '';
	let typedAt = 0;
	function keys(event: KeyboardEvent) {
		const k = event.key;
		if (k === 'ArrowDown' || k === 'ArrowUp') setActive(step(active, k === 'ArrowDown' ? 1 : -1));
		else if (k === 'Home' && !filterable) setActive(step(-1, 1));
		else if (k === 'End' && !filterable) setActive(step(shown.length, -1));
		else if (k === 'Enter' || (k === ' ' && !filterable)) pick(active);
		else if (k === 'Escape') hide();
		else if (k === 'Tab') return hide(false);
		else if (!filterable && k.length === 1 && k !== ' ') {
			// Typing jumps to the first choice that starts with the letters typed.
			const now = Date.now();
			typed = (now - typedAt > 600 ? '' : typed) + k.toLowerCase();
			typedAt = now;
			const hit = shown.findIndex((o) => !o.disabled && o.label.toLowerCase().startsWith(typed));
			if (hit >= 0) setActive(hit);
			return;
		} else return;
		event.preventDefault();
	}

	function triggerKeys(event: KeyboardEvent) {
		if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(event.key)) {
			event.preventDefault();
			show();
		}
	}

	// While open: follow the button as the page scrolls, and close on a click elsewhere.
	$effect(() => {
		if (!open) return;
		const away = (event: PointerEvent) => {
			const target = event.target as Node;
			if (target === panel || !root?.contains(target)) hide(false);
		};
		addEventListener('scroll', position, true);
		addEventListener('resize', position);
		document.addEventListener('pointerdown', away);
		return () => {
			removeEventListener('scroll', position, true);
			removeEventListener('resize', position);
			document.removeEventListener('pointerdown', away);
		};
	});

	function focusout(event: FocusEvent) {
		const next = event.relatedTarget as Node | null;
		if (next && root?.contains(next)) return;
		hide(false);
		onblur?.();
	}
</script>

<div
	bind:this={root}
	class={[
		'select relative',
		// A field fills its row unless the caller sets a width (`w-36`), which would lose to it.
		size === 'field'
			? ['block', !/(^|\s)([\w-]+:)?w-/.test(className) && 'w-full']
			: 'inline-block max-w-full',
		className
	]}
	data-size={size}
	onfocusout={focusout}
>
	{#if label && !labelHidden}
		<label class={size === 'field' ? 'label' : 'mb-1 block text-xs text-muted'} for={buttonId}
			>{label}</label
		>
	{/if}
	<button
		bind:this={button}
		id={buttonId}
		type="button"
		class={['select-trigger', error ? 'select-error' : warn && 'select-warn']}
		aria-haspopup="listbox"
		aria-expanded={open}
		aria-controls="{uid}-list"
		aria-label={labelHidden ? label : undefined}
		aria-describedby={describedBy}
		{disabled}
		onclick={() => (open ? hide() : show())}
		onkeydown={triggerKeys}
	>
		{#if selected}
			{@render lead(selected)}
			<span class="min-w-0 flex-1 truncate">{selected.label}</span>
		{:else}
			<span class="min-w-0 flex-1 truncate text-muted">{placeholder}</span>
		{/if}
		<svg
			class="select-chevron"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-width="1.75"
			stroke-linecap="round"
			stroke-linejoin="round"
			aria-hidden="true"><path d="m6 9 6 6 6-6" /></svg
		>
	</button>
	{#if name}<input type="hidden" {name} value={value ?? ''} />{/if}

	{#if open}
		<div
			bind:this={panel}
			popover="manual"
			class={['select-panel', sheet ? 'is-sheet' : up ? 'is-up' : 'is-down']}
			style={place}
		>
			{#if sheet && label}
				<p class="select-sheet-title">{label}</p>
			{:else if sheet}
				<span class="select-sheet-handle" aria-hidden="true"></span>
			{/if}
			{#if filterable}
				<div class="select-filter">
					<Icon name="search" class="size-4 text-muted" />
					<input
						bind:this={filterInput}
						bind:value={term}
						class="min-w-0 flex-1 border-0 bg-transparent p-1 text-sm text-ink placeholder:text-muted focus:ring-0"
						placeholder="Type to filter"
						aria-label="Filter choices"
						aria-controls="{uid}-list"
						aria-activedescendant={active >= 0 ? `${uid}-o${active}` : undefined}
						autocomplete="off"
						onkeydown={keys}
						oninput={() => setActive(step(-1, 1))}
					/>
					{#if term}<span class="text-xs whitespace-nowrap text-muted tabular-nums"
							>{hits} found</span
						>{/if}
				</div>
			{/if}
			<ul
				bind:this={list}
				id="{uid}-list"
				role="listbox"
				tabindex="-1"
				class="select-list"
				aria-label={label}
				aria-labelledby={label ? undefined : buttonId}
				aria-activedescendant={!filterable && active >= 0 ? `${uid}-o${active}` : undefined}
				onkeydown={keys}
			>
				{#each shown as option, i (option.value)}
					{#if groupStarts(i)}
						<li class="select-group" role="presentation">{option.group}</li>
					{/if}
					<!-- Keys are handled by the list, which keeps focus while an option is active. -->
					<!-- svelte-ignore a11y_click_events_have_key_events -->
					<li
						id="{uid}-o{i}"
						role="option"
						class={['select-option', i === active && 'is-active']}
						style="--i:{Math.min(i, 12)}"
						aria-selected={option.value === value}
						aria-disabled={option.disabled || undefined}
						onpointermove={() => !option.disabled && active !== i && setActive(i, false)}
						onclick={() => pick(i)}
					>
						{@render lead(option)}
						<span class="flex min-w-0 flex-1 flex-col">
							<span>
								{#if parts(option.label)}
									{@const [before, match, after] = parts(option.label)!}
									{before}<mark class="bg-transparent font-semibold text-teal">{match}</mark>{after}
								{:else}
									{option.label}
								{/if}
							</span>
							{#if option.hint}<span class="text-[0.8125rem] leading-snug text-muted"
									>{option.hint}</span
								>{/if}
						</span>
						{#if option.badge}<span class="chip whitespace-nowrap">{option.badge}</span>{/if}
						<Icon name="check" class="select-tick size-4" />
					</li>
					{#if lineAfter(i)}<li class="select-line" role="separator"></li>{/if}
				{:else}
					<li class="px-2.5 py-3 text-sm text-muted">No match. Try fewer letters.</li>
				{/each}
			</ul>
		</div>
	{/if}
</div>

{#snippet lead(option: SelectOption)}
	{#if option.icon}
		<span class="select-lead" style:--accent={option.color}>
			<Icon name={option.icon} class="size-[1em]" />
		</span>
	{:else if option.color}
		<span class="select-dot" style:background={option.color} aria-hidden="true"></span>
	{/if}
{/snippet}

<style>
	.select-trigger {
		display: flex;
		width: 100%;
		min-width: 0;
		align-items: center;
		gap: 0.625rem;
		border: 1px solid var(--color-line-strong);
		border-radius: 0.25rem;
		background: transparent;
		padding: 0.625rem 0.75rem 0.625rem 0.875rem;
		text-align: left;
		color: var(--color-ink);
		transition:
			border-color 150ms ease,
			box-shadow 150ms ease;
	}
	.select-trigger:hover {
		border-color: color-mix(in oklab, var(--color-ink) 55%, transparent);
	}
	.select-trigger[aria-expanded='true'],
	.select-trigger:focus-visible {
		border-color: var(--color-ink);
		outline: none;
	}
	.select-trigger:focus-visible {
		box-shadow: 0 0 0 3px color-mix(in oklab, var(--color-teal) 35%, transparent);
	}
	.select-trigger:disabled {
		border-color: var(--color-line);
		color: var(--color-muted);
	}
	.select-error,
	.select-error:hover,
	.select-error[aria-expanded='true'] {
		border-color: var(--color-danger);
	}
	.select-warn,
	.select-warn:hover,
	.select-warn[aria-expanded='true'] {
		border-color: var(--color-orange);
	}
	[data-size='compact'] .select-trigger {
		gap: 0.5rem;
		border-radius: 9999px;
		padding: 0.375rem 0.625rem 0.375rem 0.75rem;
		font-size: 0.875rem;
	}
	[data-size='mini'] .select-trigger {
		padding-inline: 0.75rem 0.5rem;
	}
	.select-chevron {
		width: 1rem;
		height: 1rem;
		flex: none;
		color: var(--color-muted);
		transition:
			rotate 220ms cubic-bezier(0.3, 0.7, 0.2, 1.2),
			color 150ms ease;
	}
	[aria-expanded='true'] .select-chevron {
		rotate: 180deg;
		color: var(--color-ink);
	}
	.select-lead {
		display: grid;
		width: 1.625rem;
		height: 1.625rem;
		flex: none;
		place-items: center;
		border: 1px solid color-mix(in oklab, var(--accent, var(--color-muted)) 40%, var(--color-line));
		border-radius: 0.3rem;
		background: color-mix(in oklab, var(--accent, var(--color-muted)) 12%, var(--color-raised));
		color: var(--accent, var(--color-text));
		font-size: 1rem;
	}
	[data-size='compact'] .select-trigger .select-lead {
		width: 1.25rem;
		height: 1.25rem;
		border-radius: 9999px;
		font-size: 0.75rem;
	}
	.select-dot {
		width: 0.625rem;
		height: 0.625rem;
		flex: none;
		border-radius: 9999px;
	}

	.select-panel {
		position: fixed;
		inset: auto;
		margin: 0;
		display: flex;
		max-width: min(26rem, calc(100vw - 1rem));
		flex-direction: column;
		overflow: hidden;
		border: 1px solid var(--color-line-strong);
		border-radius: 0.375rem;
		background: var(--color-surface);
		padding: 0.25rem;
		font-size: 0.9375rem;
		line-height: 1.4;
		color: var(--color-text);
		box-shadow:
			0 24px 48px -16px rgb(0 0 0 / 0.55),
			0 2px 6px -2px rgb(0 0 0 / 0.12);
		animation: select-in 200ms cubic-bezier(0.2, 0.8, 0.2, 1) both;
	}
	:global([data-theme='light']) .select-panel {
		box-shadow:
			0 24px 48px -18px rgb(11 18 21 / 0.28),
			0 2px 6px -2px rgb(11 18 21 / 0.08);
	}
	.select-panel.is-down {
		transform-origin: top center;
	}
	.select-panel.is-up {
		transform-origin: bottom center;
		--from: 6px;
	}
	[data-size='compact'] .select-panel {
		font-size: 0.875rem;
	}
	.select-panel.is-sheet {
		inset: auto 0 0 0;
		width: auto;
		max-width: none;
		font-size: 1rem;
		max-height: 70dvh;
		border-bottom: 0;
		border-radius: 0.875rem 0.875rem 0 0;
		padding: 0.375rem 0.625rem calc(0.75rem + env(safe-area-inset-bottom, 0px));
		animation-name: select-sheet-in;
		animation-duration: 260ms;
	}
	.select-panel.is-sheet::backdrop {
		background: rgb(0 0 0 / 0.5);
	}
	.select-sheet-handle,
	.select-sheet-title::before {
		display: block;
		width: 2.5rem;
		height: 0.25rem;
		margin: 0.25rem auto 0.625rem;
		border-radius: 9999px;
		background: var(--color-line-strong);
		content: '';
	}
	.select-sheet-title {
		padding-bottom: 0.5rem;
		text-align: center;
		font-weight: 600;
		color: var(--color-ink);
	}
	.select-filter {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin: -0.25rem -0.25rem 0.25rem;
		border-bottom: 1px solid var(--color-line);
		padding: 0.375rem 0.875rem;
	}
	/* On a phone the filter sits at the bottom of the sheet, by the thumb and the keyboard. */
	.is-sheet .select-filter {
		order: 1;
		margin: 0.25rem -0.25rem 0;
		border-top: 1px solid var(--color-line);
		border-bottom: 0;
		padding-top: 0.625rem;
	}
	.select-list {
		min-height: 0;
		flex: 1;
		overflow-y: auto;
		overscroll-behavior: contain;
		scrollbar-width: thin;
		outline: none;
	}
	.select-group {
		padding: 0.625rem 0.625rem 0.25rem;
		font-size: 0.75rem;
		font-weight: 600;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--color-muted);
	}
	.select-line {
		height: 1px;
		margin: 0.25rem 0.375rem;
		background: var(--color-line);
	}
	.select-option {
		display: flex;
		cursor: pointer;
		align-items: center;
		gap: 0.625rem;
		border-radius: 0.25rem;
		padding: 0.5rem 0.625rem;
		transition:
			background-color 120ms ease,
			color 120ms ease;
		animation: select-option-in 220ms cubic-bezier(0.2, 0.8, 0.2, 1) both;
		animation-delay: calc(var(--i) * 14ms);
	}
	.is-sheet .select-option {
		padding: 0.75rem;
	}
	.select-option.is-active {
		background: var(--color-raised);
		color: var(--color-ink);
	}
	.select-option[aria-selected='true'] {
		font-weight: 500;
		color: var(--color-ink);
	}
	.select-option[aria-disabled='true'] {
		cursor: not-allowed;
		color: var(--color-muted);
	}
	.select-option :global(.select-tick) {
		color: var(--color-teal);
		opacity: 0;
		transform: scale(0.6);
		transition:
			opacity 150ms ease,
			transform 200ms cubic-bezier(0.3, 0.7, 0.2, 1.4);
	}
	.select-option[aria-selected='true'] :global(.select-tick) {
		opacity: 1;
		transform: none;
	}
	@keyframes select-in {
		from {
			opacity: 0;
			transform: translateY(var(--from, -6px)) scale(0.97);
		}
	}
	@keyframes select-sheet-in {
		from {
			transform: translateY(100%);
		}
	}
	@keyframes select-option-in {
		from {
			opacity: 0;
			transform: translateY(-3px);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.select-panel,
		.select-option,
		.select-chevron,
		.select-option :global(.select-tick) {
			animation: none;
			transition: none;
		}
	}
</style>
