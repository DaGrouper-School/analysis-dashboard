<script lang="ts">
	/**
	 * Previous and next through a long list fetched a page at a time, with where you are. Hidden
	 * when everything fits on one page.
	 */
	let {
		page,
		pageSize,
		total,
		onpage,
		class: className = ''
	}: {
		/** The page shown, from 1. */
		page: number;
		pageSize: number;
		total: number;
		onpage: (page: number) => void;
		class?: string;
	} = $props();

	const number = new Intl.NumberFormat();
	const pages = $derived(Math.max(1, Math.ceil(total / pageSize)));
	const first = $derived(Math.min(total, (page - 1) * pageSize + 1));
	const last = $derived(Math.min(total, page * pageSize));
</script>

{#if pages > 1}
	<nav
		class={['flex flex-wrap items-center justify-between gap-3 text-sm', className]}
		aria-label="Pages"
	>
		<p class="text-muted">
			{number.format(first)}–{number.format(last)} of {number.format(total)} · page {page} of
			{number.format(pages)}
		</p>
		<div class="flex gap-2">
			<button
				type="button"
				class="btn btn-ghost"
				disabled={page <= 1}
				onclick={() => onpage(page - 1)}>Previous</button
			>
			<button
				type="button"
				class="btn btn-ghost"
				disabled={page >= pages}
				onclick={() => onpage(page + 1)}>Next</button
			>
		</div>
	</nav>
{/if}
