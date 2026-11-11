<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto, invalidate } from '$app/navigation';
	import OtpForm from '#lib/features/auth/OtpForm.svelte';
	import PhoneInput from '#lib/features/auth/PhoneInput.svelte';
	import type { PhoneChallenge } from '#lib/domain/types.ts';
	import { validateProfile } from '#lib/domain/validation.ts';
	import { validateGuardian } from '#lib/domain/consent.ts';
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
	// Under 18: a guardian's consent is needed before any observation is sent (AP-04). The link is
	// optional: the classroom can record a signed paper sheet instead (AP-38).
	let under18 = $state(false);
	let guardianName = $state('');
	let guardianContact = $state('');

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
		const sendLink = Boolean(guardianName.trim() || guardianContact.trim());
		if (under18 && !user.minor && sendLink) {
			Object.assign(errors, validateGuardian({ guardianName, guardianContact }));
		}
		if (!user.phoneVerified) {
			formError = 'Verify your phone number to continue.';
		}
		if (Object.keys(errors).length || formError) return;
		saving = true;
		try {
			await services.profile.updateProfile({ fullName, email });
			if (under18 && !user.minor) {
				if (sendLink) {
					await services.profile.requestGuardianConsent({ guardianName, guardianContact });
				} else {
					await services.profile.markMinor();
				}
			}
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
			<PhoneInput bind:value={phone} countries={data.dialCodes} id="phone" error={errors.phone} />
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

	{#if user.minor}
		<p class="rounded-md border border-line bg-raised p-4 text-sm">
			Your guardian's consent is needed. You can look around now, and send observations once it is
			recorded.
		</p>
	{:else}
		<fieldset class="space-y-4">
			<label class="flex items-start gap-2 text-sm">
				<input
					type="checkbox"
					class="mt-0.5 rounded-sm border-line-strong bg-transparent text-teal checked:bg-current focus:ring-teal"
					bind:checked={under18}
				/>
				<span>
					I am under 18
					<span class="block text-xs text-muted"
						>Your parent or guardian gives consent before you send observations: on a paper sheet
						from your facilitator, or by a link we send them.</span
					>
				</span>
			</label>
			{#if under18}
				<p class="text-sm text-muted">
					To send your guardian a link, enter their details. Leave both empty if your classroom
					gives you a paper sheet instead.
				</p>
				<div class="grid gap-4 sm:grid-cols-2">
					<div>
						<label class="label" for="guardianName">Guardian's name (optional)</label>
						<input
							id="guardianName"
							class={['input', errors.guardianName && 'input-error']}
							bind:value={guardianName}
							aria-invalid={Boolean(errors.guardianName)}
						/>
						{#if errors.guardianName}<p class="field-error">{errors.guardianName}</p>{/if}
					</div>
					<div>
						<label class="label" for="guardianContact">Their email or phone (optional)</label>
						<input
							id="guardianContact"
							class={['input', errors.guardianContact && 'input-error']}
							bind:value={guardianContact}
							aria-invalid={Boolean(errors.guardianContact)}
						/>
						{#if errors.guardianContact}<p class="field-error">{errors.guardianContact}</p>{/if}
					</div>
				</div>
			{/if}
		</fieldset>
	{/if}

	{#if formError}<p class="field-error" role="alert">{formError}</p>{/if}

	<button class="btn btn-primary w-full py-3" disabled={saving}>
		{saving ? 'Saving…' : 'Continue'}
	</button>
</form>
