<script module>
	// The tallest the cars have been at each width of the train, for as long
	// as the site stays loaded: coming back from a page without the train, it
	// is the height it was.
	const tallest = {};
</script>

<script>
	// One long train shared by Home, Shop and Boxes. It lives in the (app)
	// layout, so it stays put between those pages and slides to the car for
	// the page you're on, passing every car in between.
	//
	//   front ◀  dashboard (Home) · cargo · ticket (Shop) · cargo · stamps (Boxes)  ▶ rear
	//
	// The cars are in the top bar's order, Home, Shop, Boxes, left to right.
	// Going Home → Shop the train moves left to bring the ticket car in, as if
	// turning to the next page; Boxes → Home it rolls right past them all. The
	// page's own car has its
	// live content (handed over with <Station>); the others draw the same car
	// from what was last loaded, and can't be focused while they're off to
	// the side.
	//
	// Three things keep it steady, whatever page you're on and however fast
	// you click:
	//   - It is the first thing under the top bar on all three pages, so it
	//     never moves up or down.
	//   - Every car is as tall as the tallest any of them has been at this
	//     width (see `floor`), so the train is one height on all three pages
	//     and the page under it starts at the same place.
	//   - The slide is a single CSS transition on the strip's transform. A
	//     click part-way through just gives it a new place to stop: it carries
	//     on from where it is, with nothing queued behind it.
	import { goto } from '$app/navigation';
	import { rail, STATIONS } from './rail.svelte.js';
	import StampCar from './cars/StampCar.svelte';
	import HomePreview from './cars/HomePreview.svelte';
	import ShopPreview from './cars/ShopPreview.svelte';
	import wood from './assets/rail/wood.webp';

	let { at } = $props(); // the current path: one of STATIONS
	// Coming from a page without the train, it rolls in. On a fresh load it's
	// simply there: nothing should move while the page is still filling in.
	const roll = rail.traveled;

	// Which car is centred. The slide takes a little longer the more cars it
	// passes; it eases out, so it answers a click at once.
	const shown = $derived(STATIONS.indexOf(at));
	let from = STATIONS.indexOf(at);
	let duration = $state(0);
	$effect.pre(() => {
		duration = 0.38 + 0.14 * Math.abs(shown - from);
		from = shown;
	});

	// The cars' shared height: never less than the tallest the strip has been
	// at this width. Cars stretch to the tallest one, and which one that is
	// changes with the page (a car's live content can be taller than another's),
	// so without this the train would change height from page to page.
	let railEl;
	let strip;
	let width = 0;
	let floor = $state(0);
	$effect(() => {
		let frame;
		// Not before the fonts are in: text in the fallback font is wider and
		// wraps onto more lines, and that taller height would stick.
		let ready = false;
		const measure = () => {
			if (!ready) return;
			const w = Math.round(railEl.clientWidth);
			if (w !== width) {
				// A new width lays the cars out differently: start from what
				// this width needed before, or from nothing, and measure again
				// once the cars have settled at that.
				width = w;
				floor = tallest[w] ?? 0;
				frame = requestAnimationFrame(measure);
				return;
			}
			const h = Math.ceil(strip.getBoundingClientRect().height);
			if (h > floor) floor = tallest[w] = h;
		};
		// On the next frame, not inside the observer: resizing what it watches
		// from its own callback makes the browser report a "ResizeObserver
		// loop" error.
		const watch = new ResizeObserver(() => {
			cancelAnimationFrame(frame);
			frame = requestAnimationFrame(measure);
		});
		watch.observe(railEl);
		watch.observe(strip);
		document.fonts.ready.then(() => {
			ready = true;
			frame = requestAnimationFrame(measure);
		});
		return () => {
			ready = false;
			cancelAnimationFrame(frame);
			watch.disconnect();
		};
	});

	// A two-finger sideways scroll (a trackpad swipe) goes to the next car:
	// scrolling right goes on down the train (Home → Shop → Boxes), left back
	// toward Home. One swipe moves one car, however long the scroll's momentum
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

