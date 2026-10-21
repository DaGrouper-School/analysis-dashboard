<script lang="ts">
	import { resolve } from '$app/paths';
	import type { EffectiveAlertRule } from '#lib/domain/types.ts';

	/** Says whose alert levels a chart or badge uses, linking to the classroom's Alerts section. */
	let { classroomId, rules }: { classroomId: string; rules: EffectiveAlertRule[] } = $props();

	// A parameter's levels are either all the classroom's own or all the defaults.
	const own = $derived(rules.some((r) => r.source === 'classroom'));
	// The path is resolved; only the section's hash is added to it.
	const href = $derived(resolve('/(app)/classrooms/[id]', { id: classroomId }) + '#alerts');
</script>

<p class="mt-2 text-xs text-muted">
	Levels {own ? 'set by your classroom' : 'from the platform defaults'} ·
	<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
	<a {href} class="underline hover:text-ink">Classroom alerts</a>
</p>
