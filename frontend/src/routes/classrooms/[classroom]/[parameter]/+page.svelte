<script lang="ts">
	import PublicShell from '#lib/components/PublicShell.svelte';
	import ValuePage from '#lib/components/ValuePage.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<PublicShell content={data.content} user={data.user}>
	<ValuePage {data} />
</PublicShell>