<div class="rail" bind:this={railEl} style="--k: {shown}; --wood: url({wood}); --floor: {floor}px">
	<div class="track rails" aria-hidden="true"></div>
	<div class="arrive" class:roll>
		<div class="strip" bind:this={strip} style="transition-duration: {duration}s">
			<div class="end left" aria-hidden="true"><span class="coupler"><i class="knob"></i><i class="bar"></i><i class="block"></i></span></div>

			<section class="car station" aria-label="Your progress" inert={at !== '/home'}>
				{#if at === '/home' && rail.home}{@render rail.home()}{:else}<HomePreview />{/if}
			</section>
			<div class="joint" aria-hidden="true"><span class="coupler"><i class="block"></i><i class="bar"></i><i class="knob"></i><i class="bar"></i><i class="block"></i></span></div>
			<div class="car cargo" aria-hidden="true"><span class="door"></span></div>
			<div class="joint" aria-hidden="true"><span class="coupler"><i class="block"></i><i class="bar"></i><i class="knob"></i><i class="bar"></i><i class="block"></i></span></div>

			<section class="car station" aria-label="Your rail pass" inert={at !== '/shop'}>
				{#if at === '/shop' && rail.shop}{@render rail.shop()}{:else}<ShopPreview />{/if}
			</section>
			<div class="joint" aria-hidden="true"><span class="coupler"><i class="block"></i><i class="bar"></i><i class="knob"></i><i class="bar"></i><i class="block"></i></span></div>
			<div class="car cargo" aria-hidden="true"><span class="door"></span></div>
			<div class="joint" aria-hidden="true"><span class="coupler"><i class="block"></i><i class="bar"></i><i class="knob"></i><i class="bar"></i><i class="block"></i></span></div>

			<section class="car station" aria-label="Your stamps" inert={at !== '/boxes'}>
				<StampCar />
			</section>
			<div class="end right" aria-hidden="true"><span class="coupler"><i class="block"></i><i class="bar"></i><i class="knob"></i></span></div>
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
		/* What the tallest car needs with the usual content, so the train is
		   already full height on a first load, before `floor` has measured it. */
		--car: 320px;
		position: relative;
		left: 50%;
		display: flex;
		align-items: stretch;
		width: max-content;
		/* Centre car k (0 dashboard, 1 ticket, 2 stamps) on the page. */
		transform: translateX(calc(-1 * (var(--k) * (var(--W) + var(--C) + 2 * var(--g)) + var(--W) / 2)));
		transition-property: transform;
		/* Eases out only: it starts moving on the click, and a second click
		   part-way through doesn't have to wait for it to speed up again. */
		transition-timing-function: cubic-bezier(0.22, 1, 0.36, 1);
	}
	.rails { position: absolute; inset: 0; }

	.car { position: relative; z-index: 1; flex: none; border-radius: var(--radius); }
	/* Every car is as tall as the tallest; content sits in the middle of it.
	   (Not ".main": that's the page content wrapper in app.css.) */
	.station { display: flex; flex-direction: column; justify-content: center; width: var(--W); min-height: max(var(--car), var(--floor, 0px)); background: var(--car-bg); }
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
	.left .coupler { right: -33px; }
	.right .coupler { left: -33px; }

	/* Arriving from a page without the train (not a fresh load of one that
	   has it, where nothing should be moving while the page fills in): the
	   train starts a little to the right and rolls into place, front first. "backwards", not
	   "both": once it stops, nothing keeps the train on its own GPU layer,
	   which made the stamps and text in the cars render blurry. */
	.arrive.roll { animation: roll-in 0.5s cubic-bezier(0.22, 1, 0.36, 1) backwards; }
	@keyframes roll-in {
		from { translate: 8cqw 0; }
	}
	@media (prefers-reduced-motion: reduce) {
		.arrive.roll { animation: none; }
		.strip { transition: none; }
	}
	/* Narrower, the ticket car's three columns wrap onto more lines. */
	@container (max-width: 1200px) {
		.strip { --car: 340px; }
	}
	@media (max-width: 820px) {
		.rail { --gutter: 16px; margin-top: 16px; }
		.strip { --car: 460px; }
	}
</style>
