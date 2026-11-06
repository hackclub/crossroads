<script>
	// "Repair Bridge", from the Figma home frame (Group 28 and "train on bridge"):
	// a broken viaduct with a train stopped short of the gap, and the community
	// goal above it. The in-person event only runs if everyone together ships
	// the goal's hours by the deadline (GET /api/community). A ship counts as
	// soon as it's submitted, without waiting for review.
	import { untrack } from 'svelte';
	import { get } from './api.js';
	import { rail, BRIDGE_FOLD_MS } from './rail.svelte.js';
	import bridge from './assets/bridge/bridge.svg';
	import wagon from './assets/bridge/wagon.svg';
	import locomotive from './assets/bridge/locomotive.svg';
	import crumble from './assets/bridge/crumble.svg';

	// Open on Home only. It stays mounted on the other train pages, folded
	// away, so arriving at or leaving Home can animate: the section opens and
	// pushes the train down while the bridge rises in, then the words fade in;
	// leaving does the same backwards. It waits for the train to pull in
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

	let goal = $state(null);
	$effect(() => {
		if (!open) return;
		get('/community')
			.then((g) => (goal = g))
			.catch(() => {});
	});

	const hours = (s) => Math.floor(s / 3600);
	const pct = $derived(goal ? Math.min(goal.seconds / goal.goal_seconds, 1) * 100 : 0);
	const done = $derived(!!goal && goal.seconds >= goal.goal_seconds);
	// "11:59 PM EST on November 15", in the deadline's own Eastern time.
	const deadline = $derived.by(() => {
		if (!goal) return '11:59 PM EST on November 15';
		const at = new Date(goal.ends_at);
		const time = at.toLocaleTimeString('en-US', { timeZone: 'America/New_York', hour: 'numeric', minute: '2-digit', timeZoneName: 'short' });
		const day = at.toLocaleDateString('en-US', { timeZone: 'America/New_York', month: 'long', day: 'numeric' });
		return `${time} on ${day}`;
	});
	// The wagons, then the locomotive at the front, as placed in the Figma.
	const WAGONS = [-14, 127, 268, 409];
	// Loose bricks by the broken ends (Rectangles 204-207), x and y in the scene.
	const BRICKS = [[784, 218.5], [787, 224.5], [917, 218.5], [920, 224.5]];
</script>

