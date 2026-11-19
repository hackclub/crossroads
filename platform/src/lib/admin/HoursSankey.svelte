<script>
	// Where every hour participants logged has gone, all time, as a Sankey,
	// after Stardance's hour funnel: nodes in columns left to right, and
	// ribbons as wide as the hours (or ships) that flow between them. The
	// backend builds the nodes and links (hoursFlowFrom in
	// backend/api/activity.go); this only lays them out. It's drawn like the
	// DailyChart cards around it: their blue for what flows on, grey for
	// what's waiting on someone, orange for what's lost. Every node is
	// labelled, hovering reads one out, and the table has it all.
	import { hours } from '../format.js';

	let { nodes, links } = $props();

	let unit = $state('hours'); // or 'ships'
	let width = $state(0);
	let active = $state(null); // the node or ribbon being read: { key, x, y, flip, title, value }

	const KINDS = { kept: 'Flows on', held: 'Waiting on someone', lost: 'Lost' };
	const NODE_W = 6;
	const PAD = 22; // between nodes in a column: room for one line of label
	const HEIGHT = 280;
	const LABEL_ROOM = 220; // for the last column's labels, right of it

	// Ships start at Shipped; cuts and added hours move hours, not ships.
	const amount = (x) => (unit === 'hours' ? x.seconds : x.partial ? 0 : x.ships);
	const fmt = (v) => (unit === 'hours' ? hours(v) : `${v.toLocaleString()} ship${v === 1 ? '' : 's'}`);

	const logged = $derived(nodes.find((n) => n.key === 'logged'));
	const shipped = $derived(nodes.find((n) => n.key === 'shipped'));
	const share = (v) =>
		unit === 'hours'
			? logged?.seconds ? `${((v / logged.seconds) * 100).toFixed(1)}% of every hour logged` : ''
			: shipped?.ships ? `${Math.round((v / shipped.ships) * 100)}% of ships` : '';

	const layout = $derived.by(() => {
		const w = Math.max(width, 760);
		const shown = nodes.filter((n) => amount(n) > 0);
		const keys = new Set(shown.map((n) => n.key));
		const flows = links.filter((l) => amount(l) > 0 && keys.has(l.source) && keys.has(l.target));
		const size = (n) => {
			const into = flows.filter((l) => l.target === n.key).reduce((t, l) => t + amount(l), 0);
			const out = flows.filter((l) => l.source === n.key).reduce((t, l) => t + amount(l), 0);
			return Math.max(amount(n), into, out);
		};
		const columns = [...new Set(shown.map((n) => n.column))].sort((a, b) => a - b);
		const byColumn = columns.map((c) => shown.filter((n) => n.column === c));
		// One scale for every column, so a ribbon is as wide at both ends.
		const k = Math.min(...byColumn.map((col) => (HEIGHT - PAD * (col.length - 1)) / Math.max(1, col.reduce((t, n) => t + size(n), 0))));
		const step = (w - LABEL_ROOM - NODE_W) / Math.max(1, columns.length - 1);
		const placed = new Map();
		byColumn.forEach((col, ci) => {
			let y = 0;
			for (const n of col) {
				const h = Math.max(2, size(n) * k);
				placed.set(n.key, { ...n, x: ci * step, y, h, out: 0, in: 0 });
				y += h + PAD;
			}
		});
		// Ribbons leave and arrive in node order, so they don't cross more
		// than they must.
		const order = (key) => [...placed.keys()].indexOf(key);
		const ribbons = flows
			.toSorted((a, b) => order(a.source) - order(b.source) || order(a.target) - order(b.target))
			.map((l) => {
				const s = placed.get(l.source);
				const t = placed.get(l.target);
				const th = amount(l) * k;
				const sy = s.y + s.out;
				const ty = t.y + t.in;
				s.out += th;
				t.in += th;
				const x0 = s.x + NODE_W;
				const x1 = t.x;
				const mx = (x0 + x1) / 2;
				const d = `M${x0},${sy} C${mx},${sy} ${mx},${ty} ${x1},${ty} L${x1},${ty + th} C${mx},${ty + th} ${mx},${sy + th} ${x0},${sy + th} Z`;
				return { ...l, key: `${l.source}>${l.target}`, d, kind: t.kind, from: s.label, to: t.label };
			});
		const bottom = Math.max(...[...placed.values()].map((n) => n.y + n.h));
		return { w, nodes: [...placed.values()], ribbons, height: bottom + 4 };
	});

	// The readout follows the pointer, flipping left near the right edge.
	function read(e, key, title, value) {
		const box = e.currentTarget.ownerSVGElement.parentElement.getBoundingClientRect();
		const x = e.clientX - box.left;
		active = { key, x, y: e.clientY - box.top, flip: x > box.width - 280, title, value };
	}
	// A ribbon lights up with its two ends; a node with everything touching it.
	const lit = (r) => !active || active.key === r.key || active.key === r.source || active.key === r.target;
</script>

