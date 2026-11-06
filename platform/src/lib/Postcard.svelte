<script>
	// The front of the postcard from the crossroads-draft Figma file ("Postcard 1
	// Front", 195:3 on the assets page): "we're waiting in Berlin..." handwritten
	// on paper, with a stamp in the corner. It's 900 by 600 Figma pixels; set
	// --pc, one of those pixels, on a parent to size it.
	//
	// Every pen stroke is its own SVG from Figma, placed where Figma put it. The
	// paper texture is Figma's, resized to 1800x1350 WebP.
	const files = import.meta.glob('./assets/landing/postcard/*', { eager: true, import: 'default' });
	const src = (name) => files[`./assets/landing/postcard/${name}`];

	// [file, left, top, width, height, inset]: the stroke's layer in Figma
	// pixels, and how far its SVG reaches past that layer (the stroke's
	// thickness), as top right bottom left.
	const strokes = [
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
	];
</script>

<div class="postcard" role="img" aria-label="Postcard: we're waiting in Berlin...">
	<img class="paper" src={src('paper.webp')} alt="" />
	<div class="layer stamp" style="--l: 770; --t: 27; --w: 102; --h: 129">
		<img class="cover" src={src('stamp.png')} alt="" />
	</div>
	<div class="layer" style="--l: 770; --t: 27; --w: 102; --h: 129">
		<img class="fill" src={src('stamp.svg')} alt="" />
	</div>
	{#each strokes as [file, l, t, w, h, inset]}
		<div class="layer" style="--l: {l}; --t: {t}; --w: {w}; --h: {h}">
			<div class="reach" style:inset><img class="fill" src={src(file)} alt="" /></div>
		</div>
	{/each}
</div>

<style>
	.postcard {
		position: relative;
		width: calc(900 * var(--pc));
		height: calc(600 * var(--pc));
		overflow: hidden;
	}

	.paper,
	.cover {
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
