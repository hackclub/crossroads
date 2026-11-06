<script>
	// One long train shared by Home, Shop and Boxes. It lives in the (app)
	// layout, so it stays put between those pages and slides to the car for
	// the page you're on, passing every car in between.
	//
	//   rear ◀  stamps · cargo · ticket (Shop) · cargo · dashboard (Home)  ▶ front
	//
	// Going Home → Shop the train moves right to bring the ticket car in;
	// Boxes → Home it rolls left past them all. The page's own car shows its
	// live content (handed over with <Station>); the others show a preview and
	// can't be focused while they're off to the side.
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { rail, STATIONS } from './rail.svelte.js';
	import StampCar from './cars/StampCar.svelte';
	import HomePreview from './cars/HomePreview.svelte';
	import ShopPreview from './cars/ShopPreview.svelte';
	import wood from './assets/rail/wood.webp';

	let { at } = $props(); // the current path: one of STATIONS

	// Which station the train is drawn at, and how long the move there takes:
	// longer the more cars it passes.
	let shown = $state(untrack(() => STATIONS.indexOf(at)));
	let duration = $state(0);
	$effect(() => {
		const target = STATIONS.indexOf(at);
		untrack(() => {
			if (target === shown) return;
			duration = 0.9 + 0.6 * Math.abs(target - shown);
			shown = target;
		});
	});

	// A two-finger sideways scroll (a trackpad swipe) goes to the next car:
	// scrolling right goes toward the front (Boxes → Shop → Home), left toward
	// the back. One swipe moves one car, however long the scroll's momentum
	// keeps going, and it doesn't steal sideways scrolls from things that
	// scroll sideways themselves, like the stamp tray.
	const SWIPE = 80; // px of sideways scroll that count as a swipe
	let swiped = 0;
	let locked = false;
	let quiet; // timer: the gesture has ended once the wheel goes quiet

	function scrollsSideways(el, dx) {
		for (; el && el !== document.body; el = el.parentElement) {
			const { overflowX } = getComputedStyle(el);
			if ((overflowX === 'auto' || overflowX === 'scroll') && el.scrollWidth > el.clientWidth) {
				if (dx > 0 ? el.scrollLeft + el.clientWidth < el.scrollWidth - 1 : el.scrollLeft > 0) return true;
			}
		}
		return false;
	}

	function onWheel(e) {
		if (e.ctrlKey || Math.abs(e.deltaX) <= Math.abs(e.deltaY)) return; // pinch zoom, or mostly up/down
		if (document.querySelector('dialog[open]') || scrollsSideways(e.target, e.deltaX)) return;
		e.preventDefault(); // also stops the browser's swipe-to-go-back
		clearTimeout(quiet);
		quiet = setTimeout(() => {
			swiped = 0;
			locked = false;
		}, 250);
		if (locked) return;
		swiped += e.deltaX * (e.deltaMode === 1 ? 16 : 1);
		if (Math.abs(swiped) < SWIPE) return;
		const next = STATIONS.indexOf(at) + Math.sign(swiped);
		locked = true;
		if (next >= 0 && next < STATIONS.length) goto(STATIONS[next]);
	}

	$effect(() => {
		window.addEventListener('wheel', onWheel, { passive: false });
		return () => {
			window.removeEventListener('wheel', onWheel);
			clearTimeout(quiet);
		};
	});
</script>

