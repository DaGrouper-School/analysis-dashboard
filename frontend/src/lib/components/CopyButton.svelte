<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	let { text, label = 'Copy' }: { text: string; label?: string } = $props();

	let copied = $state(false);

	async function copy() {
		try {
			await navigator.clipboard.writeText(text);
			copied = true;
			setTimeout(() => (copied = false), 1500);
		} catch {
			window.prompt('Copy this:', text);
		}
	}
</script>

<button type="button" class="btn btn-ghost px-3 py-1.5 text-xs" onclick={copy}>
	<Icon name="copy" class="size-3.5" />
	{copied ? 'Copied' : label}
</button>
