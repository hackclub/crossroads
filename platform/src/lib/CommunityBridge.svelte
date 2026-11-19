<script>
	// Black mountains in the mist under "Coming soon...", for now. Until the
	// community goal's deadline (Nov 18) this was "Repair Bridge", from the
	// Figma home frame: a broken viaduct and the goal's hours above it. Its
	// art is still in assets/bridge, and GET /api/community still answers.
	import { untrack } from 'svelte';
	import { rail, BRIDGE_FOLD_MS } from './rail.svelte.js';
	import mountains from './assets/mountains/mountains.svg';

	// Open on Home only. It stays mounted on the other train pages, folded
	// away, so arriving at or leaving Home can animate: the section opens and
	// pushes the train down while the mountains rise in, then the words fade
	// in; leaving does the same backwards. It waits for the train to pull in
	// before opening, and the train waits for it to fold before leaving.
	let { open = false } = $props();
	let shown = $state(false);
	$effect(() => {
		if (!open) {
			if (untrack(() => shown)) rail.foldUntil = performance.now() + BRIDGE_FOLD_MS;
			shown = false;
			return;
		}
		// Two frames first, so a fresh load paints it folded, and so the
		// train has noted when it stops.
		let timer;
		let frame = requestAnimationFrame(
			() =>
				(frame = requestAnimationFrame(() => {
					timer = setTimeout(() => (shown = true), Math.max(0, rail.settle - performance.now()));
				}))
		);
		return () => {
			cancelAnimationFrame(frame);
			clearTimeout(timer);
		};
	});
</script>

<section class="goal" class:shown aria-labelledby="goal-title" inert={!open}>
	<div class="fold">
		<div class="stage">
			<div class="scene" aria-hidden="true">
				<img class="mountains" src={mountains} alt="" />
			</div>

			<div class="copy">
				<h2 id="goal-title">Coming soon...</h2>
			</div>
		</div>
	</div>
</section>

<style>
	/* Laid out in the Figma frame's design pixels: --bu is one of them at the
	   page's width (1728 across), as the top bar does. Scene y = frame y - 144.5. */
	.goal { container-type: inline-size; overflow-x: clip; }

	/* Folding: the section's height opens from nothing (pushing the train
	   down), the scene rises in from below, then the words fade in. Each
	   state's transitions are the ones used going into it, so closing runs
	   in reverse order: words out, scene down, then the fold shuts. */
	.goal { display: grid; grid-template-rows: 0fr; transition: grid-template-rows 0.6s cubic-bezier(0.65, 0, 0.35, 1) 0.2s; }
	.goal.shown { grid-template-rows: 1fr; transition: grid-template-rows 0.7s cubic-bezier(0.65, 0, 0.35, 1); }
	.fold { min-height: 0; overflow: hidden; }
	.scene { translate: 0 calc(160 * var(--bu)); transition: translate 0.5s cubic-bezier(0.55, 0, 1, 0.45) 0.1s; }
	.shown .scene { translate: 0 0; transition: translate 0.9s cubic-bezier(0.16, 1, 0.3, 1) 0.1s; }
	.copy { opacity: 0; transition: opacity 0.2s ease-in; }
	.shown .copy { opacity: 1; transition: opacity 0.45s ease-out 0.55s; }
	@media (prefers-reduced-motion: reduce) {
		.goal, .goal.shown, .scene, .shown .scene, .copy, .shown .copy { transition: none; }
	}
	.stage {
		--bu: calc(100cqw / 1728);
		position: relative;
		height: calc(413.5 * var(--bu));
		font-size-adjust: none; /* Crimson Pro at the Figma's sizes */
		color: var(--text);
	}

	/* The scene covers the stage, so the mountains sit at stage coordinates.
	   They run past both edges, below the words, and their foot fades into
	   the photo. */
	.scene { position: absolute; inset: 0; }
	.mountains {
		position: absolute;
		display: block;
		max-width: none; /* wider than the page on purpose */
		left: calc(-83 * var(--bu));
		top: calc(70.5 * var(--bu));
		width: calc(1844 * var(--bu));
		height: calc(343 * var(--bu));
	}

	/* The words, centred on x 851.5 like the Figma's title. */
	.copy {
		position: absolute;
		top: calc(32.5 * var(--bu));
		left: calc(851.5 * var(--bu));
		display: flex;
		flex-direction: column;
		align-items: center;
		width: max(300px, calc(375 * var(--bu)));
		translate: -50% 0;
		line-height: normal;
		text-align: center;
		text-shadow: 0 1px 3px rgb(0 0 0 / 0.8);
	}
	/* The title between two 2px rules (Lines 8 and 9), sitting a touch right
	   of centre, as drawn. */
	h2 {
		display: flex;
		align-items: center;
		gap: calc(14 * var(--bu));
		font-size: max(22px, calc(32 * var(--bu)));
		line-height: normal;
		white-space: nowrap;
		translate: calc(9.5 * var(--bu)) 0;
	}
	h2::before, h2::after {
		content: '';
		width: calc(98 * var(--bu));
		height: 2px;
		background: currentColor;
		translate: 0 calc(2 * var(--bu));
	}

	/* Narrower than ~1100px the words would run into the peaks, so they sit
	   above the scene instead of over it; the mountains keep their shape. */
	@container (max-width: 1100px) {
		.stage { display: flex; flex-direction: column; height: auto; padding-top: 12px; }
		.copy { order: -1; position: static; translate: none; margin: 0 auto; }
		h2 { translate: none; }
		.scene { position: relative; height: calc(343 * var(--bu)); margin-top: 12px; }
		.mountains { top: 0; }
	}
</style>
