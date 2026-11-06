<script>
	// One number per day for the stats panel: columns, or a line over a faint
	// area for running totals. Hovering, or focusing it and using the arrow
	// keys, reads out a day; the panel's table has every value too.
	import { utcDay } from '../format.js';

	// days: the API's day rows. value picks the number from one. format turns
	// a number into text for the axis and readout. label follows the number in
	// the readout ("3 on the platform").
	let { days, value, format = (n) => n.toLocaleString(), kind = 'columns', label, integer = true } = $props();

	const PLOT = 180; // the plot's height
	const TOP = 10; // room for the top tick label
	const AXIS = 30; // the date labels under the plot
	const RIGHT = 8;

	let width = $state(0);
	let active = $state(null); // index of the day being read out

	// Round tick steps (1, 2, 5, 10…), four or so up to at least the biggest
	// value; whole numbers for counts.
	function niceTicks(max) {
		if (max <= 0) return [0, 1];
		const raw = max / 4;
		const magnitude = 10 ** Math.floor(Math.log10(raw));
		let step = [1, 2, 5, 10].map((m) => m * magnitude).find((s) => s >= raw);
		if (integer) step = Math.max(1, Math.ceil(step));
		const out = [];
		for (let t = 0; t < max + step / 2; t += step) out.push(t);
		if (out.at(-1) < max) out.push(out.at(-1) + step);
		return out;
	}

	const values = $derived(days.map(value));
	const ticks = $derived(niceTicks(Math.max(0, ...values)));
	const top = $derived(ticks.at(-1));
	const left = $derived(12 + Math.max(...ticks.map((t) => format(t).length)) * 8);
	const inner = $derived(Math.max(0, width - left - RIGHT));
	const band = $derived(inner / days.length);
	const barWidth = $derived(Math.max(1, Math.min(24, band - 2)));
	const x = (i) => left + band * (i + 0.5);
	const y = (v) => TOP + PLOT - (v / top) * PLOT;
	const base = TOP + PLOT;

	// Today always gets a date label; the others are spaced out from it.
	const every = $derived(Math.max(1, Math.ceil(days.length / Math.max(1, Math.floor(inner / 72)))));
	const labelled = $derived(days.map((_, i) => i).filter((i) => (days.length - 1 - i) % every === 0));
	const labelX = (i) => Math.min(Math.max(x(i), left + 20), width - RIGHT - 20);

	// A column with a 4px rounded top, square on the baseline.
	function column(i) {
		const h = (values[i] / top) * PLOT;
		if (h <= 0) return '';
		const r = Math.min(4, barWidth / 2, h);
		const x0 = x(i) - barWidth / 2;
		const x1 = x0 + barWidth;
		const yTop = base - h;
		return `M${x0},${base}V${yTop + r}Q${x0},${yTop} ${x0 + r},${yTop}H${x1 - r}Q${x1},${yTop} ${x1},${yTop + r}V${base}Z`;
	}
	const line = $derived(values.map((v, i) => `${i ? 'L' : 'M'}${x(i)},${y(v)}`).join(''));
	const area = $derived(`${line}L${x(values.length - 1)},${base}L${x(0)},${base}Z`);

	function point(event) {
		const box = event.currentTarget.getBoundingClientRect();
		const i = Math.floor((event.clientX - box.left - left) / band);
		active = Math.min(days.length - 1, Math.max(0, i));
	}

	function key(event) {
		const last = days.length - 1;
		const next = { ArrowLeft: (active ?? last) - 1, ArrowRight: (active ?? last) + 1, Home: 0, End: last }[event.key];
		if (next === undefined) return;
		event.preventDefault();
		active = Math.min(last, Math.max(0, next));
	}

	const readout = $derived(active == null ? '' : `${utcDay(days[active].day)}: ${format(values[active])} ${label}`);
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
<div
	class="chart"
	bind:clientWidth={width}
	tabindex="0"
	role="group"
	aria-label="{label}, by day. Left and right arrow keys read each day."
	onkeydown={key}
	onfocus={() => (active ??= days.length - 1)}
	onblur={() => (active = null)}
