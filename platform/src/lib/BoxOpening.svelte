<script>
	// Opening a box, from the Figma's "closed box", "partial open box" and
	// "open box" (nodes 293:1342, 293:1336, 292:1330). The closed box rocks
	// while the server picks a stamp; then the latch falls away, the lid
	// swings up through the partly open shape to open, and the stamp rises
	// out of the box. Measurements are the Figma's, in px.
	import wood from './assets/rail/wood.webp';
	import leak from './assets/rail/leak.webp';
	import plateTopRight from './assets/box/plate-top-right.png';
	import plateTopLeft from './assets/box/plate-top-left.png';
	import plateBottomRight from './assets/box/plate-bottom-right.png';
	import plateBottomLeft from './assets/box/plate-bottom-left.png';
	import rivetTop from './assets/box/rivet-top.png';
	import rivetBottom from './assets/box/rivet-bottom.png';

	// stamp: the special stamp the box dropped, or null while it's opening.
	let { stamp = null, label = 'A box' } = $props();

	// Open only once the stamp's picture has loaded, so it doesn't pop in
	// halfway out of the box.
	let open = $state(false);
	$effect(() => {
		open = false;
		const url = stamp?.type.image_url;
		if (!stamp) return;
		let cancelled = false;
		const go = () => !cancelled && (open = true);
		if (!url) go();
		else {
			const img = new Image();
			img.src = url;
			img.decode().then(go, go);
		}
		return () => (cancelled = true);
	});
</script>

