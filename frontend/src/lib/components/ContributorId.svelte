<script lang="ts">
	import { resolve } from '$app/paths';

	/**
	 * A contributor's participant ID (AP-39), always shown with their name: it links to their
	 * profile on the platform, so anyone can check who a value is credited to.
	 */
	let { participantId }: { participantId: number } = $props();
</script>

<a
	class="font-mono text-xs text-muted underline decoration-line-strong underline-offset-2 hover:text-teal"
	href={resolve('/contributors/[participantId]', { participantId: String(participantId) })}
	title="Check this contributor on the Da Grouper School platform">#{participantId}</a
>
