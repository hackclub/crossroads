<script>
	// The postcard from the crossroads-draft Figma file (the assets page's
	// "Postcard 1 Front", 195:3, and "Postcard 1 Back", 195:146): "we're waiting
	// in Berlin..." with a stamp on the front, "are you coming? -guac" on the
	// back. Clicking it turns it over. It's 900 by 600 Figma pixels; set --pc,
	// one of those pixels, on a parent to size it.
	//
	// Every pen stroke is its own SVG from Figma, placed where Figma put it. Both
	// sides share Figma's paper texture, resized to 1800x1350 WebP.
	const files = import.meta.glob('./assets/landing/postcard/**/*', { eager: true, import: 'default' });
	const src = (name) => files[`./assets/landing/postcard/${name}`];

	// Each stroke is [file, left, top, width, height, inset]: its layer in Figma
	// pixels, and how far its SVG reaches past that layer (the stroke's
	// thickness), as top right bottom left.
	const front = {
		label: "Postcard: we're waiting in Berlin...",
		dir: 'front',
		stamp: true,
		strokes: [
			['v7.svg', 164.39, 129.87, 49.407, 46.994, '0 -2.64% -3.15% -2.88%'],
			['v8.svg', 231.93, 132.58, 38.036, 38.381, '-3.45% 0 -3.88% -3.78%'],
			['v9.svg', 266.7, 101.57, 14.149, 15.237, '-7.77% 0 0 -8.37%'],
			['v10.svg', 292.82, 131.05, 25.033, 41.394, '-2.48% 0 -3.34% -5.39%'],
			['v11.svg', 324.18, 129.51, 33.939, 41.561, '-3.11% 0 -3.26% -4.06%'],
			['v12.svg', 340.71, 240.88, 6.53, 4.354, '0'],
			['v13.svg', 332, 233.26, 38.093, 27.608, '-5.61% -1.72% -5.68% -4.06%'],
			['v14.svg', 381.26, 232.41, 32.367, 29.52, '-4.37% 0 -4.02% -4.04%'],
			['v15.svg', 382.94, 245.47, 23.068, 15.983, '-8.24% 0 -9.16% -6.2%'],
			['v16.svg', 415.81, 234.35, 10.16, 27.714, '-5.13% -6.73% -5.03% -13.99%'],
			['v17.svg', 411.5, 210.91, 11.802, 13.065, '-10.01% -10.61% -9.54% -10.48%'],
			['v18.svg', 437.58, 199.59, 19.591, 63.585, '-1.78% -1.77% -2.19% -5.52%'],
			['v19.svg', 434.31, 232.74, 27.21, 5.967, '-22.24% -4.86% -21.16% -4.64%'],
			['v20.svg', 468.05, 236.99, 15.237, 28.376, '-4.19% 0 -4.48% -7.84%'],
			['v21.svg', 471.5, 217.3, 11.058, 10.519, '-11.7% -10.91% -6.78% -9.07%'],
			['v22.svg', 491.99, 234.49, 11.972, 25.734, '-5.36% -0.48% -2.42% -11.28%'],
			['v23.svg', 502.88, 233.63, 29.386, 27.841, '-5.06% 0 -5% -4.37%'],
			['v24.svg', 535.89, 234.89, 28.349, 94.416, '-1.54% -5.53% -1.64% -5.52%'],
			['v25.svg', 613.89, 231.83, 14.662, 28.527, '-4.43% 0 -4.77% -8.22%'],
			['v26.svg', 611.72, 201.7, 17.378, 19.591, '-7.47% -8.43% 0 0'],
			['v27.svg', 635.3, 231.41, 12.272, 27.964, '-4.77% -11.22% -2.83% -10.53%'],
			['v28.svg', 648.72, 235.93, 37.005, 25.507, '-5.72% 0 -6.31% -3.55%'],
			['v29.svg', 178.54, 342.75, 66.987, 166.792, '-0.82% -2.31% -0.97% -1.78%'],
			['v30.svg', 262.35, 427.35, 50.066, 69.299, '-2.06% 0 -2.51% -2.45%'],
			['v31.svg', 323.3, 429.76, 14.817, 70.668, '-1.97% -9.44% -0.96% -8.55%'],
			['v32.svg', 339.62, 431.35, 40.208, 16.416, '0 -1.88% -7.13% -2.92%'],
			['v33.svg', 395.13, 342.59, 26.121, 160.188, '-0.87% -1.04% -1.02% -5%'],
			['v34.svg', 443.02, 445.52, 16.326, 52.108, '-2.72% -2.9% -3.03% -8.38%'],
			['v35.svg', 441.93, 410.67, 18.243, 15.825, '-8.49% -7.99% -8.82% 0'],
			['v36.svg', 472.4, 446.57, 13.906, 54.934, '-2.43% -11% -2.33% -9.03%'],
			['v37.svg', 487.64, 450.18, 41.359, 52.824, '-2.69% 0 -3.19% -3.44%'],
			['v38.svg', 534.49, 493.5, 11.818, 11.736, '-10.93% -10.55% -8.67% -11.19%'],
			['v39.svg', 552.95, 488.57, 11.731, 15.66, '0 -8.82% -8.51% -11.59%'],
			['v40.svg', 570.61, 491.21, 14.957, 15.22, '0 -7.47% -7.92% -7.85%'],
			['v43.svg', 610.54, 326.11, 86.867, 51.909, '0 0 -3.01% -1.8%'],
			['v44.svg', 634.57, 379.11, 115.369, 23.944, '0 0 -7.47% -1.55%'],
			['v46.svg', 102.63, 195.26, 77, 51.498, '-1.98% 0 -1.81% -1.37%'],
			['v47.svg', 558.39, 80.32, 181.401, 128.996, '-0.9% -0.73% 0 -0.63%'],
			['v73.svg', 31.32, 342.94, 123.339, 23.417, '-3.64% -1.49% -7.86% -0.34%'],
			['v74.svg', 50.8, 436.59, 95.297, 25.122, '-6.15% -1.62% 0 0'],
			['v75.svg', 82.71, 508.54, 85.652, 83.939, '0'],
			['v76.svg', 67.26, 506.83, 101.105, 83.146, '-2.38% -1.96% -0.91% -0.75%']
		]
	};
	const back = {
		label: 'Back of the postcard: are you coming? -guac',
		dir: 'back',
		stamp: false,
		strokes: [
			['v54.svg', 84.62, 246.47, 64.928, 77.152, '-1.89% 0 -1.95% -2.23%'],
			['v55.svg', 95.09, 280.86, 40.643, 40.516, '-3.02% -1.33% -3.63% -3.64%'],
			['v56.svg', 145.62, 249.87, 26.208, 73.436, '-1.86% -5.8% 0 -5.21%'],
			['v57.svg', 168.7, 257.74, 46.158, 17.763, '-7.72% 0 -1.29% -2.41%'],
			['v58.svg', 225.12, 262.45, 34.245, 55.259, '-2.44% 0 -2.6% -4.4%'],
			['v59.svg', 358.46, 267.26, 41.721, 129.254, '-1.2% -3.76% -1.23% -3.53%'],
			['v60.svg', 406.08, 267.26, 35.752, 40.747, '-3.93% -4.95% -4.05% 0'],
			['v61.svg', 450.59, 265.61, 36.059, 40.788, '0 -4.44% -3.98% -4.23%'],
			['v63.svg', 540.13, 267.54, 27.513, 37.085, '-4.07% 0 -4.49% -5.81%'],
			['v64.svg', 574.23, 269.67, 32.609, 30.35, '-4.47% -4.87% -5.27% 0'],
			['v65.svg', 617.08, 270.54, 41.222, 30.464, '-5.48% 0 -2.59% -4.09%'],
			['v66.svg', 669.85, 272.2, 2.473, 29.665, '-5.36% -33.67% -2.77% -64.23%'],
			['v67.svg', 664.9, 247.48, 14.63, 12.428, '0 -8.49% -9.81% 0'],
			['v68.svg', 691.28, 270.55, 28.025, 31.677, '-5.33% 0 -5.32% -6.02%'],
			['v69.svg', 704.95, 274.47, 40.51, 101.303, '-1.43% -3.95% -1.89% -4.55%'],
			['v70.svg', 767.11, 191.74, 54.53, 120.031, '-1.34% -3.26% 0 -2.97%'],
			['v71.svg', 778.65, 337.25, 32.751, 25.122, '-4.68% -5.27% -6.32% 0'],
			['v72.svg', 369.5, 319.01, 109.579, 13.188, '0 -0.77% -13.98% -1.68%'],
			['v48.svg', 597, 494.16, 11.443, 3.302, '-39.06% -10.35% -26.88% -11.4%'],
			['v49.svg', 606.6, 482.32, 31.101, 61.454, '-2.19% -4.66% -2.77% -5.02%'],
			['v50.svg', 647.63, 481.26, 35.917, 21.144, '-6.28% 0 -7.02% -3.7%'],
			['v51.svg', 684.64, 481.74, 23.944, 24.573, '-5.56% -1.28% -4.34% -5.22%'],
			['v52.svg', 685, 491.06, 20.315, 13.463, '-9.7% 0 -10.81% -7%'],
			['v53.svg', 715.08, 480, 18.736, 22.139, '-6.08% -4.03% -6.54% -7.49%']
		]
	};

	let flipped = $state(false);
