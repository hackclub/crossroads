<script>
	// The public landing page, from the 1728x3060 Figma frame ("MacBook Pro 16\" - 6" in
	// crossroads-draft). Positions are design pixels; --u scales them to the viewport, and
	// below about 780px wide the page stops shrinking and crops the sides instead.
	// The film-*.png strips are baked from Figma's film shader at 2x; re-export them from
	// Figma if the design changes. "Get Started" goes to /signin.
	import thibaultLamTran from '#lib/assets/landing/thibault-lam-tran.jpg';
	import aliKazal from '#lib/assets/landing/ali-kazal.jpg';
	import jessiPena from '#lib/assets/landing/jessi-pena.jpg';
	import filmTopLeft from '#lib/assets/landing/film-top-left.png';
	import filmTopRight from '#lib/assets/landing/film-top-right.png';
	import filmBottomLeft from '#lib/assets/landing/film-bottom-left.png';
	import filmBottomRight from '#lib/assets/landing/film-bottom-right.png';
	import ticketShape from '#lib/assets/landing/ticket-shape.svg';
	import ticketPerforation from '#lib/assets/landing/ticket-perforation.svg';
	import ticketPhoto from '#lib/assets/landing/ticket-photo.jpg';
	import bookdragonPattern from '#lib/assets/landing/bookdragon-pattern.png';
	import ticketCode from '#lib/assets/landing/ticket-code.png';
	import arrowUp from '#lib/assets/landing/arrow-up.svg';
	import arrowDown from '#lib/assets/landing/arrow-down.svg';
	import fadeLine from '#lib/assets/landing/fade-line.svg';
	import SplitFlapDisplay from '#lib/SplitFlapDisplay.svelte';
	import { fade } from 'svelte/transition';
	import { goto } from '$app/navigation';
	import { signIn, isDevMode } from '#lib/api.js';
	import { refresh } from '#lib/session.svelte.js';

	// "Get Started": signed in goes home; signed out goes straight into Hack Club
	// sign-in and comes back home. With the backend in DEV_MODE the real sign-in
	// has no credentials, so it goes to /signin for the dev sign-in instead. The
	// href stays /signin so the link still works without JavaScript.
	let starting = false;
	async function getStarted(event) {
		if (event.metaKey || event.ctrlKey || event.shiftKey || event.button !== 0) return;
		event.preventDefault();
		if (starting) return;
		starting = true;
		try {
			if (await refresh()) return goto('/home');
			if (await isDevMode()) return goto('/signin?next=%2Fhome');
			signIn('/home');
		} catch {
			goto('/signin');
		} finally {
			starting = false;
		}
	}

	// How far the ticket can rotate on each axis, in degrees.
	const MAX_TILT = 7;

	// How far outside the ticket the cursor starts to tilt it, as a multiple of the ticket's height.
	const REACH = 0.6;

	// How quickly the tilt catches up with the cursor, in seconds. Higher is smoother but laggier.
	const SMOOTHING = 0.15;

	// The pointer sets `target`; each frame `tilt` eases toward it so fast sweeps don't jolt the ticket.
	let tilt = $state({ x: 0, y: 0 });
	let target = { x: 0, y: 0 };
	let frame = 0;
	let lastTime = 0;
	let ticketArea;

	function setTarget(x, y) {
		target = { x, y };
		if (!frame) {
			lastTime = performance.now();
			frame = requestAnimationFrame(step);
		}
	}

	function step(now) {
		const ease = 1 - Math.exp(-(now - lastTime) / 1000 / SMOOTHING);
		lastTime = now;
		const x = tilt.x + (target.x - tilt.x) * ease;
		const y = tilt.y + (target.y - tilt.y) * ease;
		if (Math.abs(target.x - x) < 0.01 && Math.abs(target.y - y) < 0.01) {
			tilt = { ...target };
			frame = 0;
			return;
		}
		tilt = { x, y };
		frame = requestAnimationFrame(step);
	}

	// Rotate so the point under the cursor tips away from the viewer. Outside the
	// ticket the tilt aims at the nearest edge and fades out with distance, so it
	// builds up as the cursor approaches instead of snapping on at the edge.
	function tiltTicket(event) {
		if (event.pointerType !== 'mouse') return;
		const box = ticketArea.getBoundingClientRect();
		const px = (event.clientX - box.left) / box.width - 0.5;
		const py = (event.clientY - box.top) / box.height - 0.5;
		const dx = Math.max(box.left - event.clientX, 0, event.clientX - box.right);
		const dy = Math.max(box.top - event.clientY, 0, event.clientY - box.bottom);
		const t = Math.max(0, 1 - Math.hypot(dx, dy) / (box.height * REACH));
		const strength = t * t * (3 - 2 * t);
		if (strength === 0) return resetTicket();
		const clamp = (v) => Math.min(0.5, Math.max(-0.5, v));
		setTarget(-clamp(py) * 2 * MAX_TILT * strength, clamp(px) * 2 * MAX_TILT * strength);
	}

	function resetTicket() {
		if (target.x || target.y) setTarget(0, 0);
	}

	// relatedTarget is null when the pointer leaves the window entirely.
	function leaveWindow(event) {
		if (!event.relatedTarget) resetTicket();
	}

	// Photos of places on the route fade from one to the next on the right of
	// the first screen, with the place's name on a small board in the corner.
	// Stand-ins until the real photos arrive; give each one its `place` and a
	// `src` (an imported image).
	const route = [
		{ place: 'Stop 1', src: null },
		{ place: 'Stop 2', src: null },
		{ place: 'Stop 3', src: null }
	];

	// How long each photo stays, and how long the fade to the next takes, in ms.
	const PHOTO_TIME = 7000;
	const FADE_TIME = 1600;

	let photo = $state(0);
	$effect(() => {
		if (route.length < 2 || matchMedia('(prefers-reduced-motion: reduce)').matches) return;
		const timer = setInterval(() => (photo = (photo + 1) % route.length), PHOTO_TIME);
		return () => clearInterval(timer);
	});

	// Straight tracks down the left of the first screen, under the logo's
	// track, from the top of the window to the film strip: the x of each
	// one's middle in design pixels, evenly spaced across the left half.
	// Cars carrying event details will run on them.
	const tracks = [108, 324, 540, 756];
