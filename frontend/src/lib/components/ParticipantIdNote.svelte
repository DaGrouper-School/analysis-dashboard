<script lang="ts">
	import { resolve } from '$app/paths';

	/**
	 * Where a recording or sync is confirmed (DAG-143): what is recorded is tied to the person's
	 * participant ID, and whether their profile is public is theirs to set in Settings.
	 */
	let { class: className = '' }: { class?: string } = $props();
</script>

<p class={['text-xs text-muted', className]}>
	What you record is tied to your participant ID, not your name. You can change whether your profile
	is public in
	<a class="link" href="{resolve('/(app)/settings')}?tab=profile#publicProfile">Settings</a>.
</p>
