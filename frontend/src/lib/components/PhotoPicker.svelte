<script lang="ts">
	import { scaledImage } from '#lib/utils/image.ts';

	/**
	 * Pick a photo: a wide preview, or a placeholder with none, and a file input. The picked file is
	 * scaled down and handed back as a data URL in `image`.
	 */
	let {
		id,
		label,
		image = $bindable(),
		alt = '',
		help = '',
		error = '',
		required = false,
		onpick
	}: {
		id: string;
		label: string;
		/** A data URL or image URL; null for none. */
		image: string | null;
		/** What the photo shows, for the preview. */
		alt?: string;
		help?: string;
		error?: string;
		required?: boolean;
		/** After a new photo is picked, or the file could not be read (with its error). */
		onpick?: (error: string) => void;
	} = $props();

	let input = $state<HTMLInputElement>();

	async function pick(event: Event) {
		const file = (event.currentTarget as HTMLInputElement).files?.[0];
		if (!file) return;
		try {
			image = await scaledImage(file);
			onpick?.('');
		} catch {
			onpick?.('That file could not be read as an image.');
		}
	}

	/** Clears the chosen file, so the same file can be picked again. */
	export function reset() {
		if (input) input.value = '';
	}
</script>

<div class="space-y-3">
	<div class="aspect-[16/9] overflow-hidden rounded-md border border-line bg-raised">
		{#if image}
			<img src={image} alt={alt || 'The new photo'} class="size-full object-cover" />
		{:else}
			<p class="grid size-full place-items-center text-sm text-muted">No photo yet</p>
		{/if}
	</div>
	<div>
		<label class="label" for={id}>
			{label}
			{#if !required}<span class="font-normal text-muted">(optional)</span>{/if}
		</label>
		<input
			bind:this={input}
			{id}
			type="file"
			accept="image/png,image/jpeg,image/webp"
			aria-invalid={error ? true : undefined}
			class="block w-full text-sm text-text file:mr-3 file:rounded-sm file:border file:border-line-strong file:bg-transparent file:px-3 file:py-2 file:text-ink"
			onchange={pick}
		/>
		{#if help}<p class="mt-1 text-xs text-muted">{help}</p>{/if}
		{#if error}<p class="field-error">{error}</p>{/if}
	</div>
</div>
