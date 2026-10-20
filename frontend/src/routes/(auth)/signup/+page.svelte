<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import GoogleAccountPicker from '#lib/components/GoogleAccountPicker.svelte';
	import PhoneInput from '#lib/components/PhoneInput.svelte';
	import { normalizePhone } from '#lib/domain/validation.ts';
	import { savePendingChallenge } from '#lib/pending-challenge.ts';
	import { errorMessage, isMockDataSource, services } from '#lib/services/index.ts';
	import { refreshSessionAndContinue } from '#lib/session.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let phone = $state('');
	let phoneError = $state('');
	let formError = $state('');
	let sendingCode = $state(false);
	let googleBusy = $state(false);
	let pickerOpen = $state(false);

	async function continueWithGoogle(idToken: string) {
		pickerOpen = false;
		googleBusy = true;
		formError = '';
		try {
			const { user } = await services.auth.signInWithGoogle(idToken);
			await refreshSessionAndContinue(user);
		} catch (e) {
			formError = errorMessage(e);
		} finally {
			googleBusy = false;
		}
	}

	async function continueWithPhone(event: SubmitEvent) {
		event.preventDefault();
		formError = '';
		if (!normalizePhone(phone)) {
			phoneError = 'Enter a valid phone number.';
			return;
		}
		phoneError = '';
		sendingCode = true;
		try {
			const challenge = await services.auth.requestPhoneCode(phone);
			savePendingChallenge(challenge);
			await goto(resolve('/(auth)/verify-phone'));
		} catch (e) {
			phoneError = errorMessage(e);
		} finally {
			sendingCode = false;
		}
	}
</script>

<svelte:head><title>Sign up · DaGrouper Analysis</title></svelte:head>

<h1 class="font-display text-3xl">Join your community station</h1>
<p class="mt-2 text-muted">Sign up or sign in. No password needed.</p>

<div class="mt-8 space-y-6">
	<button
		class="btn btn-ghost w-full py-3"
		onclick={() => (pickerOpen = true)}
		disabled={googleBusy}
	>
		<svg viewBox="0 0 24 24" class="size-5" aria-hidden="true">
			<path
				fill="#4285F4"
				d="M22.6 12.2c0-.8-.1-1.5-.2-2.2H12v4.2h6a5 5 0 0 1-2.2 3.4v2.8h3.5c2.1-1.9 3.3-4.8 3.3-8.2Z"
			/>
			<path
				fill="#34A853"
				d="M12 23c3 0 5.5-1 7.3-2.7l-3.5-2.8c-1 .7-2.3 1-3.8 1-2.9 0-5.4-2-6.3-4.6H2v2.9A11 11 0 0 0 12 23Z"
			/>
			<path fill="#FBBC05" d="M5.7 13.9a6.6 6.6 0 0 1 0-4.2V6.8H2a11 11 0 0 0 0 9.9l3.7-2.8Z" />
			<path
				fill="#EA4335"
				d="M12 5.4c1.6 0 3.1.6 4.2 1.7l3.1-3.1A11 11 0 0 0 2 6.8l3.7 2.9C6.6 7.3 9.1 5.4 12 5.4Z"
			/>
		</svg>
		{googleBusy ? 'Signing in…' : 'Continue with Google'}
	</button>

	<div class="flex items-center gap-3 text-xs tracking-wide text-muted uppercase">
		<span class="h-px flex-1 bg-line"></span>or<span class="h-px flex-1 bg-line"></span>
	</div>

	<form class="space-y-4" onsubmit={continueWithPhone} novalidate>
		<div>
			<label class="label" for="phone">Phone number</label>
			<PhoneInput bind:value={phone} error={phoneError} />
		</div>
		<button class="btn btn-primary w-full py-3" disabled={sendingCode}>
			{sendingCode ? 'Sending code…' : 'Continue with phone'}
		</button>
	</form>

	{#if formError}<p class="field-error" role="alert">{formError}</p>{/if}

	{#if isMockDataSource}
		<details class="rounded-md border border-line bg-surface px-4 py-3 text-sm text-muted">
			<summary class="cursor-pointer font-medium text-ink">Demo accounts</summary>
			<ul class="mt-2 list-disc space-y-1 pl-5">
				<li>
					Google: Tesfaye G. (institution head), Dr. Marcus L. (researcher), Leila M. (platform
					admin) and Sarah K. (custodian) already have accounts.
				</li>
				<li>Google: Hana Bekele and Amina Yusuf are new, so they go through onboarding.</li>
				<li>Phone: Ethiopia 911 000 001 is Abebe T. Any other number creates a new account.</li>
			</ul>
		</details>
	{/if}
</div>

<p class="mt-10 text-xs text-muted">
	By continuing you agree to the DaGrouper terms and privacy notice.
</p>

{#if pickerOpen}
	<GoogleAccountPicker
		accounts={data.googleAccounts}
		onPick={continueWithGoogle}
		onClose={() => (pickerOpen = false)}
	/>
{/if}
