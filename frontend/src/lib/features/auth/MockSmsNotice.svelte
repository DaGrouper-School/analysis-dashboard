<script lang="ts">
	let { phone, code }: { phone: string; code?: string } = $props();
</script>

{#if code}
	<div
		class="flex items-start gap-3 rounded-md border border-lime-strong/50 bg-lime/25 px-4 py-3 text-sm"
		role="status"
	>
		<span class="mt-0.5 rounded-full bg-lime px-2 py-0.5 text-xs font-semibold text-canvas"
			>Demo</span
		>
		<p>
			No SMS is sent in demo mode. The code for {phone} is
			<strong class="font-mono tracking-widest">{code}</strong>.
		</p>
	</div>
{/if}
