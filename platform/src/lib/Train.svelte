<script>
	// One train car riding a track that runs the full width of the window,
	// as on the Figma home frame: a coupler at its front end, and a coupler
	// to the next car, which peeks in from the right. The middle car holds
	// the content and lines up with the page's content column. Put it
	// directly in .main.
	let { children, label = undefined, labelledby = undefined, class: klass = '' } = $props();
</script>

<div class="train">
	<div class="track rails" aria-hidden="true"></div>
	<!-- Couplers come before the cars so the car bodies cover their ends. -->
	<div class="coupler stub" aria-hidden="true"><i class="knob"></i><i class="bar"></i><i class="block"></i></div>
	<div class="coupler joint" aria-hidden="true"><i class="block"></i><i class="bar"></i><i class="knob"></i><i class="bar"></i><i class="block"></i></div>
	<section class="car middle {klass}" aria-label={label} aria-labelledby={labelledby}>
		{@render children()}
	</section>
	<div class="car neighbour" aria-hidden="true"></div>
</div>

<style>
	.train {
		/* Gap between cars, and the side gutter the content column keeps. */
		--couple: clamp(16px, 2.8vw, 48px);
		position: relative;
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, var(--content)) minmax(0, 1fr);
		column-gap: var(--couple);
		/* Full bleed out of .main; .app-shell clips the sideways overflow. */
		margin-inline: calc(50% - 50vw);
		padding-block: 16px;
	}
	/* The ties stick out a little above and below the cars. */
	.rails { position: absolute; inset: 0; }
	.car { position: relative; background: var(--car-bg); border-radius: var(--radius); }
	.middle { grid-column: 2; grid-row: 1; min-width: 0; }
	.neighbour { grid-column: 3; grid-row: 1; justify-self: start; width: 1156px; max-width: 70vw; }

	/* Couplers, from the Figma "connector" groups: #262626 parts with a
	   #2b2a2a edge, centred on the car's height. They tuck 33px under the
	   car at each end. */
	.coupler {
		position: relative;
		z-index: 0; /* own layer, still under the cars that follow it */
		grid-row: 1;
		grid-column: 2;
		align-self: center;
		display: flex;
		align-items: center;
		height: 36px;
	}
	.coupler i { display: block; flex: none; background: #262626; border: 1px solid #2b2a2a; }
	.coupler .block { width: 39px; height: 36px; }
	.coupler .bar { height: 12px; }
	.coupler .knob { width: 18px; height: 18px; border-radius: 8px; }
	/* Front end: knob, bar, then the block under the car. */
	.stub { justify-self: start; margin-left: -33px; }
	.stub .bar { width: 20px; margin-inline: -10px -2px; }
	.stub .knob { position: relative; z-index: 1; }
	/* Between cars: block, bar, knob, bar, block; the bars stretch with the gap. */
	.joint { justify-self: end; width: calc(var(--couple) + 64px); margin-right: calc(-1 * (var(--couple) + 31px)); }
	.joint .bar { flex: 1; min-width: 4px; margin-inline: -3px; }
	.joint .knob { position: relative; z-index: 1; margin-inline: -11px; }

	@media (max-width: 820px) {
		.train { --couple: 16px; }
	}
</style>
