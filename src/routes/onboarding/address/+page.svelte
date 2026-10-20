<script lang="ts">
	import { resolve } from '$app/paths';
	import AddressForm from '#lib/components/AddressForm.svelte';
	import type { Address } from '#lib/domain/types.ts';
	import { services } from '#lib/services/index.ts';
	import { refreshSessionAndContinue } from '#lib/session.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	async function save(address: Address) {
		await services.profile.updateAddress(address);
		const user = await services.profile.completeOnboarding();
		await refreshSessionAndContinue(user);
	}
</script>

<svelte:head><title>Your address · DaGrouper Analysis</title></svelte:head>

<a href={resolve('/onboarding/profile')} class="mb-6 text-sm text-muted hover:text-ink">← Back</a>
<h1 class="font-display text-4xl">Where are you based?</h1>
<p class="mt-2 mb-8 text-muted">This ties your account to the right community and station.</p>

<AddressForm initial={data.user.address} onSave={save} submitLabel="Finish setup" />
