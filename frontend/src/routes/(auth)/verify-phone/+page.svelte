<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import OtpForm from '#lib/components/OtpForm.svelte';
	import type { PhoneChallenge } from '#lib/domain/types.ts';
	import {
		clearPendingChallenge,
		loadPendingChallenge,
		savePendingChallenge
	} from '#lib/pending-challenge.ts';
	import { services } from '#lib/services/index.ts';
	import { refreshSessionAndContinue } from '#lib/session.ts';

	let challenge = $state<PhoneChallenge | null>(null);

	onMount(() => {
		challenge = loadPendingChallenge();
		if (!challenge) goto(resolve('/(auth)/signup'), { replaceState: true });
	});

	async function verify(code: string) {
		const { user } = await services.auth.verifyPhoneCode(challenge!.requestId, code);
		clearPendingChallenge();
		await refreshSessionAndContinue(user);
	}

	async function resend() {
		challenge = await services.auth.requestPhoneCode(challenge!.phone);
		savePendingChallenge(challenge);
	}
</script>

<svelte:head><title>Verify your phone · DaGrouper Analysis</title></svelte:head>

{#if challenge}
	<a href={resolve('/(auth)/signup')} class="mb-6 text-sm text-muted hover:text-ink"
		>← Use a different number</a
	>
	<h1 class="font-display text-3xl">Check your messages</h1>
	<p class="mt-2 mb-8 text-muted">
		We sent a 6-digit code to <strong class="text-ink">{challenge.phone}</strong>.
	</p>
	{#key challenge.requestId}
		<OtpForm {challenge} onVerify={verify} onResend={resend} submitLabel="Verify and continue" />
	{/key}
{/if}
