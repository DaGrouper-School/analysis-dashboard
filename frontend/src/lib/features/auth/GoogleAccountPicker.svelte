<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import Avatar from '#lib/components/Avatar.svelte';
	import type { DemoGoogleAccount } from '#lib/domain/types.ts';

	let {
		accounts,
		onPick,
		onClose
	}: {
		accounts: DemoGoogleAccount[];
		onPick: (sub: string) => void;
		onClose: () => void;
	} = $props();

	let dialog: HTMLDialogElement;

	$effect(() => {
		dialog.showModal();
	});
</script>

<!-- Stand-in for the Google OAuth popup. The real flow returns an ID token instead of `sub`. -->
<dialog
	bind:this={dialog}
	class="m-auto w-[min(30rem,calc(100vw-2rem))] rounded-md border border-line-strong bg-raised p-0 text-text shadow-2xl backdrop:bg-black/70"
	onclose={onClose}
	aria-labelledby="google-title"
>
	<div class="border-b border-line px-6 pt-6 pb-4">
		<p class="text-xs font-semibold tracking-wide text-muted uppercase">Demo · Google sign-in</p>
		<h2 id="google-title" class="mt-1 text-lg font-semibold">Choose an account</h2>
		<p class="text-sm text-muted">to continue to DaGrouper Analysis</p>
	</div>
	<ul class="divide-y divide-line">
		{#each accounts as account (account.sub)}
			<li>
				<button
					class="flex w-full items-center gap-3 px-6 py-3 text-left hover:bg-raised"
					onclick={() => onPick(account.sub)}
				>
					<Avatar name={account.name} />
					<span class="min-w-0 flex-1">
						<span class="block text-sm font-medium">{account.name}</span>
						<span class="block truncate text-sm text-muted">{account.email}</span>
					</span>
					{#if account.type}<span class="chip shrink-0 text-xs">{account.type}</span>{/if}
				</button>
			</li>
		{/each}
	</ul>
	<div class="flex justify-end px-6 py-4">
		<button class="btn btn-ghost" onclick={() => dialog.close()}
			><Icon name="close" class="size-4" /> Cancel</button
		>
	</div>
</dialog>
