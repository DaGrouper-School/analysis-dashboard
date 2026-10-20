<script lang="ts">
	import { onDestroy } from 'svelte';
	import { isValidOtp } from '#lib/domain/validation.ts';
	import type { PhoneChallenge } from '#lib/domain/types.ts';
	import { errorMessage } from '#lib/services/index.ts';
	import MockSmsNotice from './MockSmsNotice.svelte';

	let {
		challenge,
		onVerify,
		onResend,
		submitLabel = 'Verify'
	}: {
		challenge: PhoneChallenge;
		onVerify: (code: string) => Promise<void>;
		onResend: () => Promise<void>;
		submitLabel?: string;
	} = $props();

	let code = $state('');
	let error = $state('');
	let busy = $state(false);
	let resending = $state(false);
	let now = $state(Date.now());

	const timer = setInterval(() => (now = Date.now()), 500);
	onDestroy(() => clearInterval(timer));

	const secondsLeft = $derived(
		Math.max(0, Math.ceil((Date.parse(challenge.resendAvailableAt) - now) / 1000))
	);

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (!isValidOtp(code)) {
			error = 'Enter the 6-digit code.';
			return;
		}
		busy = true;
		error = '';
		try {
			await onVerify(code);
		} catch (e) {
			error = errorMessage(e);
			code = '';
		} finally {
			busy = false;
		}
	}

	async function resend() {
		resending = true;
		error = '';
		try {
			await onResend();
			code = '';
		} catch (e) {
			error = errorMessage(e);
		} finally {
			resending = false;
		}
	}
</script>

<form class="space-y-5" onsubmit={submit} novalidate>
	<MockSmsNotice phone={challenge.phone} code={challenge.devCode} />
	<div>
		<label class="label" for="otp">6-digit code</label>
		<input
			id="otp"
			class={['input text-center font-mono text-2xl tracking-[0.5em]', error && 'input-error']}
			inputmode="numeric"
			autocomplete="one-time-code"
			maxlength="6"
			placeholder="••••••"
			bind:value={code}
			oninput={() => (code = code.replace(/\D/g, '').slice(0, 6))}
			aria-invalid={Boolean(error)}
			aria-describedby={error ? 'otp-error' : undefined}
		/>
		{#if error}<p id="otp-error" class="field-error" role="alert">{error}</p>{/if}
	</div>
	<button class="btn btn-primary w-full" disabled={busy || code.length !== 6}>
		{busy ? 'Checking…' : submitLabel}
	</button>
	<p class="text-center text-sm text-muted">
		Didn't get it?
		{#if secondsLeft > 0}
			Resend in 0:{String(secondsLeft).padStart(2, '0')}
		{:else}
			<button type="button" class="link" onclick={resend} disabled={resending}>
				{resending ? 'Sending…' : 'Resend code'}
			</button>
		{/if}
	</p>
</form>
