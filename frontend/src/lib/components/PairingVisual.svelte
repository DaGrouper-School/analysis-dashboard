<script lang="ts" module>
	export type PairingState = 'idle' | 'scan' | 'handshake' | 'stream' | 'done' | 'error';
</script>

<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';

	/**
	 * The link between this console and a sensor: the console scans, the two handshake, readings
	 * stream across, then the link locks. Drawn in the sensor's registry color (`--accent` on a
	 * parent). Motion stops for people who ask for reduced motion; each state still reads still.
	 */
	let {
		state,
		icon,
		identifier,
		status
	}: {
		state: PairingState;
		/** The sensor's registry icon. */
		icon: string;
		/** Shown on the sensor side of the readout. */
		identifier?: string;
		/** A short word for the readout, e.g. "Scanning". */
		status: string;
	} = $props();
</script>

<div class="pairing" data-state={state} aria-hidden="true">
	<div class="grid-bg"></div>
	<div class="readout left">
		<span class="dot"></span>{status}
	</div>
	{#if identifier}<div class="readout right">{identifier}</div>{/if}

	<div class="stage">
		<div class="node console">
			{#each [0, 1, 2] as i (i)}<span class="ring" style:--i={i}></span>{/each}
			<span class="core"><Icon name="antenna" class="size-6" /></span>
		</div>

		<div class="link">
			<span class="beam"></span>
			{#each [0, 1, 2, 3] as i (i)}<span class="packet" style:--i={i}></span>{/each}
		</div>

		<div class="node device">
			{#each [0, 1] as i (i)}<span class="ring" style:--i={i}></span>{/each}
			<span class="core"><Icon name={icon} class="size-7" /></span>
			<span class="badge">
				<Icon name={state === 'error' ? 'close' : 'check'} class="size-3.5" />
			</span>
		</div>
	</div>
</div>

<style>
	.pairing {
		--accent-soft: color-mix(in oklab, var(--accent) 22%, transparent);
		position: relative;
		overflow: hidden;
		height: 11rem;
		border: 1px solid color-mix(in oklab, var(--accent) 35%, transparent);
		border-radius: 0.375rem;
		background:
			radial-gradient(
				ellipse at 50% 60%,
				color-mix(in oklab, var(--accent) 14%, transparent),
				transparent 70%
			),
			var(--color-canvas);
		color: var(--accent);
	}
	.grid-bg {
		position: absolute;
		inset: 0;
		background-image:
			linear-gradient(var(--color-line) 1px, transparent 1px),
			linear-gradient(90deg, var(--color-line) 1px, transparent 1px);
		background-size: 22px 22px;
		mask-image: radial-gradient(ellipse at center, black 30%, transparent 75%);
		opacity: 0.7;
	}
	.readout {
		position: absolute;
		top: 0.6rem;
		display: flex;
		align-items: center;
		gap: 0.4rem;
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
		font-size: 0.65rem;
		letter-spacing: 0.16em;
		text-transform: uppercase;
		color: var(--color-muted);
	}
	.readout.left {
		left: 0.75rem;
	}
	.readout.right {
		right: 0.75rem;
	}
	.readout .dot {
		width: 6px;
		height: 6px;
		border-radius: 999px;
		background: var(--accent);
		animation: blink 1s steps(2, start) infinite;
	}
	[data-state='done'] .readout .dot,
	[data-state='idle'] .readout .dot {
		animation: none;
	}
	[data-state='error'] .readout .dot {
		animation: none;
		background: var(--color-danger);
	}

	.stage {
		position: absolute;
		inset: 1.75rem 2rem 1rem;
		display: grid;
		grid-template-columns: auto 1fr auto;
		align-items: center;
	}
	.node {
		position: relative;
		display: grid;
		place-items: center;
		width: 4.5rem;
		height: 4.5rem;
	}
	.core {
		position: relative;
		z-index: 1;
		display: grid;
		place-items: center;
		width: 3.5rem;
		height: 3.5rem;
		border-radius: 999px;
		border: 1px solid color-mix(in oklab, var(--accent) 60%, transparent);
		background: color-mix(in oklab, var(--accent) 12%, var(--color-surface));
		transition:
			box-shadow 400ms ease,
			opacity 400ms ease,
			border-color 400ms ease;
	}
	.console .core {
		color: var(--color-ink);
		border-color: var(--color-line-strong);
		background: var(--color-surface);
	}
	.ring {
		position: absolute;
		inset: 0.5rem;
		border-radius: 999px;
		border: 1px solid var(--accent);
		opacity: 0;
	}
	.badge {
		position: absolute;
		z-index: 2;
		right: 0.2rem;
		bottom: 0.2rem;
		display: grid;
		place-items: center;
		width: 1.4rem;
		height: 1.4rem;
		border-radius: 999px;
		background: var(--accent);
		color: var(--color-canvas);
		opacity: 0;
		transform: scale(0.4);
		transition:
			opacity 300ms ease,
			transform 300ms cubic-bezier(0.3, 1.6, 0.5, 1);
	}

	.link {
		position: relative;
		height: 2px;
		margin: 0 0.5rem;
	}
	.beam {
		position: absolute;
		inset: 0;
		background: repeating-linear-gradient(
			90deg,
			var(--color-line-strong) 0 6px,
			transparent 6px 12px
		);
		transition: background 400ms ease;
	}
	.packet {
		position: absolute;
		top: 50%;
		left: 100%;
		width: 8px;
		height: 8px;
		margin: -4px 0 0 -4px;
		border-radius: 999px;
		background: var(--accent);
		box-shadow: 0 0 10px var(--accent);
		opacity: 0;
	}

	/* Idle: the sensor waits, dimmed. */
	[data-state='idle'] .device .core {
		opacity: 0.55;
	}

	/* Scan: the console sends out rings; the sensor is not found yet. */
	[data-state='scan'] .console .ring {
		animation: ping 1.8s ease-out infinite;
		animation-delay: calc(var(--i) * 0.6s);
	}
	[data-state='scan'] .device .core {
		opacity: 0.35;
		border-style: dashed;
	}

	/* Handshake: both ends pulse and the beam marches back and forth. */
	[data-state='handshake'] .ring {
		animation: ping 1.2s ease-out infinite;
		animation-delay: calc(var(--i) * 0.4s);
	}
	[data-state='handshake'] .beam {
		background: repeating-linear-gradient(90deg, var(--accent) 0 6px, transparent 6px 12px);
		background-size: 24px 2px;
		animation: march 0.6s linear infinite alternate;
	}

	/* Stream: readings travel from the sensor to the console. */
	[data-state='stream'] .beam,
	[data-state='done'] .beam {
		background: var(--accent-soft);
	}
	[data-state='stream'] .packet {
		animation: travel 1.6s linear infinite;
		animation-delay: calc(var(--i) * 0.4s);
	}
	[data-state='stream'] .device .core,
	[data-state='done'] .device .core,
	[data-state='done'] .console .core {
		box-shadow: 0 0 24px var(--accent-soft);
	}

	/* Done: the link locks and the sensor gets a check. */
	[data-state='done'] .beam {
		background: var(--accent);
		box-shadow: 0 0 12px var(--accent);
	}
	[data-state='done'] .badge,
	[data-state='error'] .badge {
		opacity: 1;
		transform: scale(1);
	}
	[data-state='done'] .device .ring:first-of-type {
		animation: ping 0.9s ease-out 1;
	}

	/* Error: the link breaks. */
	[data-state='error'] .beam {
		background: repeating-linear-gradient(90deg, var(--color-danger) 0 4px, transparent 4px 16px);
		opacity: 0.6;
	}
	[data-state='error'] .badge {
		background: var(--color-danger);
	}
	[data-state='error'] .device .core {
		border-color: var(--color-danger);
		opacity: 0.6;
	}

	@keyframes ping {
		0% {
			opacity: 0.7;
			transform: scale(1);
		}
		100% {
			opacity: 0;
			transform: scale(2.2);
		}
	}
	@keyframes march {
		to {
			background-position: 24px 0;
		}
	}
	@keyframes travel {
		0% {
			left: 100%;
			opacity: 0;
		}
		12%,
		88% {
			opacity: 1;
		}
		100% {
			left: 0%;
			opacity: 0;
		}
	}
	@keyframes blink {
		to {
			opacity: 0.2;
		}
	}

	/* Still frames: one ring stays out for scanning and handshaking, one packet sits mid-way. */
	@media (prefers-reduced-motion: reduce) {
		.pairing *,
		.pairing *::before {
			animation: none !important;
			transition: none !important;
		}
		[data-state='scan'] .console .ring:first-of-type,
		[data-state='handshake'] .ring:first-of-type {
			opacity: 0.5;
			transform: scale(1.5);
		}
		[data-state='stream'] .packet:first-of-type {
			left: 50%;
			opacity: 1;
		}
	}
</style>
