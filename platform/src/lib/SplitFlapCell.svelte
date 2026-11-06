<script>
	// One character of a SplitFlapDisplay. When char changes, it waits `delay`
	// ms, then flips one flap every `flipSpeed` ms through CHARS until it lands
	// on it, like a real departures board.
	import { untrack } from 'svelte';

	let { char, size = 'md', delay = 0, flipSpeed = 35, lowercase = false } = $props();

	// The order the flaps come round in, with capitals or with small letters.
	// A character with no flap would never come round, so it shows as a blank
	// instead of flipping forever.
	const UPPER = " ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789$.,'!?:;+-=%&#@";
	const LOWER = UPPER.toLowerCase();
	const after = (chars, c) => chars[chars.indexOf(c) + 1] ?? chars[0];

	let shown = $state(' '); // on the two fixed halves
	let previous = $state(' '); // on the top flap as it falls away
	let flipping = $state(false);
	let phase = $state('idle'); // each flap: 'top-down', then 'bottom-up', then 'idle'

	$effect(() => {
		const chars = lowercase ? LOWER : UPPER;
		const cased = lowercase ? char.toLowerCase() : char.toUpperCase();
		const target = cased.length === 1 && chars.includes(cased) ? cased : ' ';
		if (untrack(() => shown) === target) {
			flipping = false;
			phase = 'idle';
			return;
		}
		const timers = [];
		const later = (ms, fn) => timers.push(setTimeout(fn, ms));
		let interval;
		later(delay, () => {
			flipping = true;
			interval = setInterval(() => {
				previous = shown;
				shown = after(chars, shown);
				phase = 'top-down';
				later(flipSpeed * 0.4, () => (phase = 'bottom-up'));
				later(flipSpeed * 0.8, () => (phase = 'idle'));
				if (shown === target) {
					clearInterval(interval);
					later(flipSpeed, () => {
						flipping = false;
						phase = 'idle';
					});
				}
			}, flipSpeed);
		});
		// If char changes mid-flip, start again from wherever it got to.
		return () => {
			clearInterval(interval);
			timers.forEach(clearTimeout);
			phase = 'idle';
		};
	});
</script>

<div class="cell {size}" style:--flip="{flipSpeed * 0.4}ms">
	<div class="half top"><span>{shown}</span></div>
	<div class="half bottom"><span>{shown}</span></div>
	{#if flipping && phase === 'top-down'}
		<div class="half top flap falling"><span>{previous}</span></div>
	{:else if flipping && phase === 'bottom-up'}
		<div class="half bottom flap rising"><span>{shown}</span></div>
	{/if}
	<div class="hinge"></div>
	<div class="bevel"></div>
</div>

<style>
	/* Type is fixed here rather than inherited: line-height places the glyphs
	   on the hinge, .app-shell's font-size-adjust would shrink them, and
	   .main's text-shadow would put back the gloss. */
	.cell {
		--px: var(--u, 1px); /* one design pixel; see SplitFlapDisplay */
		position: relative;
		user-select: none;
		font-weight: 700;
		line-height: 1.5;
		font-size-adjust: none;
		text-shadow: none;
		perspective: calc(400 * var(--px));
	}
	.sm { width: calc(26 * var(--px)); height: calc(38 * var(--px)); font-size: calc(16 * var(--px)); }
	.md { width: calc(38 * var(--px)); height: calc(54 * var(--px)); font-size: calc(24 * var(--px)); }
	.lg { width: calc(52 * var(--px)); height: calc(72 * var(--px)); font-size: calc(34 * var(--px)); }

	/* Each half shows half of the same glyph, pushed 48% past its edge. Flat
	   colours, no glyph shadows: matte rather than the original's gloss. */
	.half { position: absolute; inset-inline: 0; height: 50%; overflow: hidden; }
	.top { top: 0; background: #1b1b1b; }
	.bottom { bottom: 0; background: #141414; }
	span { position: absolute; left: 50%; font-family: 'SF Mono', 'Fira Code', 'Courier New', monospace; }
	.top span { bottom: 0; translate: -50% 48%; color: #e8e6e3; }
	.bottom span { top: 0; translate: -50% -48%; color: #d4d2cf; }

	/* The moving flaps: the old top half falls to the hinge, then the new
	   bottom half swings down from it. */
	.flap { z-index: 10; }
	.falling {
		background: #1e1e1e;
		transform-origin: bottom center;
		animation: fall var(--flip) ease-in forwards;
	}
	.rising {
		background: #141414;
		transform-origin: top center;
		animation: rise var(--flip) ease-out forwards;
	}
	@keyframes fall { from { transform: rotateX(0deg); } to { transform: rotateX(-90deg); } }
	@keyframes rise { from { transform: rotateX(90deg); } to { transform: rotateX(0deg); } }

	.hinge {
		position: absolute;
		inset-inline: 0;
		top: 50%;
		translate: 0 -50%;
		z-index: 20;
		pointer-events: none;
		height: calc(2 * var(--px));
		background: rgba(0, 0, 0, 0.8);
	}
	.bevel {
		position: absolute;
		inset: 0;
		z-index: 20;
		pointer-events: none;
		box-shadow: inset 0 var(--px) calc(2 * var(--px)) rgba(0, 0, 0, 0.3);
	}
</style>
