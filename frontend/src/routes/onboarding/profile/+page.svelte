<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto, invalidate } from '$app/navigation';
	import OtpForm from '#lib/components/OtpForm.svelte';
	import PhoneInput from '#lib/components/PhoneInput.svelte';
	import type { PhoneChallenge } from '#lib/domain/types.ts';
	import { validateProfile } from '#lib/domain/validation.ts';
	import { errorMessage, services, ServiceError } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const user = $derived(data.user);

	// Seed the form once from the account (Google pre-fills name and email).
	// svelte-ignore state_referenced_locally
	let fullName = $state(data.user.fullName);
	// svelte-ignore state_referenced_locally
	let email = $state(data.user.email ?? '');
	let phone = $state('');

	let errors = $state<Record<string, string>>({});
	let formError = $state('');
	let saving = $state(false);
	let challenge = $state<PhoneChallenge | null>(null);
	let sendingCode = $state(false);

	async function sendCode() {
		errors.phone = '';
		sendingCode = true;
		try {
			challenge = await services.profile.startPhoneVerification(phone);
		} catch (e) {
			errors.phone = errorMessage(e);
		} finally {
			sendingCode = false;
		}
	}

	async function confirmCode(code: string) {
		await services.profile.confirmPhoneVerification(challenge!.requestId, code);
		challenge = null;
		formError = '';
		await invalidate('app:session');
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		formError = '';
		errors = validateProfile({ fullName, email });
		if (!user.phoneVerified) {
			formError = 'Verify your phone number to continue.';
		}
		if (Object.keys(errors).length || formError) return;
		saving = true;
		try {
			await services.profile.updateProfile({ fullName, email });
			await invalidate('app:session');
			await goto(resolve('/onboarding/address'));
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			formError = errorMessage(e);
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head><title>Your profile · DaGrouper Analysis</title></svelte:head>

<h1 class="font-display text-3xl">Tell us about you</h1>
<p class="mt-2 mb-8 text-muted">
	{user.authProvider === 'google'
		? 'We filled in what Google shared. Add a phone number so your station can reach you.'
		: 'Your phone is verified. Add your name and email.'}
</p>

<form class="space-y-5" onsubmit={submit} novalidate>
	<div>
		<label class="label" for="fullName">Full name</label>
		<input
			id="fullName"
			class={['input', errors.fullName && 'input-error']}
			autocomplete="name"
			bind:value={fullName}
			aria-invalid={Boolean(errors.fullName)}
		/>
		{#if errors.fullName}<p class="field-error">{errors.fullName}</p>{/if}
	</div>

	<div>
		<label class="label" for="email">Email</label>
		<input
			id="email"
			type="email"
			class={['input', errors.email && 'input-error']}
			autocomplete="email"
			bind:value={email}
			aria-invalid={Boolean(errors.email)}
		/>
		{#if errors.email}<p class="field-error">{errors.email}</p>{/if}
	</div>

	<div>
		<span class="label">Phone number</span>
		{#if user.phoneVerified}
			<p
				class="flex items-center justify-between rounded-xl border border-line bg-canvas px-3.5 py-2.5"
			>
				<span>{user.phone}</span>
				<span
					class="rounded-full bg-lime/15 px-2.5 py-0.5 text-xs font-medium text-lime ring-1 ring-lime/40"
					>Verified</span
				>
			</p>
		{:else if challenge}
			<div class="rounded-md border border-line bg-raised p-4">
				<p class="mb-4 text-sm text-muted">
					Enter the code sent to <strong class="text-ink">{challenge.phone}</strong>.
					<button type="button" class="link" onclick={() => (challenge = null)}>Change</button>
				</p>
				{#key challenge.requestId}
					<OtpForm
						{challenge}
						onVerify={confirmCode}
						onResend={sendCode}
						submitLabel="Verify phone"
					/>
				{/key}
			</div>
		{:else}
			<PhoneInput bind:value={phone} id="phone" error={errors.phone} />
			<button
				type="button"
				class="btn btn-ghost mt-3"
				onclick={sendCode}
				disabled={sendingCode || !phone}
			>
				{sendingCode ? 'Sending…' : 'Send verification code'}
			</button>
		{/if}
	</div>

	{#if formError}<p class="field-error" role="alert">{formError}</p>{/if}

	<button class="btn btn-primary w-full py-3" disabled={saving}>
		{saving ? 'Saving…' : 'Continue'}
	</button>
</form>