<div class="stage" class:open style="--wood: url({wood}); --leak: url({leak})" role="img" aria-label={open ? `${label}, open` : `${label}, opening`}>
	<div class="chest">
		<span class="foot left"></span>
		<span class="foot right"></span>
		<span class="lid"></span>
		<span class="inside"><span class="inside-panel"></span></span>
		<span class="glow"></span>
		<div class="mouth">
			{#if stamp}
				<div class="stamp">
					{#if stamp.type.image_url}<img src={stamp.type.image_url} alt="" />{:else}<span class="blank">{stamp.type.name}</span>{/if}
				</div>
			{/if}
		</div>
		<span class="base"></span>
		<span class="latch" aria-hidden="true">
			<span class="plate up right"><img src={plateTopRight} alt="" /></span>
			<span class="plate up left"><img src={plateTopLeft} alt="" /></span>
			<img class="rivet up" src={rivetTop} alt="" />
			<span class="lock hasp"></span>
			<span class="lock shackle"></span>
			<span class="lock body"></span>
			<span class="plate down right"><img src={plateBottomRight} alt="" /></span>
			<span class="plate down left"><img src={plateBottomLeft} alt="" /></span>
			<img class="rivet down" src={rivetBottom} alt="" />
		</span>
	</div>
</div>

<style>
	/* Room above the box for the stamp to rise into. */
	.stage {
		position: relative;
		display: grid;
		place-items: end center;
		height: 380px;
		padding-bottom: 8px;
	}
	.chest {
		/* The Figma's wood is see-through; it was drawn on this. */
		--under: #1f1f1f;
		position: relative;
		width: 275px;
		height: 216px; /* the open box, 204, and its feet */
		transform-origin: 50% 100%;
		animation: rock 700ms ease-in-out infinite;
	}
	.open .chest { animation: none; }
	/* A shadow on the floor. */
	.chest::before {
		content: '';
		position: absolute;
		left: -12px;
		right: -12px;
		bottom: -8px;
		height: 18px;
		border-radius: 50%;
		background: radial-gradient(closest-side, rgb(0 0 0 / 0.55), transparent);
	}
	.chest > * { position: absolute; display: block; }

	/* The Figma's two woods: the lid is brown with 20% of the grain on top;
	   the base and feet are the grain under 80% brown. */
	.lid {
		left: 0;
		width: 275px;
		bottom: 107px;
		height: 38px;
		border-radius: 4px 4px 0 0;
		background: linear-gradient(rgb(69 52 27 / 0.8), rgb(69 52 27 / 0.8)), var(--under);
		box-shadow: 0 2px 4px rgb(0 0 0 / 0.25);
		overflow: hidden;
	}
	.lid::after {
		content: '';
		position: absolute;
		inset: 0;
		background: var(--wood) center / cover;
		opacity: 0.2;
	}
	.base,
	.foot {
		background: linear-gradient(rgb(69 52 27 / 0.8), rgb(69 52 27 / 0.8)), var(--wood) center / cover, var(--under);
		border-radius: 0 0 4px 4px;
		box-shadow: 0 4px 4px rgb(0 0 0 / 0.25);
	}
	.base { left: 0; bottom: 4px; width: 275px; height: 103px; }
	.foot { bottom: 0; width: 20px; height: 12px; }
	.foot.left { left: 8px; }
	.foot.right { left: 247px; }

	/* The underside of the lid, seen as it swings up: none of it when shut. */
	.inside {
		left: 0;
		width: 275px;
		bottom: 107px;
		height: 0;
		border-radius: 4px;
		background: linear-gradient(rgb(49 37 20 / 0.8), rgb(49 37 20 / 0.8)), var(--wood) center / cover, var(--under);
		box-shadow: 0 2px 4px rgb(0 0 0 / 0.25);
	}
	.inside-panel {
		position: absolute;
		left: 7px;
		width: 262px;
		top: 10.5%;
		height: 81.6%;
		border-radius: 4px;
		background: linear-gradient(rgb(37 27 13 / 0.8), rgb(37 27 13 / 0.8)), var(--wood) center / cover;
	}

	/* Warm light from inside once it's open, behind the stamp. */
	.glow {
		left: 50%;
		bottom: 90px;
		width: 260px;
		height: 200px;
		translate: -50% 0;
		border-radius: 50%;
		background: radial-gradient(closest-side, rgb(255 214 140 / 0.55), rgb(255 180 90 / 0.15) 60%, transparent);
		opacity: 0;
		pointer-events: none;
	}

	/* The stamp starts inside the box and comes out of its top; below the
	   rim it's clipped, as if behind the front. */
	.mouth {
		left: 0;
		right: 0;
		bottom: 107px;
		height: 300px;
		clip-path: inset(-100px -100px 0 -100px);
	}
	.stamp {
		position: absolute;
		left: 50%;
		bottom: 0;
		translate: -50% 105%;
	}
	.stamp img {
		display: block;
		height: 140px;
		width: auto;
		max-width: 180px;
		filter: drop-shadow(0 4px 8px rgb(0 0 0 / 0.55));
	}
	.blank {
		display: grid;
		place-items: center;
		width: 110px;
		height: 140px;
		padding: 8px;
		text-align: center;
		color: var(--ink);
		background: #efe6d2;
		border: 3px dashed rgb(0 0 0 / 0.25);
		outline: 6px solid #efe6d2;
		box-shadow: 0 4px 10px rgb(0 0 0 / 0.5);
	}

	/* The latch and padlock on the closed box, whose top is 71px down the
	   216px chest. */
	.latch { inset: 0; pointer-events: none; }
	.latch > * { position: absolute; display: block; }
	.plate { width: 26.077px; }
	.plate img { display: block; width: 100%; }
	.plate.up { top: 85.141px; height: 22.859px; }
	.plate.up img { height: 22.148px; margin-top: 0.711px; }
	.plate.up.right { left: 138px; scale: 1 -1; }
	.plate.up.left { left: 111.923px; rotate: 180deg; }
	.plate.down { top: 110px; height: 26.835px; }
	.plate.down img { height: 26px; }
	.plate.down.right { left: 138px; }
	.plate.down.left { left: 111.923px; scale: -1 1; }
	.rivet.up { left: 137px; top: 85px; width: 2px; height: 4.259px; }
	.rivet.down { left: 137px; top: 132px; width: 2px; height: 5px; }
	.lock {
		border-radius: 2px;
		background: #4a4040;
		overflow: hidden;
	}
	.lock::after {
		content: '';
		position: absolute;
		inset: 0;
		background: var(--leak) center / cover;
		mix-blend-mode: multiply;
		opacity: 0.2;
	}
	.lock.hasp { left: 131px; top: 100px; width: 14px; height: 7px; box-shadow: 0 2px 4px rgb(0 0 0 / 0.25); }
	.lock.shackle { left: 135px; top: 104px; width: 6px; height: 9px; }
	.lock.body { left: 131px; top: 111px; width: 14px; height: 13px; box-shadow: 0 2px 4px rgb(0 0 0 / 0.25); }

	/* Opening: the padlock drops, the latch goes with the lid, the lid swings
	   up, then the stamp rises. */
	.open .lock.body { animation: drop 320ms ease-in forwards; }
	.open .latch { animation: fade 260ms ease-out 220ms forwards; }
	.open .lid { animation: lid 900ms cubic-bezier(0.3, 0, 0.2, 1) 300ms forwards; }
	.open .inside { animation: inside 900ms cubic-bezier(0.3, 0, 0.2, 1) 300ms forwards; }
	.open .glow { animation: glow 900ms ease-out 900ms forwards; }
	.open .stamp { animation: rise 1100ms cubic-bezier(0.2, 0.9, 0.3, 1.15) 1000ms forwards; }

	@keyframes rock {
		0%, 100% { rotate: 0deg; }
		25% { rotate: -1.6deg; }
		75% { rotate: 1.6deg; }
	}
	@keyframes drop {
		to { translate: 0 10px; opacity: 0; }
	}
	@keyframes fade {
		to { opacity: 0; }
	}
	/* closed → partly open → open, as in the Figma: the lid's top edge rides
	   up on its underside, which grows from nothing to 38px to 76px. */
	@keyframes lid {
		0% { bottom: 107px; height: 38px; }
		45% { bottom: 135px; height: 40px; }
		100% { bottom: 175px; height: 33px; }
	}
	@keyframes inside {
		0% { height: 0; }
		45% { height: 38px; }
		100% { height: 76px; }
	}
	@keyframes glow {
		to { opacity: 1; }
	}
	@keyframes rise {
		0% { translate: -50% 105%; rotate: 0deg; }
		100% { translate: -50% -72px; rotate: -4deg; }
	}

	@media (prefers-reduced-motion: reduce) {
		.chest { animation: none; }
		.open .lock.body,
		.open .latch,
		.open .lid,
		.open .inside,
		.open .glow,
		.open .stamp { animation-duration: 1ms; animation-delay: 0ms; }
	}
</style>
