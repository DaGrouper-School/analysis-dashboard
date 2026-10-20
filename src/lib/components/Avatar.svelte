<script lang="ts">
	let {
		name,
		src = null,
		size = 'md'
	}: { name: string; src?: string | null; size?: 'md' | 'lg' } = $props();

	const initials = $derived(
		name
			.split(/\s+/)
			.filter((part) => /[a-z]/i.test(part))
			.slice(0, 2)
			.map((part) => part[0].toUpperCase())
			.join('') || '?'
	);
</script>

{#if src}
	<img {src} alt="" class={['rounded-full object-cover', size === 'lg' ? 'size-16' : 'size-9']} />
{:else}
	<span
		class={[
			'grid shrink-0 place-items-center rounded-full bg-forest font-semibold text-lime',
			size === 'lg' ? 'size-16 text-xl' : 'size-9 text-sm'
		]}
		aria-hidden="true">{initials}</span
	>
{/if}