</script>

<!-- Spans rather than divs, since a button can only hold inline content. The
     button's label says what's on the side showing, so the faces are hidden
     from screen readers. -->
{#snippet side(face)}
	<span class="face {face.dir}" aria-hidden="true">
		<img class="paper" src={src('paper.webp')} alt="" />
		{#if face.stamp}
			<span class="layer stamp" style="--l: 770; --t: 27; --w: 102; --h: 129">
				<img class="paper" src={src('front/stamp.png')} alt="" />
			</span>
			<span class="layer" style="--l: 770; --t: 27; --w: 102; --h: 129">
				<img class="fill" src={src('front/stamp.svg')} alt="" />
			</span>
		{/if}
		{#each face.strokes as [file, l, t, w, h, inset]}
			<span class="layer" style="--l: {l}; --t: {t}; --w: {w}; --h: {h}">
				<span class="reach" style:inset><img class="fill" src={src(`${face.dir}/${file}`)} alt="" /></span>
			</span>
		{/each}
	</span>
{/snippet}

<button
	class="postcard"
	class:flipped
	aria-label="{(flipped ? back : front).label} (turn over)"
	onclick={() => (flipped = !flipped)}
>
	<span class="card">
		{@render side(front)}
		{@render side(back)}
	</span>
</button>

<style>
	/* A plain button the size of the card, with depth for turning it over. */
	.postcard {
		display: block;
		width: calc(900 * var(--pc));
		height: calc(600 * var(--pc));
		padding: 0;
		border: 0;
		background: none;
		cursor: pointer;
		perspective: calc(3000 * var(--pc));
	}

	.postcard:focus-visible {
		outline: calc(4 * var(--pc)) solid #fff;
		outline-offset: calc(10 * var(--pc));
	}

	.card {
		display: block;
		position: relative;
		height: 100%;
		transform-style: preserve-3d;
		transition: transform 0.7s cubic-bezier(0.3, 0, 0.2, 1);
	}

	.flipped .card {
		transform: rotateY(180deg);
	}

	@media (prefers-reduced-motion: reduce) {
		.card {
			transition: none;
		}
	}

	/* The back starts turned away, so a half turn brings it round. */
	.face {
		position: absolute;
		inset: 0;
		overflow: hidden;
		backface-visibility: hidden;
	}

	.back {
		transform: rotateY(180deg);
	}

	.paper {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		object-fit: cover;
		pointer-events: none;
	}

	.layer {
		position: absolute;
		left: calc(var(--l) * var(--pc));
		top: calc(var(--t) * var(--pc));
		width: calc(var(--w) * var(--pc));
		height: calc(var(--h) * var(--pc));
	}

	.stamp {
		border: var(--pc) solid #000;
	}

	.reach {
		position: absolute;
	}

	.fill {
		display: block;
		width: 100%;
		height: 100%;
		max-width: none;
	}
</style>