</script>

<svelte:head>
	<title>Hack Club Crossroads</title>
	<meta name="description" content="Travel across Europe with 30 other teenage builders" />
</svelte:head>

<svelte:window onpointermove={tiltTicket} onpointerout={leaveWindow} />

<main class="page">
	<!-- Positions are design pixels from the 1728x3060 Figma frame; --u scales them. -->
	<div class="frame">
		<img class="abs" src={thibaultLamTran} alt="" style="--x: -104; --y: -587; --w: 1920; --h: 2397" />
		<img class="abs" src={aliKazal} alt="" style="--x: -77; --y: -381; --w: 1920; --h: 2879" />
		<img class="abs multiply" src={jessiPena} alt="" style="--x: -251; --y: -1210; --w: 2400; --h: 3593" />

		<!-- Above the film strip, the first screen splits: tracks on the left, the route's photos on the right. -->
		<div class="yard" aria-hidden="true">
			{#each tracks as x}<div class="yard-track" style="--x: {x}"></div>{/each}
		</div>
		<div class="route" aria-hidden="true">
			{#key photo}
				<div class="photo" transition:fade={{ duration: FADE_TIME }}>
					{#if route[photo].src}
						<img src={route[photo].src} alt="" />
					{:else}
						<div class="stand-in" style:--hue={photo * 110 + 20}></div>
					{/if}
				</div>
			{/key}
		</div>
		<div class="destination" aria-hidden="true">
			<SplitFlapDisplay text={route[photo].place} columns={12} size="sm" centered lowercase />
		</div>

		<!-- Film strips are baked from Figma's shader; the left ones include the effect's bleed past the strip. -->
		<!-- .below moves them, and everything after them, with the height of the first screen. -->
		<img class="abs below" src={filmTopRight} alt="" style="--x: 640; --y: 1113; --w: 1088; --h: 295.5" />
		<img class="abs below" src={filmBottomRight} alt="" style="--x: 637; --y: 2328; --w: 1091; --h: 295.5" />
		<img class="abs below" src={filmTopLeft} alt="" style="--x: 0; --y: 1111; --w: 1094; --h: 295.5" />
		<img class="abs below" src={filmBottomLeft} alt="" style="--x: 0; --y: 2326; --w: 1091; --h: 295.5" />

		<!-- Everything on the ticket is placed relative to the ticket so it can tilt as one piece. -->
		<!-- The tilt is measured against this untransformed box, so the tilting ticket doesn't feed back into it. -->
		<div class="abs" style="--x: 648.105; --y: 312.961; --w: 399.012; --h: 164.139" bind:this={ticketArea}>
			<div
				class="ticket abs"
				style="--x: 0; --y: 0; --w: 399.012; --h: 164.139; --tilt-x: {tilt.x}deg; --tilt-y: {tilt.y}deg"
			>
				<div class="ticket-paper abs" style="--x: 0; --y: 0; --w: 399.012; --h: 164.139; --mask: url({ticketShape})">
					<div class="abs" style="--x: -2.605; --y: -11.561; --w: 411.3; --h: 184.5; background: #e4d6d1"></div>
					<div class="abs" style="--x: 268.295; --y: -21.461; --w: 199.8; --h: 207.9; background: #53758b"></div>
					<img class="perforation abs" src={ticketPerforation} alt="" style="--x: 269.395; --y: 1.039; --w: 165.6; --h: 2" />
					<img class="abs multiply faint" src={ticketPhoto} alt="" style="--x: -33.205; --y: -79.961; --w: 465.975; --h: 310.65" />
					<img class="abs multiply faint opaque" src={bookdragonPattern} alt="" style="--x: -47.605; --y: -162.761; --w: 471.994; --h: 489.192" />
					<p class="ticket-side abs" style="--x: 334.895; --y: 74.839; --w: 52.2; --h: 14.4">Admit One</p>
					<p class="ticket-side abs" style="--x: 292.595; --y: 74.839; --w: 104.4; --h: 14.4">Leaving Paris, France</p>
				</div>
				<div class="outline abs" style="--x: 290.795; --y: 7.339; --w: 81; --h: 149.4; --border: 2; --radius: 2"></div>
				<div class="outline abs" style="--x: 22.595; --y: 14.539; --w: 221.4; --h: 48.6; --border: 3; --radius: 4"></div>
				<img class="abs" src={ticketCode} alt="" style="--x: 296.195; --y: 24.439; --w: 35.202; --h: 35.896" />
				<img class="abs" src={ticketCode} alt="" style="--x: 296.195; --y: 64.039; --w: 35.202; --h: 35.896" />
				<img class="abs" src={ticketCode} alt="" style="--x: 296.195; --y: 103.639; --w: 35.202; --h: 35.896" />
				<p class="ticket-text centered" style="--x: 133.295; --y: 18.139; --w: 246.6; --size: 38">To Qualify:</p>
				<p class="ticket-text centered" style="--x: 134.645; --y: 73.039; --w: 216.9; --size: 40">60 Hours Spent Creating</p>
			</div>
		</div>

		<!-- The title flips into place on a split-flap board when the page loads. The
		     board is decoration; the heading is there for screen readers. It sits
		     centred where the Figma title and its reflection were. -->
		<h1 class="visually-hidden">Crossroads</h1>
		<div class="board-title centered" style="--x: 851; --y: 130" aria-hidden="true">
			<SplitFlapDisplay text="Crossroads" columns={14} size="lg" centered />
		</div>

		<p class="doto centered" style="--x: 852; --y: 550; --w: 842; --size: 40">
			Travel across Europe with 30 other teenage builders
		</p>

		<img class="line abs" src={arrowUp} alt="" style="--x: 841.113; --y: 664.17; --w: 29.887; --h: 5.774; --angle: 90deg" />
		<a class="button abs" href="/signin" onclick={getStarted} style="--x: 742; --y: 688; --w: 228; --h: 67">
			<span class="doto centered" style="--x: 114; --y: 15; --size: 32">Get Started</span>
		</a>
		<img class="drop-line" src={fadeLine} alt="" />

		<h2 class="doto centered below" style="--x: 875.5; --y: 1450; --w: 803; --size: 32">What is Hack Club Crossroads?</h2>
		<p class="doto centered below" style="--x: 877.5; --y: 1510; --w: 803; --size: 24">
			Crossroads is a <a href="https://hackclub.com" target="_blank" rel="noopener">Hack Club</a> Event running January
			8–12 where a group of 30 teens build projects for 60 hours, then travel by train across Europe together.
			Everything (including flights, food and accommodation) is totally free for teenagers who qualify by building a
			project. Can't come? We're also providing funding and prizes for every teen who ships a technical project.
		</p>
		<h2 class="doto centered below" style="--x: 888.5; --y: 1765; --w: 803; --size: 32">What is Hack Club Crossroads?</h2>

		<img class="line abs below" src={arrowDown} alt="" style="--x: 772.056; --y: 2705.057; --w: 169.887; --h: 5.774; --angle: -90deg" />
		<a class="button abs below" href="/signin" onclick={getStarted} style="--x: 742; --y: 2813; --w: 228; --h: 67">
			<span class="doto centered" style="--x: 114; --y: 15; --size: 32">Get Started</span>
		</a>

		<p class="doto centered nowrap below" style="--x: 856.5; --y: 3010; --size: 16">Fuffilment Bounty + Privacy and Terms</p>
	</div>

	<svg class="filters" aria-hidden="true">
		<filter id="opaque">
			<feColorMatrix values="1 0 0 0 0  0 1 0 0 0  0 0 1 0 0  0 0 0 0 1" />
		</filter>
	</svg>
</main>

<style>
	:global(*) {
		box-sizing: border-box;
	}

	:global(body) {
		margin: 0;
		background: #000;
	}

	/* The platform's global img { max-width: 100% } would squash the oversized
	   background photos; positions here are exact. */
	.frame :global(img) {
		max-width: none;
	}

	.page {
		container: page / inline-size;
		overflow-x: clip;
	}

	/*
	 * The frame scales with the viewport so everything stays where Figma put it.
	 * Below ~780px wide it stops shrinking and the sides get cropped instead,
	 * so the text in the middle stays readable.
	 */
	.frame {
		--u: max(100cqw / 1728, 0.45px);
		/* The first screen: the visitor's window, with the middle of the top
		   film strip along its bottom edge. It's never shorter than 935, so on
		   very short windows the strip moves below the fold rather than
		   covering Get Started. */
		--screen: max(100svh, 935 * var(--u));
		/* How far the strip, and everything below it, moves from where Figma
		   put it: its top at 1111, its middle at 1258.75. */
		--shift: calc(var(--screen) - 1258.75 * var(--u));
		position: relative;
		width: calc(1728 * var(--u));
		height: calc(3060 * var(--u) + var(--shift));
		margin-left: calc(50cqw - 864 * var(--u));
		overflow: hidden;
		color: #fff;
	}

	.abs,
	.centered {
		position: absolute;
		left: calc(var(--x) * var(--u));
		top: calc(var(--y) * var(--u));
		margin: 0;
	}

	.below {
		top: calc(var(--y) * var(--u) + var(--shift));
	}

	.abs {
		display: block;
		width: calc(var(--w) * var(--u));
		height: calc(var(--h) * var(--u));
	}

	.centered {
		width: calc(var(--w) * var(--u));
		translate: -50% 0;
		text-align: center;
	}

	.nowrap {
		width: auto;
		white-space: nowrap;
	}

	.multiply {
		mix-blend-mode: multiply;
		object-fit: cover;
	}

	.faint {
		opacity: 0.4;
	}

	/* Figma draws the pattern PNG as if it had no alpha channel; this matches it. */
	.opaque {
		filter: url(#opaque);
	}

	.filters {
		position: absolute;
		width: 0;
		height: 0;
	}

	.ticket {
		transform: perspective(calc(1000 * var(--u))) rotateX(var(--tilt-x)) rotateY(var(--tilt-y));
		will-change: transform;
	}

	/* Every layer of the paper is clipped to the ticket's notched outline. */
	.ticket-paper {
		mask: var(--mask) 0 0 / 100% 100% no-repeat;
	}

	@media (prefers-reduced-motion: reduce) {
		.ticket {
			transform: none;
		}
	}

	.perforation {
		transform-origin: 0 0;
		rotate: 90deg;
	}

	.ticket-side {
		rotate: -90deg;
		font-family: 'Playfair Display', serif;
		font-size: calc(12 * var(--u));
		line-height: normal;
		color: #000;
		overflow-wrap: break-word;
	}

	.outline {
		border: calc(var(--border) * var(--u)) solid #000;
		border-radius: calc(var(--radius) * var(--u));
	}

	.ticket-text {
		font-family: 'Cute Font', sans-serif;
		font-size: calc(var(--size) * var(--u));
		line-height: calc(42 * var(--u));
		color: #000;
		overflow-wrap: break-word;
	}

	.doto {
		font-family: Doto, monospace;
		font-weight: 400;
		font-variation-settings: 'ROND' 0;
		font-size: calc(var(--size) * var(--u));
		line-height: normal;
		overflow-wrap: break-word;
	}

	/* As wide and tall as the board, so .centered centres it on --x and the
	   rails centre on it. */
	.board-title {
		display: flex;
		width: auto;
	}

	/* A plain black track behind the board, centred on it and long enough to
	   cross the whole frame, which clips it. Ties 82 long and 20 wide, every
	   36, with one under the board's middle so they fall evenly either side;
	   two rails 6 thick, 10 in from the ties' ends. */
	.board-title::before {
		content: '';
		position: absolute;
		z-index: -1;
		top: 50%;
		left: 50%;
		width: calc(2 * 1728 * var(--u));
		height: calc(82 * var(--u));
		translate: -50% -50%;
		background:
			linear-gradient(#000 0 0) left 0 top calc(10 * var(--u)) / 100% calc(6 * var(--u)) no-repeat,
			linear-gradient(#000 0 0) left 0 bottom calc(10 * var(--u)) / 100% calc(6 * var(--u)) no-repeat,
			linear-gradient(90deg, transparent calc(8 * var(--u)), #000 0 calc(28 * var(--u)), transparent 0)
				center / calc(36 * var(--u)) 100% repeat-x;
	}

	.visually-hidden {
		position: absolute;
		width: 1px;
		height: 1px;
		margin: -1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
	}

	.line {
		rotate: var(--angle);
	}

	/* From under Get Started down to the film strip, however far that is:
	   the fade line, turned on its end so it fades out downwards. */
	.drop-line {
		position: absolute;
		left: calc(857.5 * var(--u));
		top: calc(762.5 * var(--u));
		width: calc(var(--screen) - 915 * var(--u));
		height: var(--u);
		transform-origin: 0 0;
		rotate: 90deg;
	}

	/* The two halves above the film strip. */
	.yard,
	.route {
		position: absolute;
		top: 0;
		height: calc(var(--screen) - 147.75 * var(--u));
		overflow: hidden;
	}

	.yard {
		left: 0;
		width: calc(864 * var(--u));
	}

	/* Like the logo's track stood on end, a shade lighter so the logo's stays
	   on top, and wider so the gaps between them stay small: ties 96 long and
	   17 wide, every 31; rails 5 thick, 11 in from the ties' ends. */
	.yard-track {
		position: absolute;
		top: 0;
		left: calc(var(--x) * var(--u));
		width: calc(96 * var(--u));
		height: 100%;
		translate: -50% 0;
		background:
			linear-gradient(#262626 0 0) left calc(11 * var(--u)) top 0 / calc(5 * var(--u)) 100% no-repeat,
			linear-gradient(#262626 0 0) right calc(11 * var(--u)) top 0 / calc(5 * var(--u)) 100% no-repeat,
			linear-gradient(transparent calc(7 * var(--u)), #262626 0 calc(24 * var(--u)), transparent 0)
				top / 100% calc(31 * var(--u)) repeat-y;
	}

	/* A soft left edge: the photos fade in over 240, centred on the middle of
	   the screen. */
	.route {
		left: calc(744 * var(--u));
		width: calc(984 * var(--u));
		opacity: 0.3;
		mask-image: linear-gradient(to right, transparent, #000 calc(240 * var(--u)));
	}

	.photo {
		position: absolute;
		inset: 0;
	}

	.photo img {
		display: block;
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	/* Until the real photos arrive. */
	.stand-in {
		height: 100%;
		background: linear-gradient(135deg, hsl(var(--hue) 55% 55%), hsl(calc(var(--hue) + 40) 55% 25%));
	}

	/* The current photo's place, on a small board in the panel's bottom right
	   corner: 48 in from the edge and 32 above the film strip. Not inside the
	   panel, so it's at full strength. */
	.destination {
		position: absolute;
		right: calc(48 * var(--u));
		top: calc(var(--screen) - 179.75 * var(--u));
		translate: 0 -100%;
		display: flex;
	}

	.button {
		border: calc(3 * var(--u)) solid #fff;
		border-radius: calc(12 * var(--u));
		background: rgb(69 69 69 / 0.2);
		color: inherit;
		text-decoration: none;
	}

	/* The label is placed from the button's outer edge, like in Figma. */
	.button > .centered {
		margin: calc(-3 * var(--u)) 0 0 calc(-3 * var(--u));
		white-space: nowrap;
		width: auto;
	}

	.button:hover {
		background: rgb(69 69 69 / 0.4);
	}

	.button:focus-visible {
		outline: calc(2 * var(--u)) solid #fff;
		outline-offset: calc(4 * var(--u));
	}

	a {
		color: inherit;
	}
</style>
