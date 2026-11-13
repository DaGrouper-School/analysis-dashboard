<script lang="ts">
	import { resolve } from '$app/paths';
	import { goto, invalidate, invalidateAll } from '$app/navigation';
	import AddressForm from '#lib/features/places/AddressForm.svelte';
	import Avatar from '#lib/components/Avatar.svelte';
	import ClassroomList from '#lib/features/classroom/ClassroomList.svelte';
	import InstallAppCard from '#lib/features/app/InstallAppCard.svelte';
	import GuardianConsentCard from '#lib/features/consent/GuardianConsentCard.svelte';
	import Icon from '#lib/components/Icon.svelte';
	import VerticalTabs, { type TabItem } from '#lib/components/VerticalTabs.svelte';
	import { resetDb } from '#lib/data/mock-db.ts';
	import type { Address } from '#lib/domain/types.ts';
	import { validateProfile } from '#lib/domain/validation.ts';
	import { errorMessage, isMockDataSource, services, ServiceError } from '#lib/services/index.ts';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const user = $derived(data.user);

	/** Each tab's icon, also drawn by its section title. */
	const ICONS = {
		profile: 'user',
		address: 'pin',
		classrooms: 'users',
		app: 'device',
		demo: 'database'
	};
	type Tab = keyof typeof ICONS;
	let tab = $state<Tab>();
	const tabs: TabItem<Tab>[] = [
		{ id: 'profile', label: 'Profile & contact', icon: ICONS.profile },
		{ id: 'address', label: 'Address', icon: ICONS.address },
		{ id: 'classrooms', label: 'Classrooms', icon: ICONS.classrooms },
		{ id: 'app', label: 'App', icon: ICONS.app },
		...(isMockDataSource ? [{ id: 'demo' as const, label: 'Demo data', icon: ICONS.demo }] : [])
	];

	const ROLE_LABELS: Record<string, string> = {
		community_scientist: 'Community scientist',
		custodian: 'Community custodian',
		facilitator: 'Facilitator',
		institution_head: 'Institution head',
		researcher: 'Researcher'
	};

	// svelte-ignore state_referenced_locally
	let fullName = $state(data.user.fullName);
	// svelte-ignore state_referenced_locally
	let email = $state(data.user.email ?? '');
	let errors = $state<Record<string, string>>({});
	let profileMessage = $state('');
	let addressMessage = $state('');
	let saving = $state(false);

	let savingPublic = $state(false);
	let publicMessage = $state('');

	async function setPublicProfile(show: boolean) {
		savingPublic = true;
		publicMessage = '';
		try {
			await services.profile.setPublicProfile(show);
			await invalidate('app:session');
			publicMessage = show ? 'Your profile is public.' : 'Your profile is private.';
		} catch (e) {
			publicMessage = errorMessage(e);
		} finally {
			savingPublic = false;
		}
	}

	async function saveProfile(event: SubmitEvent) {
		event.preventDefault();
		profileMessage = '';
		errors = validateProfile({ fullName, email });
		if (Object.keys(errors).length) return;
		saving = true;
		try {
			await services.profile.updateProfile({ fullName, email });
			await invalidate('app:session');
			profileMessage = 'Profile saved.';
		} catch (e) {
			if (e instanceof ServiceError) errors = e.fields;
			profileMessage = errorMessage(e);
		} finally {
			saving = false;
		}
	}

	async function saveAddress(address: Address) {
		addressMessage = '';
		await services.profile.updateAddress(address);
		await invalidate('app:session');
		addressMessage = 'Address saved.';
	}

	async function signOut() {
		await services.auth.signOut();
		await invalidate('app:session');
		await goto(resolve('/(auth)/signup'));
	}

	async function resetDemo() {
		if (!confirm('Reset all demo data? Accounts you created in this browser will be removed.'))
			return;
		resetDb();
		await invalidateAll();
		await goto(resolve('/(auth)/signup'));
	}
</script>

<svelte:head><title>Settings · DaGrouper Analysis</title></svelte:head>

<div class="flex items-center gap-4">
	<Avatar name={user.fullName} src={user.avatarUrl} size="lg" />
	<div>
		<h1 class="font-display text-3xl">{user.fullName}</h1>
		<p class="text-sm text-muted">
			{user.platformAdmin ? 'Platform admin' : ROLE_LABELS[user.role]} · signed in with {user.authProvider ===
			'google'
				? 'Google'
				: 'phone'}
		</p>
	</div>
	<button class="btn btn-ghost ml-auto" onclick={signOut}
		><Icon name="logout" class="size-4" /> Sign out</button
	>
</div>

