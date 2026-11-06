<script>
	// A split-flap departures board, ported from the 21st.dev React component
	// (componentry/split-flap-display), made matte and square and without its
	// accent strips. Give it one line of `text`, or `rows` of { label, value },
	// each spread to the two ends of its line. Lines are cut or padded to
	// `columns` characters.
	//
	// Sizes are design pixels: inside an element that sets --u, like the
	// landing page's frame, the board scales with it. Elsewhere one is 1px.
	//
	//   <SplitFlapDisplay text="Next stop Paris" columns={16} size="lg" centered />
	//   <SplitFlapDisplay rows={[{ label: 'Hours', value: '42' }]} columns={12} />
	import SplitFlapCell from './SplitFlapCell.svelte';

	let {
		text = undefined,
		rows = undefined,
		columns = 14,
		size = 'md', // 'sm', 'md' or 'lg'
		centered = false, // centre each line in its columns instead of starting it at the left
		lowercase = false, // small letters instead of capitals
		staggerDelay = 30, // ms between neighbouring cells starting
		flipSpeed = 35, // ms per flap
		class: klass = ''
	} = $props();

	const single = $derived(Boolean(text && !rows));
	const lines = $derived(
		single
			? [text]
			: (rows ?? []).map(({ label, value }) => label + ' '.repeat(Math.max(1, columns - label.length - value.length)) + value)
	);
	// Accents are dropped (José shows as JOSE) and curly apostrophes
	// straightened, since the board has flaps for neither. Split by code
	// point, so an emoji takes one cell, which shows blank.
	// Centred lines are trimmed first, and an odd space left over goes on
	// the right.
	const fit = (line) => {
		if (centered) line = line.trim();
		line = line.normalize('NFD').replace(/\p{M}/gu, '').replace(/[‘’]/g, "'");
		const chars = [...(lowercase ? line.toLowerCase() : line.toUpperCase())];
		const start = centered ? Math.max(0, Math.floor((columns - chars.length) / 2)) : 0;
		return Array.from({ length: columns }, (_, i) => chars[i - start] ?? ' ');
	};
</script>

<div class="board {klass}" class:single>
	{#each lines as line}
		<div class="line">
			{#each fit(line) as char, i}
				<SplitFlapCell {char} {size} delay={i * staggerDelay} {flipSpeed} {lowercase} />
			{/each}
		</div>
	{/each}
</div>

<style>
	/* Matte: a flat face and a soft shadow, without the original's sheen
	   gradient and top highlight. */
	.board {
		--px: var(--u, 1px); /* one design pixel */
		display: inline-flex;
		flex-direction: column;
		gap: calc(8 * var(--px));
		padding: calc(20 * var(--px));
		background: #0a0a0a;
		border: var(--px) solid rgba(255, 255, 255, 0.06);
		box-shadow: 0 calc(16 * var(--px)) calc(40 * var(--px)) rgba(0, 0, 0, 0.5);
	}
	.single { padding: calc(16 * var(--px)); }
	.line { display: flex; gap: calc(3 * var(--px)); }
</style>