<section class="goal" class:shown aria-labelledby="goal-title" inert={!open}>
	<div class="fold">
		<div class="stage">
			<div class="scene" aria-hidden="true">
				<img class="bridge" src={bridge} alt="" />
				<!-- The crumbled ends either side of the gap (Vector 1). -->
				<img class="crumble" src={crumble} alt="" />
				{#each BRICKS as [x, y]}<i class="brick" style="--x: {x}; --y: {y}"></i>{/each}
				<!-- The train on the bridge rolls in as the scene rises. -->
				<div class="consist">
					{#each WAGONS as x}<img class="wagon" src={wagon} alt="" style="--x: {x}" />{/each}
					<img class="loco" src={locomotive} alt="" />
				</div>
			</div>

			<div class="copy">
				<h2 id="goal-title">{done ? 'Bridge repaired' : 'Repair Bridge'}</h2>
				<p>
					{#if done}
						The community shipped {goal ? hours(goal.goal_seconds) : 200} hours together, so the in-person event is on.
					{:else}
						The in-person event can only run if the community ships {goal ? hours(goal.goal_seconds) : 200} hours together, from launch
						to {deadline}.
					{/if}
				</p>
				<div class="meter" style="--pct: {pct}%">
					<!-- The hours so far, with a tick down to the end of the fill. -->
					<span class="now">{goal ? hours(goal.seconds) : 0}H</span>
					<div
						class="gauge"
						role="progressbar"
						aria-label="Hours shipped by the community"
						aria-valuemin="0"
						aria-valuemax={goal ? hours(goal.goal_seconds) : 200}
						aria-valuenow={goal ? hours(goal.seconds) : 0}
						aria-valuetext="{goal ? hours(goal.seconds) : 0} of {goal ? hours(goal.goal_seconds) : 200} hours"
					>
						{#if pct > 0}<span class="fill"></span>{/if}
					</div>
				</div>
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
	/* The train on the bridge starts off the left edge and rolls in while the
	   scene rises, slowing to a stop; it rolls back out as the scene sinks.
	   (transform, not translate: phones use translate to lift the scene.) */
	.consist { transform: translateX(calc(-720 * var(--bu))); transition: transform 0.5s cubic-bezier(0.55, 0, 1, 0.45); }
	.shown .consist { transform: none; transition: transform 1.5s cubic-bezier(0.16, 1, 0.3, 1) 0.1s; }
	.copy { opacity: 0; transition: opacity 0.2s ease-in; }
	.shown .copy { opacity: 1; transition: opacity 0.45s ease-out 0.55s; }
	@media (prefers-reduced-motion: reduce) {
		.goal, .goal.shown, .scene, .shown .scene, .consist, .shown .consist, .copy, .shown .copy { transition: none; }
	}
	.stage {
		--bu: calc(100cqw / 1728);
		position: relative;
		height: calc(413.5 * var(--bu));
		font-size-adjust: none; /* Crimson Pro at the Figma's sizes */
		color: var(--text);
	}

	/* The scene covers the stage, so its pieces sit at stage coordinates. */
	.scene { position: absolute; inset: 0; }
	.scene > *, .consist > * { position: absolute; display: block; max-width: none; } /* the bridge is wider than the page on purpose */
	.consist { inset: 0; }
	/* The viaduct runs past both edges; its foot fades into the photo. */
	.bridge { left: calc(-83 * var(--bu)); top: calc(193.5 * var(--bu)); width: calc(1844 * var(--bu)); height: calc(220 * var(--bu)); }
	.crumble { left: calc(788.6 * var(--bu)); top: calc(195.5 * var(--bu)); width: calc(143.36 * var(--bu)); height: calc(55 * var(--bu)); }
	.brick {
		left: calc(var(--x) * var(--bu));
		top: calc(var(--y) * var(--bu));
		width: calc(11 * var(--bu));
		height: calc(5 * var(--bu));
		border-radius: calc(2 * var(--bu));
		background: #000;
	}
	.wagon { left: calc(var(--x) * var(--bu)); top: calc(136.5 * var(--bu)); width: calc(143 * var(--bu)); height: calc(57 * var(--bu)); }
	.loco { left: calc(548 * var(--bu)); top: calc(123 * var(--bu)); width: calc(151 * var(--bu)); height: calc(70.5 * var(--bu)); }

	/* The words, centred on x 851.5 like the Figma's paragraph and bar. */
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
	   of the paragraph's centre, as drawn. */
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
	p { font-size: max(15px, calc(20 * var(--bu))); line-height: normal; }

	/* Rectangles 208 and 209: a 2px white outline with a solid white fill
	   over its start, both radius 4. The label and its tick (Line 10) sit at
	   the end of the fill. */
	.meter { width: max(180px, calc(269 * var(--bu))); margin-top: calc(3 * var(--bu)); }
	.now {
		position: relative;
		display: block;
		width: max-content;
		left: clamp(0px, var(--pct), 100%);
		translate: -50% 0;
		font-size: max(15px, calc(20 * var(--bu)));
	}
	.now::after {
		content: '';
		position: absolute;
		left: 50%;
		top: calc(100% - 2 * var(--bu));
		width: 1px;
		height: max(3px, calc(4 * var(--bu)));
		background: currentColor;
	}
	/* Not ".bar" or ".track": app.css has both. */
	.gauge { position: relative; height: max(12px, calc(20 * var(--bu))); border-radius: 4px; box-shadow: inset 0 0 0 2px currentColor; }
	.fill { position: absolute; inset: 0 auto 0 0; width: max(8px, var(--pct)); border-radius: 4px; background: currentColor; }

	/* Narrower than ~1100px the words would run into the train, so they sit
	   above the scene instead of over it; the scene keeps its shape. */
	@container (max-width: 1100px) {
		.stage { display: flex; flex-direction: column; height: auto; padding-top: 12px; }
		.copy { order: -1; position: static; translate: none; margin: 0 auto; }
		h2 { translate: none; }
		.meter { margin-top: 18px; }
		.scene { position: relative; height: calc(413.5 * var(--bu) - 123 * var(--bu)); margin-top: 12px; }
		.scene > * { translate: 0 calc(-123 * var(--bu)); }
	}
</style>