<div class="mt-8">
	<VerticalTabs {tabs} bind:active={tab} label="Settings">
		{#if tab === 'profile'}
			<div class="space-y-6">
				<section class="card max-w-2xl">
					<h2 class="flex items-center gap-2 text-lg font-semibold">
						<Icon name={ICONS.profile} class="size-5 text-muted" /> Profile & contact
					</h2>
					<form class="mt-5 space-y-5" onsubmit={saveProfile} novalidate>
						<div>
							<label class="label" for="fullName">Full name</label>
							<input
								id="fullName"
								class={['input', errors.fullName && 'input-error']}
								bind:value={fullName}
							/>
							{#if errors.fullName}<p class="field-error">{errors.fullName}</p>{/if}
						</div>
						<div>
							<label class="label" for="email">Email</label>
							<input
								id="email"
								type="email"
								class={['input', errors.email && 'input-error']}
								bind:value={email}
							/>
							{#if errors.email}<p class="field-error">{errors.email}</p>{/if}
						</div>
						<div>
							<span class="label">Phone number</span>
							<p
								class="flex items-center justify-between rounded-sm border border-line bg-raised px-3.5 py-2.5"
							>
								<span>{user.phone}</span>
								<span
									class="rounded-full bg-lime/15 px-2.5 py-0.5 text-xs font-medium text-lime ring-1 ring-lime/40"
									>Verified</span
								>
							</p>
						</div>
						<div class="flex items-center gap-3">
							<button class="btn btn-primary" disabled={saving}
								><Icon name="check" class="size-4" /> {saving ? 'Saving…' : 'Save profile'}</button
							>
							{#if profileMessage}<p class="text-sm text-muted" role="status">
									{profileMessage}
								</p>{/if}
						</div>
					</form>
				</section>
				<section class="card max-w-2xl">
					<h2 class="flex items-center gap-2 text-lg font-semibold">
						<Icon name="tag" class="size-5 text-muted" /> Participant ID and public profile
					</h2>
					<p class="mt-3 text-sm">
						Your participant ID is
						<a
							class="font-mono text-ink underline decoration-teal underline-offset-4"
							href={resolve('/contributors/[participantId]', {
								participantId: String(user.participantId)
							})}>{user.participantId}</a
						>. It never changes. SAM, data exports and public pages credit every value you record to
						this ID, never to your name or email.
					</p>
					{#if user.minor}
						<p class="mt-3 text-sm text-muted">
							A public profile is for contributors 18 and over. Your values stay credited to your
							ID, so they will show on your profile if you turn it on later.
						</p>
					{:else}
						<label class="mt-4 flex items-start gap-3">
							<input
								id="publicProfile"
								type="checkbox"
								class="mt-0.5"
								checked={user.publicProfile}
								disabled={savingPublic}
								onchange={(e) => setPublicProfile(e.currentTarget.checked)}
							/>
							<span>
								<span class="font-medium text-ink">Show my public profile</span>
								<span class="block text-sm text-muted"
									>Your name, email and role appear on your contributor page and next to your ID on
									classroom pages. Turn it off at any time.</span
								>
							</span>
						</label>
						{#if publicMessage}<p class="mt-2 text-sm text-muted" role="status">
								{publicMessage}
							</p>{/if}
					{/if}
				</section>
				{#if user.minor}
					<GuardianConsentCard consent={data.consent} />
				{/if}
			</div>
		{:else if tab === 'address'}
			<section class="card max-w-2xl">
				<h2 class="flex items-center gap-2 text-lg font-semibold">
					<Icon name={ICONS.address} class="size-5 text-muted" /> Address
				</h2>
				<div class="mt-5">
					<AddressForm initial={user.address} onSave={saveAddress} />
					{#if addressMessage}<p class="mt-3 text-sm text-muted" role="status">
							{addressMessage}
						</p>{/if}
				</div>
			</section>
		{:else if tab === 'classrooms'}
			<ClassroomList
				classrooms={data.classrooms}
				invitations={data.invitations}
				canCreate={data.canCreate}
				platformAdmin={user.platformAdmin}
				head={user.role === 'institution_head'}
				icon={ICONS.classrooms}
			/>
		{:else if tab === 'app'}
			<InstallAppCard icon={ICONS.app} />
		{:else}
			<section
				class="card flex max-w-2xl flex-wrap items-center justify-between gap-4 border-dashed"
			>
				<div>
					<h2 class="flex items-center gap-2 font-semibold">
						<Icon name={ICONS.demo} class="size-5 text-muted" /> Demo data
					</h2>
					<p class="text-sm text-muted">
						Everything is stored in this browser's localStorage. Reset to restore the seeded
						personas.
					</p>
				</div>
				<button class="btn btn-ghost" onclick={resetDemo}
					><Icon name="history" class="size-4" /> Reset demo data</button
				>
			</section>
		{/if}
	</VerticalTabs>
</div>
