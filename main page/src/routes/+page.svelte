<script>
	import thibaultLamTran from '#lib/assets/thibault-lam-tran.jpg';
	import aliKazal from '#lib/assets/ali-kazal.jpg';
	import jessiPena from '#lib/assets/jessi-pena.jpg';
	import filmTopLeft from '#lib/assets/film-top-left.png';
	import filmTopRight from '#lib/assets/film-top-right.png';
	import filmBottomLeft from '#lib/assets/film-bottom-left.png';
	import filmBottomRight from '#lib/assets/film-bottom-right.png';
	import ticketShape from '#lib/assets/ticket-shape.svg';
	import ticketPerforation from '#lib/assets/ticket-perforation.svg';
	import ticketPhoto from '#lib/assets/ticket-photo.jpg';
	import bookdragonPattern from '#lib/assets/bookdragon-pattern.png';
	import ticketCode from '#lib/assets/ticket-code.png';
	import arrowUp from '#lib/assets/arrow-up.svg';
	import arrowDown from '#lib/assets/arrow-down.svg';
	import fadeLine from '#lib/assets/fade-line.svg';

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

		<!-- Film strips are baked from Figma's shader; the left ones include the effect's bleed past the strip. -->
		<img class="abs" src={filmTopRight} alt="" style="--x: 640; --y: 1113; --w: 1088; --h: 295.5" />
		<img class="abs" src={filmBottomRight} alt="" style="--x: 637; --y: 2328; --w: 1091; --h: 295.5" />
		<img class="abs" src={filmTopLeft} alt="" style="--x: 0; --y: 1111; --w: 1094; --h: 295.5" />
		<img class="abs" src={filmBottomLeft} alt="" style="--x: 0; --y: 2326; --w: 1091; --h: 295.5" />

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

		<h1 class="title centered" style="--x: 851; --y: 93">Crossroads</h1>
		<p class="title reflection centered" style="--x: 851; --y: 157" aria-hidden="true">Crossroads</p>

		<p class="doto centered" style="--x: 852; --y: 550; --w: 842; --size: 40">
			Travel across Europe with 30 other teenage builders
		</p>

		<img class="line abs" src={arrowUp} alt="" style="--x: 841.113; --y: 664.17; --w: 29.887; --h: 5.774; --angle: 90deg" />
		<a class="button abs" href="#" style="--x: 742; --y: 688; --w: 228; --h: 67">
			<span class="doto centered" style="--x: 114; --y: 15; --size: 32">Get Started</span>
		</a>
		<img class="line abs" src={fadeLine} alt="" style="--x: 685.25; --y: 933.743; --w: 343.5; --h: 1; --angle: 90deg" />

		<h2 class="doto centered" style="--x: 875.5; --y: 1450; --w: 803; --size: 32">What is Hack Club Crossroads?</h2>
		<p class="doto centered" style="--x: 877.5; --y: 1510; --w: 803; --size: 24">
			Crossroads is a <a href="https://hackclub.com" target="_blank" rel="noopener">Hack Club</a> Event running January
			8–12 where a group of 30 teens build projects for 60 hours, then travel by train across Europe together.
			Everything (including flights, food and accommodation) is totally free for teenagers who qualify by building a
			project. Can't come? We're also providing funding and prizes for every teen who ships a technical project.
		</p>
		<h2 class="doto centered" style="--x: 888.5; --y: 1765; --w: 803; --size: 32">What is Hack Club Crossroads?</h2>

		<img class="line abs" src={arrowDown} alt="" style="--x: 772.056; --y: 2705.057; --w: 169.887; --h: 5.774; --angle: -90deg" />
		<a class="button abs" href="#" style="--x: 742; --y: 2813; --w: 228; --h: 67">
			<span class="doto centered" style="--x: 114; --y: 15; --size: 32">Get Started</span>
		</a>

		<p class="doto centered nowrap" style="--x: 856.5; --y: 3010; --size: 16">Fuffilment Bounty + Privacy and Terms</p>
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
		position: relative;
		width: calc(1728 * var(--u));
		height: calc(3060 * var(--u));
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

	.doto,
	.title {
		font-family: Doto, monospace;
		font-weight: 400;
		font-variation-settings: 'ROND' 0;
		font-size: calc(var(--size) * var(--u));
		line-height: normal;
		overflow-wrap: break-word;
	}

	.title {
		--size: 96;
		width: auto;
		white-space: nowrap;
	}

	.reflection {
		transform: scaleY(-1);
		color: #242e4d;
	}

	.line {
		rotate: var(--angle);
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
