<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * A modal dialog. Escape, the close button and a click on the backdrop all close it, and
	 * `onclose` runs once each time it closes, so a page can refresh what the modal changed.
	 */
	let {
		open = $bindable(false),
		title,
		eyebrow,
		wide = false,
		onclose,
		children,
		footer
	}: {
		open?: boolean;
		title: string;
		eyebrow?: string;
		/** For forms with tables or several columns. */
		wide?: boolean;
		onclose?: () => unknown;
		children: Snippet;
		footer?: Snippet;
	} = $props();

	let dialog = $state<HTMLDialogElement>();
	const id = $props.id();

	$effect(() => {
		if (!dialog) return;
		if (open && !dialog.open) dialog.showModal();
		if (!open && dialog.open) dialog.close();
	});
</script>

<dialog
	bind:this={dialog}
	class={[
		'modal m-auto overflow-hidden rounded-md border border-line-strong bg-surface p-0 text-text shadow-2xl backdrop:bg-black/75 backdrop:backdrop-blur-sm',
		wide ? 'w-[min(60rem,calc(100vw-2rem))]' : 'w-[min(36rem,calc(100vw-2rem))]'
	]}
	aria-labelledby="{id}-title"
	onclose={() => {
		open = false;
		onclose?.();
	}}
	onclick={(e) => {
		// A click on the dialog element itself is a click on the backdrop.
		if (e.target === dialog) dialog.close();
	}}
>
	<div class="flex max-h-[calc(100dvh-2rem)] flex-col">
		<div class="flex items-start justify-between gap-3 border-b border-line px-6 py-4">
			<div>
				{#if eyebrow}<p class="text-xs font-medium tracking-[0.18em] text-muted uppercase">
						{eyebrow}
					</p>{/if}
				<h2 id="{id}-title" class="text-lg font-semibold">{title}</h2>
			</div>
			<button
				type="button"
				class="grid size-8 place-items-center rounded-full border border-line-strong text-muted transition hover:border-ink hover:text-ink focus-visible:ring-2 focus-visible:ring-teal focus-visible:outline-none"
				aria-label="Close"
				onclick={() => dialog?.close()}
			>
				<Icon name="close" class="size-4" />
			</button>
		</div>
		<div class="min-h-0 flex-1 overflow-y-auto px-6 py-5">{@render children()}</div>
		{#if footer}
			<div class="flex flex-wrap justify-end gap-2 border-t border-line px-6 py-4">
				{@render footer()}
			</div>
		{/if}
	</div>
</dialog>

<style>
	.modal[open] {
		animation: modal-in 220ms cubic-bezier(0.2, 0.8, 0.2, 1);
	}
	.modal[open]::backdrop {
		animation: backdrop-in 220ms ease-out;
	}
	@keyframes modal-in {
		from {
			opacity: 0;
			transform: translateY(8px) scale(0.98);
		}
	}
	@keyframes backdrop-in {
		from {
			opacity: 0;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.modal[open],
		.modal[open]::backdrop {
			animation: none;
		}
	}
</style>
