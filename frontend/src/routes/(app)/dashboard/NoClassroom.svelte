<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import { resolve } from '$app/paths';

	/** The dashboard for someone in no classroom yet: how to join one, or start one. */
	let { firstName }: { firstName: string } = $props();
</script>

<p class="text-sm font-medium text-muted">Hello, {firstName}</p>
<div class="card mt-6">
	<p class="text-sm text-muted">Current conditions</p>
	<h2 class="mt-1 text-xl font-semibold">You're not in a classroom yet</h2>
	<p class="mt-2 text-sm text-muted">
		Conditions come only from devices in your classrooms' labs. Join a classroom with an invite link
		to see its measurements here.
	</p>
	<a class="btn btn-ghost mt-4" href={resolve('/(app)/classrooms/new')}
		><Icon name="plus" class="size-4" /> Start a classroom</a
	>
</div>
