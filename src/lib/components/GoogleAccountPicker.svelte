<script lang="ts">
	import Avatar from './Avatar.svelte';

	export interface MockGoogleAccount {
		sub: string;
		name: string;
		email: string;
	}

	let {
		accounts,
		onPick,
		onClose
	}: {
		accounts: MockGoogleAccount[];
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
	class="m-auto w-[min(26rem,calc(100vw-2rem))] rounded-3xl bg-white p-0 shadow-2xl backdrop:bg-ink/40"
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
					class="flex w-full items-center gap-3 px-6 py-3 text-left hover:bg-canvas"
					onclick={() => onPick(account.sub)}
				>
					<Avatar name={account.name} />
					<span>
						<span class="block text-sm font-medium">{account.name}</span>
						<span class="block text-sm text-muted">{account.email}</span>
					</span>
				</button>
			</li>
		{/each}
	</ul>
	<div class="flex justify-end px-6 py-4">
		<button class="btn btn-ghost" onclick={() => dialog.close()}>Cancel</button>
	</div>
</dialog>