<div class="rail" style="--k: {shown}; --wood: url({wood})">
	<div class="track rails" aria-hidden="true"></div>
	<div class="arrive">
		<div class="strip" style="transition-duration: {duration}s">
			<div class="end rear" aria-hidden="true"><span class="coupler"><i class="knob"></i><i class="bar"></i><i class="block"></i></span></div>

			<section class="car station" aria-label="Your stamps" inert={at !== '/boxes'}>
				<StampCar />
			</section>
			<div class="joint" aria-hidden="true"><span class="coupler"><i class="block"></i><i class="bar"></i><i class="knob"></i><i class="bar"></i><i class="block"></i></span></div>
			<div class="car cargo" aria-hidden="true"><span class="door"></span></div>
			<div class="joint" aria-hidden="true"><span class="coupler"><i class="block"></i><i class="bar"></i><i class="knob"></i><i class="bar"></i><i class="block"></i></span></div>

			<section class="car station" aria-label="Your ticket" inert={at !== '/shop'}>
				{#if at === '/shop' && rail.shop}{@render rail.shop()}{:else}<ShopPreview />{/if}
			</section>
			<div class="joint" aria-hidden="true"><span class="coupler"><i class="block"></i><i class="bar"></i><i class="knob"></i><i class="bar"></i><i class="block"></i></span></div>
			<div class="car cargo" aria-hidden="true"><span class="door"></span></div>
			<div class="joint" aria-hidden="true"><span class="coupler"><i class="block"></i><i class="bar"></i><i class="knob"></i><i class="bar"></i><i class="block"></i></span></div>

			<section class="car station front" aria-label="Your progress" inert={at !== '/home'}>
				{#if at === '/home' && rail.home}{@render rail.home()}{:else}<HomePreview />{/if}
			</section>
			<div class="end front" aria-hidden="true"><span class="coupler"><i class="block"></i><i class="bar"></i><i class="knob"></i></span></div>
		</div>
	</div>
</div>

<style>
	.rail {
		/* Sizes in terms of the rail's own width (a container), so the main
		   cars line up with the page's content column below. */
		container-type: inline-size;
		--gutter: 32px;
		position: relative;
		margin-top: 24px;
		padding-block: 16px;
		overflow-x: clip;
	}
	.strip {
		--W: min(var(--content), 100cqw - 2 * var(--gutter)); /* a station car */
		--C: calc(var(--W) * 0.42); /* a cargo car */
		--g: clamp(16px, 2.8cqw, 48px); /* coupling gap */
		position: relative;
		left: 50%;
		display: flex;
		align-items: stretch;
		width: max-content;
		/* Centre car k (0 stamps, 1 ticket, 2 dashboard) on the page. */
		transform: translateX(calc(-1 * (var(--k) * (var(--W) + var(--C) + 2 * var(--g)) + var(--W) / 2)));
		transition-property: transform;
		transition-timing-function: cubic-bezier(0.65, 0, 0.35, 1);
	}
	.rails { position: absolute; inset: 0; }

	.car { position: relative; z-index: 1; flex: none; border-radius: var(--radius); }
	/* Every car is as tall as the tallest; content sits in the middle of it.
	   (Not ".main": that's the page content wrapper in app.css.) */
	.station { display: flex; flex-direction: column; justify-content: center; width: var(--W); min-height: 220px; background: var(--car-bg); }
	/* A plank-sided boxcar with a sliding door. */
	.cargo {
		width: var(--C);
		background:
			repeating-linear-gradient(90deg, rgb(0 0 0 / 0.35) 0 2px, transparent 2px 56px),
			linear-gradient(rgb(38 30 22 / 0.78), rgb(38 30 22 / 0.78)),
			var(--wood) center / cover;
		box-shadow: inset 0 0 0 3px rgb(0 0 0 / 0.35);
	}
	.door {
		position: absolute;
		inset: 18px 34% 18px 34%;
		border: 3px solid rgb(0 0 0 / 0.45);
		border-radius: 4px;
		background: linear-gradient(rgb(0 0 0 / 0.18), rgb(0 0 0 / 0.18));
	}

	/* Couplers, as in Train.svelte: #262626 parts tucked 32px under each car. */
	.joint, .end { position: relative; z-index: 0; flex: none; }
	.joint { width: var(--g); }
	.end { width: 0; }
	.coupler {
		position: absolute;
		top: 50%;
		display: flex;
		align-items: center;
		height: 36px;
		translate: 0 -50%;
	}
	.coupler i { display: block; flex: none; background: #262626; border: 1px solid #2b2a2a; }
	.coupler .block { width: 39px; height: 36px; }
	.coupler .bar { flex: 1; min-width: 4px; height: 12px; margin-inline: -3px; }
	.coupler .knob { position: relative; z-index: 1; width: 18px; height: 18px; margin-inline: -11px; border-radius: 8px; }
	.joint .coupler { left: -32px; right: -32px; }
	.end .coupler { width: 72px; }
	.end .bar { flex: none; width: 20px; }
	.rear .coupler { right: -33px; }
	.front .coupler { left: -33px; }

	/* Arriving from another page (or a fresh load): the train is a quarter of
	   the window to the left and rolls into place, slowing to a stop. "backwards",
	   not "both": once it stops, nothing keeps the train on its own GPU layer,
	   which made the stamps and text in the cars render blurry. */
	.arrive { animation: roll-in 1.6s cubic-bezier(0.16, 1, 0.3, 1) backwards; }
	@keyframes roll-in {
		from { translate: -25cqw 0; }
	}
	@media (prefers-reduced-motion: reduce) {
		.arrive { animation: none; }
		.strip { transition: none; }
	}
	@media (max-width: 820px) {
		.rail { --gutter: 16px; margin-top: 16px; }
	}
</style>
