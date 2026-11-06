<script>
	// The public landing page, from the 1728x3060 Figma frame ("MacBook Pro 16\" - 6" in
	// crossroads-draft). Positions are design pixels; --u scales them to the viewport, and
	// below about 780px wide the page stops shrinking and crops the sides instead.
	// The film strips are film.svg, redrawn from the Figma strip without its colour bleed.
	// "Get Started" goes to /signin.
	import arrowUp from '#lib/assets/landing/arrow-up.svg';
	import arrowDown from '#lib/assets/landing/arrow-down.svg';
	import fadeLine from '#lib/assets/landing/fade-line.svg';
	import paris from '#lib/assets/landing/route/paris.webp';
	import berlin from '#lib/assets/landing/route/berlin.webp';
	import genoa from '#lib/assets/landing/route/genoa.webp';
	import sanSalvatore from '#lib/assets/landing/route/san-salvatore.webp';
	import brussels from '#lib/assets/landing/route/brussels.webp';
	import outernetCoding from '#lib/assets/landing/events/outernet-coding.webp';
	import outernetFlamethrower from '#lib/assets/landing/events/outernet-flamethrower.webp';
	import outernetGroup from '#lib/assets/landing/events/outernet-group.webp';
	import flagshipHands from '#lib/assets/landing/events/flagship-hands.webp';
	import assembleWorkshop from '#lib/assets/landing/events/assemble-workshop.webp';
	import assembleGroup from '#lib/assets/landing/events/assemble-group.webp';
	import SplitFlapDisplay from '#lib/SplitFlapDisplay.svelte';
	import Postcard from '#lib/Postcard.svelte';
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

	// How far the postcard can rotate on each axis, in degrees.
	const MAX_TILT = 7;

	// How far outside the postcard the cursor starts to tilt it, as a multiple of the postcard's height.
	const REACH = 0.6;

	// How quickly the tilt catches up with the cursor, in seconds. Higher is smoother but laggier.
	const SMOOTHING = 0.15;

	// The pointer sets `target`; each frame `tilt` eases toward it so fast sweeps don't jolt the postcard.
	let tilt = $state({ x: 0, y: 0 });
	let target = { x: 0, y: 0 };
	let frame = 0;
	let lastTime = 0;
	let postcardArea;

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
	// postcard the tilt aims at the nearest edge and fades out with distance, so it
	// builds up as the cursor approaches instead of snapping on at the edge.
	function tiltPostcard(event) {
		if (event.pointerType !== 'mouse') return;
		const box = postcardArea.getBoundingClientRect();
		const px = (event.clientX - box.left) / box.width - 0.5;
		const py = (event.clientY - box.top) / box.height - 0.5;
		const dx = Math.max(box.left - event.clientX, 0, event.clientX - box.right);
		const dy = Math.max(box.top - event.clientY, 0, event.clientY - box.bottom);
		const t = Math.max(0, 1 - Math.hypot(dx, dy) / (box.height * REACH));
		const strength = t * t * (3 - 2 * t);
		if (strength === 0) return resetPostcard();
		const clamp = (v) => Math.min(0.5, Math.max(-0.5, v));
		setTarget(-clamp(py) * 2 * MAX_TILT * strength, clamp(px) * 2 * MAX_TILT * strength);
	}

	function resetPostcard() {
		if (target.x || target.y) setTarget(0, 0);
	}

	// relatedTarget is null when the pointer leaves the window entirely.
	function leaveWindow(event) {
		if (!event.relatedTarget) resetPostcard();
	}

	// Photos of places on the route fade from one to the next on the right of
	// the first screen, with the place's name on a small board in the corner
	// and the photo's credit under it. They're from Wikimedia Commons, resized
	// to 1800px and converted to WebP, and their licences need that credit
	// shown. `position` is an object-position, where a photo's subject would
	// otherwise be cropped or faded out.
	const BY_SA = 'https://creativecommons.org/licenses/by-sa/4.0/';
	const route = [
		{
			place: 'Paris',
			src: paris,
			author: 'Fabien Barrau',
			license: 'CC BY-SA 4.0',
			licenseUrl: BY_SA,
			page: 'https://commons.wikimedia.org/wiki/File:Tour_Saint-Jacques_au_cr%C3%A9puscule.jpg'
		},
		{
			place: 'Berlin',
			src: berlin,
			author: 'Diego Delso',
			license: 'CC BY-SA 4.0',
			licenseUrl: BY_SA,
			page: 'https://commons.wikimedia.org/wiki/File:Fernsehturm,_Berl%C3%ADn,_Alemania,_2016-04-22,_DD_40-42_HDR.jpg'
		},
		{
			place: 'Genoa',
			src: genoa,
			author: 'Abldecn',
			license: 'CC BY 4.0',
			licenseUrl: 'https://creativecommons.org/licenses/by/4.0/',
			page: 'https://commons.wikimedia.org/wiki/File:Pier_of_Boccadasse.jpg'
		},
		{
			place: 'San Salvatore',
			src: sanSalvatore,
			position: 'left center',
			author: 'JoachimKohler-HB',
			license: 'CC BY-SA 4.0',
			licenseUrl: BY_SA,
			page: 'https://commons.wikimedia.org/wiki/File:Lugano_-_Luganersee_%26_Monte_San_Salvatore_(2013).jpg'
		},
		{
			place: 'Brussels',
			src: brussels,
			author: 'Diego Delso',
			license: 'CC BY-SA 4.0',
			licenseUrl: BY_SA,
			page: 'https://commons.wikimedia.org/wiki/File:Edificios_en_la_Grand-Place,_Bruselas,_B%C3%A9lgica,_2021-12-15,_DD_184-186_HDR.jpg'
		}
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

	// Load the next photo while this one shows, so it's ready to fade in.
	$effect(() => {
		new Image().src = route[(photo + 1) % route.length].src;
	});

	// Photos of Hack Club events in the film strips' frames, from the press
	// page (hackclub.com/press), cropped to the frames' shape. Each strip shows
	// five frames, the first and last cut off by the screen's edges, so every
	// photo gets a whole frame once and the cut-off ones repeat.
	const shots = {
		outernetCoding: { src: outernetCoding, alt: 'Teens coding at Outernet' },
		outernetFlamethrower: { src: outernetFlamethrower, alt: 'Teens demonstrating a flame thrower at Outernet' },
		outernetGroup: { src: outernetGroup, alt: 'Group photo at Outernet, our Summer 2023 hackathon' },
		flagshipHands: { src: flagshipHands, alt: 'Teens with hands raised at Flagship' },
		assembleWorkshop: { src: assembleWorkshop, alt: 'Teenager leading a workshop at Assemble' },
		assembleGroup: { src: assembleGroup, alt: 'Group photo at Assemble, our Summer 2022 hackathon' }
	};
	const reels = [
		[shots.assembleGroup, shots.outernetCoding, shots.flagshipHands, shots.assembleWorkshop, shots.outernetFlamethrower],
		[shots.assembleWorkshop, shots.outernetFlamethrower, shots.outernetGroup, shots.assembleGroup, shots.outernetCoding]
	];

	// Straight tracks down the left of the first screen, under the logo's
	// track, from the top of the window to the film strip: the x of each
	// one's middle in design pixels, evenly spaced across the left half.
	// Cars carrying event details will run on them.
	const tracks = [108, 324, 540, 756];

	// People riding cars on the three left tracks (the fourth runs behind the
	// postcard). Their pictures come from Cachet, Hack Club's cache of Slack
	// avatars, by Slack ID; a rider without one yet gets an empty window.
	const riders = [
		{ name: 'Joaquin', slack: 'U083W5CCFDG' },
		{ name: 'Max H', slack: 'U078ZJHUKFW' },
		{ name: 'Barnav', slack: 'U08RVF1BAN4' },
		{ name: 'Selena', slack: 'U06U1CBK589' }
	];
	const avatar = (slack) => `https://cachet.dunkirk.sh/users/${slack}/r`;

	// A train on each of those tracks, carrying every third rider: `time` is
	// how many seconds it takes to cross the screen, `start` how far across it
	// is when the page loads, and `up` which way it runs.
	const trains = [
		{ time: 40, start: 0.2, up: false },
		{ time: 52, start: 0.65, up: true },
		{ time: 46, start: 0.4, up: false }
	].map((train, t) => ({ ...train, x: tracks[t], cars: riders.filter((_, i) => i % 3 === t) }));
</script>

<svelte:head>
	<title>Hack Club Crossroads</title>
	<meta name="description" content="Hack across Europe. 6 cities. 5 days. 30 teenagers." />
</svelte:head>

<svelte:window onpointermove={tiltPostcard} onpointerout={leaveWindow} />

<main class="page">
	<!-- Positions are design pixels from the 1728x3060 Figma frame; --u scales them. -->
	<div class="frame">
		<!-- Above the film strip, the first screen splits: tracks on the left, the route's photos on the right. -->
		<div class="yard" aria-hidden="true">
			{#each tracks as x}<div class="yard-track" style="--x: {x}"></div>{/each}
			{#each trains as train}
				<div class="yard-train" class:up={train.up} style="--x: {train.x}; --time: {train.time}s; --start: {-train.start * train.time}s">
					{#each train.cars as rider}
						<div class="car">
							<div class="window">{#if rider.slack}<img src={avatar(rider.slack)} alt="" />{/if}</div>
						</div>
					{/each}
				</div>
			{/each}
		</div>
		<div class="route" aria-hidden="true">
			{#key photo}
				<div class="photo" transition:fade={{ duration: FADE_TIME }}>
					<img src={route[photo].src} alt="" style:object-position={route[photo].position} />
				</div>
			{/key}
		</div>
		<div class="destination" aria-hidden="true">
			<SplitFlapDisplay text={route[photo].place} columns={13} size="sm" centered lowercase />
		</div>
		<p class="doto credit" style="--size: 11">
			Photo: <a href={route[photo].page} target="_blank" rel="noopener">{route[photo].author}</a>,
			<a href={route[photo].licenseUrl} target="_blank" rel="noopener">{route[photo].license}</a>
		</p>

		<!-- Film strips. .below moves them, and everything after them, with the height of the first screen. -->
		{#each reels as reel, r}
			<div class="film abs below" style="--x: 0; --y: {[1111, 2326][r]}; --w: 1728; --h: 295.5">
				{#each reel as shot, i}
					<!-- The cut-off frames at the ends repeat photos, so only the whole ones are described. -->
					<img class="shot" src={shot.src} alt={i === 0 || i === 4 ? '' : shot.alt} style="--x: {691 + 404 * (i - 2)}" />
				{/each}
			</div>
		{/each}

		<!-- The postcard, 390 wide (900 Figma pixels, so --pc is 390/900 of a design
		     pixel), centred between the board and the tagline. It tilts toward the
		     cursor; the tilt is measured against this untransformed box, so the
		     tilting card doesn't feed back into it. -->
		<div class="abs" style="--x: 657; --y: 263; --w: 390; --h: 260" bind:this={postcardArea}>
			<div class="tilt" style="--tilt-x: {tilt.x}deg; --tilt-y: {tilt.y}deg; --pc: calc(var(--u) * 390 / 900)">
				<Postcard />
			</div>
		</div>

		<!-- The title flips into place on a split-flap board when the page loads. The
		     board is decoration; the heading is there for screen readers. It sits
		     centred where the Figma title and its reflection were. -->
		<h1 class="visually-hidden">Crossroads</h1>
		<div class="board-title centered" style="--x: 851; --y: 130" aria-hidden="true">
			<SplitFlapDisplay text="Crossroads" columns={14} size="lg" centered />
		</div>

		<p class="crimson centered" style="--x: 852; --y: 550; --w: 842; --size: 40">
			Hack across Europe.
		</p>

		<p class="doto centered" style="--x: 852; --y: 610; --w: 842; --size: 30">
			5 days. 6 cities. 30 teenagers.
		</p>

		<img class="line abs" src={arrowUp} alt="" style="--x: 841.113; --y: 664.17; --w: 29.887; --h: 5.774; --angle: 90deg" />
		<a class="button abs" href="/signin" onclick={getStarted} style="--x: 742; --y: 688; --w: 228; --h: 67">
			<span class="doto centered" style="--x: 114; --y: 15; --size: 32">Get Started</span>
		</a>
		<img class="drop-line" src={fadeLine} alt="" />

		<h2 class="doto centered below" style="--x: 875.5; --y: 1450; --w: 803; --size: 32">What is Hack Club Crossroads?</h2>
		<p class="doto centered below" style="--x: 877.5; --y: 1510; --w: 803; --size: 24">
			Crossroads is a <a href="https://hackclub.com" target="_blank" rel="noopener">Hack Club</a> event running in February 2027
			where a group of 30 teens build projects for 65 hours, then travel by train across Europe together.
			Everything (including train passes, food and accommodation) is totally free for teenagers who qualify by building a
			project. Can't come? We're also providing funding and prizes for every teen who ships a technical project.
		</p>
		<h2 class="doto centered below" style="--x: 888.5; --y: 1765; --w: 803; --size: 32">What is Hack Club Crossroads?</h2>

		<img class="line abs below" src={arrowDown} alt="" style="--x: 772.056; --y: 2705.057; --w: 169.887; --h: 5.774; --angle: -90deg" />
		<a class="button abs below" href="/signin" onclick={getStarted} style="--x: 742; --y: 2813; --w: 228; --h: 67">
			<span class="doto centered" style="--x: 114; --y: 15; --size: 32">Get Started</span>
		</a>

		<p class="doto centered nowrap below" style="--x: 856.5; --y: 3010; --size: 16"><a href="https://forms.hackclub.com/bounty" target="_blank">Fulfillment Bounty</a> + <a href="https://hackclub.com/privacy-and-terms" target="_blank">Privacy and Terms</a></p>
	</div>
</main>

<style>
	:global(*) {
		box-sizing: border-box;
	}

	:global(body) {
		margin: 0;
		background: #000;
	}

	/* The platform's global img { max-width: 100% } would squash images that
	   reach past the frame; positions here are exact. */
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
		/* Where the top film strip starts, and the first screen's halves end. */
		--strip-top: calc(var(--screen) - 147.75 * var(--u));
		position: relative;
		width: calc(1728 * var(--u));
		height: calc(3060 * var(--u) + var(--shift));
		margin-left: calc(50cqw - 864 * var(--u));
		overflow: hidden;
		color: #fff;
		/* Textured black: faint film grain over near-black. */
		background: url('../lib/assets/landing/grain.svg') 0 0 / 256px #070707;
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

	.tilt {
		transform: perspective(calc(1000 * var(--u))) rotateX(var(--tilt-x)) rotateY(var(--tilt-y));
		will-change: transform;
	}

	@media (prefers-reduced-motion: reduce) {
		.tilt {
			transform: none;
		}
	}

	.doto {
		font-family: Doto, monospace;
		font-weight: 400;
		font-variation-settings: 'ROND' 0;
		font-size: calc(var(--size) * var(--u));
		line-height: normal;
		overflow-wrap: break-word;
	}

	.crimson {
		font-family: "Crimson Text", serif;
		font-weight: 400;
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

	/* One frame of film, repeated across the page and centred on it. */
	.film {
		background: url('../lib/assets/landing/film.svg') center top / calc(404 * var(--u)) 100% repeat-x;
	}

	/* A photo in one of film.svg's frames: 346 by 202 with round corners, 43
	   down from the top of the strip, one every 404 with the middle one
	   centred on the page. */
	.shot {
		position: absolute;
		left: calc(var(--x) * var(--u));
		top: calc(43 * var(--u));
		width: calc(346 * var(--u));
		height: calc(202 * var(--u));
		border-radius: calc(20 * var(--u));
		object-fit: cover;
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
		height: var(--strip-top);
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

	/* Trains loop from just above the top of the screen to just below the
	   film strip, starting part of the way along so there are cars on screen
	   from the start. With reduced motion they stay where they start. */
	.yard-train {
		position: absolute;
		top: 0;
		left: calc(var(--x) * var(--u));
		display: flex;
		flex-direction: column;
		gap: calc(12 * var(--u));
		animation: run var(--time) linear var(--start) infinite;
	}

	.yard-train.up {
		animation-direction: reverse;
	}

	@media (prefers-reduced-motion: reduce) {
		.yard-train {
			animation-play-state: paused;
		}
	}

	@keyframes run {
		from {
			translate: -50% -100%;
		}
		to {
			translate: -50% var(--strip-top);
		}
	}

	/* A car seen from above: black with a faint edge, like the boards, and a
	   window with its rider's Slack picture. Couplers join it to the car in
	   front. */
	.car {
		position: relative;
		display: grid;
		place-items: center;
		width: calc(80 * var(--u));
		height: calc(112 * var(--u));
		background: #0a0a0a;
		border: var(--u) solid rgba(255, 255, 255, 0.06);
		box-shadow: 0 calc(8 * var(--u)) calc(20 * var(--u)) rgba(0, 0, 0, 0.5);
	}

	.car + .car::before {
		content: '';
		position: absolute;
		bottom: 100%;
		left: 50%;
		translate: -50% 0;
		width: calc(10 * var(--u));
		height: calc(12 * var(--u));
		background: #262626;
	}

	/* Empty, it's the colour of the boards' tiles. */
	.window {
		width: calc(56 * var(--u));
		height: calc(56 * var(--u));
		background: #1b1b1b;
		overflow: hidden;
	}

	.window img {
		display: block;
		width: 100%;
		height: 100%;
		object-fit: cover;
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

	/* The current photo's place, on a small board in the panel's bottom right
	   corner: 48 in from the edge and 24 above the film strip. Not inside the
	   panel, so it's at full strength. */
	.destination {
		position: absolute;
		right: calc(48 * var(--u));
		top: calc(var(--screen) - 171.75 * var(--u));
		translate: 0 -100%;
		display: flex;
		--split-flap-padding: 4;
	}

	/* The photo's credit, which its licence asks for: sitting on the top of
	   the film strip, 12 in from the right edge. */
	.credit {
		position: absolute;
		right: calc(12 * var(--u));
		top: calc(var(--screen) - 151.75 * var(--u));
		translate: 0 -100%;
		margin: 0;
		opacity: 0.7;
		white-space: nowrap;
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