>
	{#if width}
		<svg {width} height={TOP + PLOT + AXIS} aria-hidden="true" onpointermove={point} onpointerleave={() => (active = null)}>
			{#each ticks as t}
				<line class="grid" x1={left} x2={width - RIGHT} y1={y(t)} y2={y(t)} />
				<text class="tick" x={left - 8} y={y(t)} dy="0.35em" text-anchor="end">{format(t)}</text>
			{/each}
			{#each labelled as i}
				<text class="tick" x={labelX(i)} y={base + 22} text-anchor="middle">{i === days.length - 1 ? 'Today' : utcDay(days[i].day)}</text>
			{/each}
			{#if kind === 'area'}
				<path class="area" d={area} />
				<path class="line" d={line} />
				{#if active != null}
					<line class="crosshair" x1={x(active)} x2={x(active)} y1={TOP} y2={base} />
					<circle class="dot" cx={x(active)} cy={y(values[active])} r="4" />
				{/if}
				<circle class="dot" cx={x(values.length - 1)} cy={y(values.at(-1))} r="4" />
			{:else}
				{#if active != null}
					<rect class="hover-band" x={x(active) - band / 2} y={TOP} width={band} height={PLOT} />
				{/if}
				{#each values as _, i}
					<path class="column" class:active={active === i} d={column(i)} />
				{/each}
			{/if}
		</svg>
		{#if active != null}
			<div class="tip" class:flip={x(active) > width * 0.6} style:left="{x(active)}px" aria-hidden="true">
				<strong>{format(values[active])}</strong>
				<span>{label}</span>
				<span class="muted">{active === days.length - 1 ? 'Today' : utcDay(days[active].day)}</span>
			</div>
		{/if}
	{/if}
	<p class="visually-hidden" aria-live="polite">{readout}</p>
</div>

<style>
	.chart {
		/* Slot 1 of the dataviz palette's dark steps, checked against the card
		   (about #2f2d2c under its texture). */
		--series: #3987e5;
		--surface: #2f2d2c;
		--hairline: rgb(255 255 255 / 0.12);
		position: relative;
		text-shadow: none;
		border-radius: 8px;
	}

	.chart:focus-visible {
		outline: 2px solid var(--muted);
		outline-offset: 4px;
	}

	svg {
		display: block;
		overflow: visible;
		touch-action: pan-y;
	}

	.grid {
		stroke: var(--hairline);
		stroke-width: 1;
		shape-rendering: crispEdges;
	}

	.tick {
		fill: var(--muted);
		font-size: 16px;
		font-variant-numeric: tabular-nums;
	}

	.column {
		fill: var(--series);
	}

	.column.active {
		filter: brightness(1.3);
	}

	/* Behind the hovered day, so a day with nothing still shows it's being read. */
	.hover-band {
		fill: rgb(255 255 255 / 0.06);
	}

	.area {
		fill: var(--series);
		opacity: 0.1;
	}

	.line {
		fill: none;
		stroke: var(--series);
		stroke-width: 2;
		stroke-linejoin: round;
		stroke-linecap: round;
	}

	.crosshair {
		stroke: var(--muted);
		stroke-width: 1;
		shape-rendering: crispEdges;
	}

	.dot {
		fill: var(--series);
		stroke: var(--surface);
		stroke-width: 2;
	}

	.tip {
		position: absolute;
		top: 0;
		transform: translateX(12px);
		display: flex;
		flex-direction: column;
		padding: 6px 12px;
		background: var(--navy-deep);
		border-radius: 8px;
		box-shadow: 0 2px 10px rgb(0 0 0 / 0.5);
		font-size: 18px;
		line-height: 1.25;
		white-space: nowrap;
		pointer-events: none;
	}

	.tip.flip {
		transform: translateX(calc(-100% - 12px));
	}

	.tip strong {
		font-size: 24px;
	}

	.visually-hidden {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
	}
</style>