<div class="sankey">
	<div class="controls">
		<ul class="legend">
			{#each Object.entries(KINDS) as [kind, label] (kind)}<li><span class="key {kind}"></span>{label}</li>{/each}
		</ul>
		<div class="toggle" role="group" aria-label="Show in">
			<button aria-pressed={unit === 'hours'} onclick={() => (unit = 'hours')}>Hours</button>
			<button aria-pressed={unit === 'ships'} onclick={() => (unit = 'ships')}>Ships</button>
		</div>
	</div>

	<div class="plot" bind:clientWidth={width}>
		<svg width={layout.w} height={layout.height} role="img" aria-label="Where every hour logged has gone: shipped, in review, rejected, approved, and in the Unified YSWS DB. The table below has every number.">
			{#each layout.ribbons as r (r.key)}
				<path
					class="ribbon {r.kind}"
					class:dim={!lit(r)}
					class:on={active?.key === r.key}
					d={r.d}
					role="presentation"
					onpointermove={(e) => read(e, r.key, `${r.from} → ${r.to}`, amount(r))}
					onpointerleave={() => (active = null)}
				/>
			{/each}
			{#each layout.nodes as n (n.key)}
				<g role="presentation" onpointermove={(e) => read(e, n.key, n.note ? `${n.label} (${n.note})` : n.label, amount(n))} onpointerleave={() => (active = null)}>
					<rect class="hit" x={n.x - 6} y={n.y - 8} width={NODE_W + 12} height={n.h + 16} />
					<rect class="node {n.kind}" x={n.x} y={n.y} width={NODE_W} height={n.h} rx="2" />
					<text x={n.x + NODE_W + 8} y={n.y + n.h / 2} dy="0.35em">
						<tspan class="name">{n.label}</tspan><tspan class="value" dx="8">{fmt(amount(n))}</tspan>
					</text>
				</g>
			{/each}
		</svg>
		{#if active}
			<div class="tip" class:flip={active.flip} style:left="{active.x}px" style:top="{active.y}px">
				<span>{active.title}</span>
				<strong>{fmt(active.value)}</strong>
				{#if share(active.value)}<span class="muted">{share(active.value)}</span>{/if}
			</div>
		{/if}
	</div>

	<details class="table">
		<summary class="small">Every step as a table</summary>
		<table>
			<thead><tr><th>Step</th><th class="num">Hours</th><th class="num">Of logged</th><th class="num">Ships</th></tr></thead>
			<tbody>
				{#each nodes as n (n.key)}
					<tr>
						<td><span class="key {n.kind}"></span>{n.label}{#if n.note} <span class="muted small">{n.note}</span>{/if}</td>
						<td class="num">{hours(n.seconds)}</td>
						<td class="num">{logged?.seconds ? `${((n.seconds / logged.seconds) * 100).toFixed(1)}%` : '–'}</td>
						<td class="num">{n.key === 'logged' || n.key === 'not_shipped' ? '–' : n.partial ? (n.ships ? `affects ${n.ships}` : '–') : n.ships.toLocaleString()}</td>
					</tr>
				{/each}
			</tbody>
		</table>
		<p class="small muted">A soft-rejected ship that was shipped again isn't counted: its hours flow through the newer ship.</p>
	</details>
</div>

<style>
	.sankey {
		/* The DailyChart series blue; a neutral for waiting, an orange for
		   lost. Checked for colour-blind separation on the card (about
		   #2f2d2c); grey is deliberately colourless, and every node is
		   labelled, so colour is never the only cue. */
		--kept: #3987e5;
		--held: #8d8782;
		--lost: #dc6b35;
		display: flex;
		flex-direction: column;
		gap: 10px;
		text-shadow: none;
	}

	.controls { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px 16px; }
	.legend { list-style: none; margin: 0; padding: 0; display: flex; flex-wrap: wrap; gap: 4px 18px; font-size: 16px; color: var(--muted); }
	.legend li { display: inline-flex; align-items: center; gap: 8px; }
	.key { display: inline-block; width: 10px; height: 10px; border-radius: 2px; }
	td .key { margin-right: 8px; }
	.key.kept { background: var(--kept); }
	.key.held { background: var(--held); }
	.key.lost { background: var(--lost); }

	.toggle { display: inline-flex; padding: 2px; border-radius: 999px; background: var(--deep); }
	.toggle button { padding: 2px 14px; border: 0; border-radius: 999px; background: transparent; color: var(--muted); font: inherit; font-size: 16px; cursor: pointer; }
	.toggle button[aria-pressed='true'] { background: var(--body); color: var(--text); }
	.toggle button:focus-visible { outline: 2px solid var(--muted); outline-offset: 2px; }

	/* Wider than a phone: it scrolls sideways in its own box. */
	.plot { position: relative; overflow-x: auto; padding: 4px 0 2px; }
	svg { display: block; overflow: visible; }

	.ribbon { opacity: 0.16; transition: opacity 0.15s ease-out; }
	.ribbon.lost { opacity: 0.32; }
	.ribbon.on { opacity: 0.5; }
	.ribbon.dim { opacity: 0.06; }
	.ribbon.kept, .node.kept { fill: var(--kept); }
	.ribbon.held, .node.held { fill: var(--held); }
	.ribbon.lost, .node.lost { fill: var(--lost); }
	.hit { fill: transparent; }

	/* Like the charts' ticks. The ribbons are faint enough to read across. */
	text { font-size: 16px; pointer-events: none; }
	.name { fill: var(--text); }
	.value { fill: var(--muted); font-variant-numeric: tabular-nums; }

	/* The same readout as the DailyChart cards. */
	.tip {
		position: absolute;
		transform: translate(12px, -110%);
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
		z-index: 2;
	}
	.tip.flip { transform: translate(calc(-100% - 12px), -110%); }
	.tip strong { font-size: 24px; font-weight: 400; font-variant-numeric: tabular-nums; }
	.tip .muted { font-size: 16px; }

	.table summary { cursor: pointer; color: var(--muted); font-size: 16px; }
	.table table { margin-top: 8px; font-size: 18px; }
	.table th { background: none; color: var(--muted); font-weight: 400; font-size: 16px; border-bottom: 1px solid rgb(255 255 255 / 0.12); }
	.table th, .table td { padding: 4px 10px; }
	.table p { margin: 8px 0 0; }
	.num { text-align: right; font-variant-numeric: tabular-nums; white-space: nowrap; }
</style>
