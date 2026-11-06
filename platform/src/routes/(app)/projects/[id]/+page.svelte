<script>
	import Train from '#lib/Train.svelte';
	import StampBox from '#lib/StampBox.svelte';
	import papua from '#lib/assets/stamps/papua.webp';
	import divan from '#lib/assets/stamps/divan.webp';
	import monet from '#lib/assets/stamps/monet.webp';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { api, get } from '#lib/api.js';
	import { refresh } from '#lib/session.svelte.js';
	import { hours, date, projectStatus, EFFECTS, effectShort } from '#lib/format.js';

	// "NOV 4" for the postmark.
	const shortDate = (iso) => new Date(iso).toLocaleDateString('en', { month: 'short', day: 'numeric' }).toUpperCase();

	let project = $state(null);
	let stamps = $state([]);
	let error = $state('');
	let actionError = $state('');
	let busy = $state(false);
	let shotFailed = $state(false);

	async function load() {
		error = '';
		try {
			[project, stamps] = await Promise.all([get(`/projects/${page.params.id}`), get('/special-stamps')]);
			project.stamps ??= [];
			// Open the stamp box the first time there's something in it.
			if (!boxTouched) {
				boxOpen = inBox.length > 0 && innerWidth > 820;
				boxTouched = true;
			}
		} catch (e) {
			error = e.message;
		}
	}
	onMount(load);

	// Postage for the ship postcards: the special stamps' own art, so new
	// special stamps show up here too. The local copies stand in until it loads.
	let postage = $state([papua, divan, monet]);
	onMount(async () => {
		try {
			const art = [...new Set((await get('/box-types')).flatMap((t) => t.odds.map((o) => o.special_stamp_type.image_url)).filter(Boolean))];
			if (art.length) postage = art;
		} catch {
			// Keep the local copies.
		}
	});
	// A random stamp per ship, seeded by its id so it doesn't change between visits.
	const stampFor = (sub) => postage[(Math.imul(sub.id, 2654435761) >>> 0) % postage.length];

	const latest = $derived(project?.submissions[0]);
	const isPending = $derived(latest?.status === 'pending');
	// ---- Stamps on the project ----
	// Stamps stuck on this project are part of it, so they're drawn for anyone
	// viewing it. Only the owner can stick new ones on or move them; this page
	// is owner-only today, so canEdit is always true here. A public project
	// page would render the same board with canEdit false.
	const canEdit = true;
	const inBox = $derived(stamps.filter((st) => !st.project_id && !st.submission_id));
	const placed = $derived(project?.stamps ?? []);
	const priority = $derived(!!latest && placed.some((st) => st.type.effect === 'priority_review' && st.submission_id === latest.id));
	const spiked = $derived(placed.some((st) => st.type.effect === 'stamp_multiplier'));
	const waitingExpress = $derived(placed.some((st) => st.type.effect === 'priority_review' && !st.submission_id));

	let innerWidth = $state(1440);
	let boxOpen = $state(false);
	let boxTouched = false;
	let board = $state(); // the screenshot card stamps sit on
	let drag = $state(null); // a stamp being dragged out of the box
	let toPlace = $state(null); // { stamp, x, y, rotation } waiting for the warning popup
	let placeDialog = $state();
	let placeError = $state('');
	let placing = $state(false);

	// Where a screen point lands on the board, 0-1 each way, or null if off it.
	function boardPoint(cx, cy, clamp = false) {
		const r = board?.getBoundingClientRect();
		if (!r) return null;
		const x = (cx - r.left) / r.width;
		const y = (cy - r.top) / r.height;
		if (clamp) return { x: Math.min(Math.max(x, 0), 1), y: Math.min(Math.max(y, 0), 1) };
		return x < 0 || x > 1 || y < 0 || y > 1 ? null : { x, y };
	}

	// Dragging a stamp out of the box: a copy follows the pointer, and letting
	// go over the card asks before sticking it on. A press without a drag is
	// a click, which StampBox turns into onpick.
	function startBoxDrag(stamp, e) {
		if (e.button !== 0) return;
		const el = e.currentTarget;
		el.setPointerCapture(e.pointerId);
		drag = { stamp, cx: e.clientX, cy: e.clientY, sx: e.clientX, sy: e.clientY, moved: false, over: false };
		const move = (ev) => {
			drag.cx = ev.clientX;
			drag.cy = ev.clientY;
			if (Math.hypot(ev.clientX - drag.sx, ev.clientY - drag.sy) > 6) drag.moved = true;
			drag.over = !!boardPoint(ev.clientX, ev.clientY);
		};
		const up = (ev) => {
			el.removeEventListener('pointermove', move);
			el.removeEventListener('pointerup', up);
			el.removeEventListener('pointercancel', up);
			const d = drag;
			drag = null;
			if (!d?.moved) return;
			el.dataset.dragged = '1';
			const at = ev.type === 'pointerup' ? boardPoint(ev.clientX, ev.clientY) : null;
			if (at) askToPlace(d.stamp, at.x, at.y);
		};
		el.addEventListener('pointermove', move);
		el.addEventListener('pointerup', up);
		el.addEventListener('pointercancel', up);
	}

	// Clicking (or Enter on) a stamp in the box: same popup, at a free-ish spot.
	const pickStamp = (stamp) => askToPlace(stamp, 0.15 + Math.random() * 0.7, 0.15 + Math.random() * 0.55);

	function askToPlace(stamp, x, y) {
		placeError = '';
		toPlace = { stamp, x, y, rotation: Math.round(Math.random() * 20 - 10) };
		placeDialog.showModal();
	}

	async function confirmPlace(e) {
		e.preventDefault();
		placing = true;
		placeError = '';
		try {
			const { stamp, x, y, rotation } = toPlace;
			const stuck = await api('POST', `/special-stamps/${stamp.id}/placement`, { project_id: project.id, x, y, rotation });
			project.stamps = [...project.stamps, stuck];
			stamps = stamps.map((st) => (st.id === stamp.id ? { ...st, project_id: project.id, submission_id: stuck.submission_id } : st));
			placeDialog.close();
		} catch (err) {
			placeError = err.message;
		} finally {
			placing = false;
		}
	}

	// Moving a stamp that's already on the card, by pointer or arrow keys.
	function startMove(st, e) {
		if (!canEdit || e.button !== 0) return;
		const el = e.currentTarget;
		el.setPointerCapture(e.pointerId);
		const from = { x: st.x, y: st.y };
		let moved = false;
		const move = (ev) => {
			const at = boardPoint(ev.clientX, ev.clientY, true);
			if (!at) return;
			moved = true;
			st.x = at.x;
			st.y = at.y;
		};
		const up = () => {
			el.removeEventListener('pointermove', move);
			el.removeEventListener('pointerup', up);
			el.removeEventListener('pointercancel', up);
			if (moved && (st.x !== from.x || st.y !== from.y)) saveSpot(st, from);
		};
		el.addEventListener('pointermove', move);
		el.addEventListener('pointerup', up);
		el.addEventListener('pointercancel', up);
	}

	let nudgeTimer;
	function nudge(st, e) {
		const step = e.shiftKey ? 0.08 : 0.02;
		const d = { ArrowLeft: [-step, 0], ArrowRight: [step, 0], ArrowUp: [0, -step], ArrowDown: [0, step] }[e.key];
		if (!d) return;
		e.preventDefault();
		const from = { x: st.x, y: st.y };
		st.x = Math.min(Math.max(st.x + d[0], 0), 1);
		st.y = Math.min(Math.max(st.y + d[1], 0), 1);
		clearTimeout(nudgeTimer);
		nudgeTimer = setTimeout(() => saveSpot(st, from), 400);
	}

	async function saveSpot(st, from) {
		try {
			await api('POST', `/special-stamps/${st.id}/placement`, { project_id: project.id, x: st.x, y: st.y });
		} catch (err) {
			st.x = from.x;
			st.y = from.y;
			actionError = `Couldn't move the stamp: ${err.message}`;
		}
	}
	const missing = $derived(
		project
			? [
					!project.description && 'a description',
					!project.repo_url && 'a repo link',
					!project.demo_url && 'a demo link',
					!project.screenshot_url && 'a screenshot',
					!project.hackatime_projects.length && 'a Hackatime project'
				].filter(Boolean)
			: []
	);

	async function ship() {
		actionError = '';
		busy = true;
		try {
			await api('POST', `/projects/${project.id}/submissions`);
			await load();
		} catch (e) {
			actionError = e.message;
			if (e.status === 409) await load();
		} finally {
			busy = false;
		}
	}

	async function remove() {
		if (!confirm(`Delete ${project.name}? This can't be undone.`)) return;
		try {
			await api('DELETE', `/projects/${project.id}`);
			await refresh();
			goto('/home');
		} catch (e) {
			actionError = e.message;
		}
	}
</script>

<svelte:window bind:innerWidth />

{#if error}
	<p class="callout warn">{error}</p>
	<p><a href="/home">Back home</a></p>
{:else if !project}
	<p class="muted">Loading… (this asks Hackatime, so it can take a moment)</p>
{:else}
	{@const s = projectStatus(project)}
	<div class="stack-lg">
		<div class="stack-sm" style="align-items: center">
			<h1 class="page-title" style="margin-bottom: 4px">{project.name}</h1>
			<span class="badge {s.tone}">{s.label}</span>
		</div>

		<div class="stack">
			<Train label="Screenshot and links">
			<div class="hero-well" bind:this={board} class:receiving={drag?.moved} class:over={drag?.over}>
				{#if project.screenshot_url && !shotFailed}
					<img src={project.screenshot_url} alt="Screenshot of {project.name}" onerror={() => (shotFailed = true)} />
				{:else}
					<span class="muted small">{project.screenshot_url ? "The screenshot didn't load." : 'No screenshot yet.'}</span>
				{/if}
				<div class="pill-row pills">
					{#if project.demo_url}<a class="chip" href={project.demo_url} target="_blank" rel="noreferrer">Demo</a>{/if}
					{#if project.repo_url}<a class="chip" href={project.repo_url} target="_blank" rel="noreferrer">Repo</a>{/if}
					<a class="chip" href="/projects/{project.id}/edit">Edit</a>
					{#if !project.submissions.length}<button class="chip" onclick={remove}>Delete</button>{/if}
				</div>
				<!-- Special stamps stuck on the project. -->
				{#each placed as st (st.id)}
					{#if canEdit}
						<button
							type="button"
							class="placed"
							style="left: {st.x * 100}%; top: {st.y * 100}%; rotate: {st.rotation}deg"
							onpointerdown={(e) => startMove(st, e)}
							onkeydown={(e) => nudge(st, e)}
							aria-label="{st.type.name} stamp ({effectShort(st.type.effect)}). Use the arrow keys to move it."
							title="{st.type.name}: {effectShort(st.type.effect)}"
						>{#if st.type.image_url}<img src={st.type.image_url} alt="" draggable="false" />{:else}<span class="blank">{st.type.name}</span>{/if}</button>
					{:else}
						<span class="placed" style="left: {st.x * 100}%; top: {st.y * 100}%; rotate: {st.rotation}deg" title="{st.type.name}: {effectShort(st.type.effect)}">
							{#if st.type.image_url}<img src={st.type.image_url} alt="{st.type.name} stamp" />{:else}<span class="blank">{st.type.name}</span>{/if}
						</span>
					{/if}
				{/each}
			</div>
			</Train>

			<section class="panel">
				<div class="panel-head">Description</div>
				<div class="panel-body"><p style="white-space: pre-wrap">{project.description || 'No description yet.'}</p></div>
			</section>

			<div class="grid-halves">
				<section class="panel">
					<div class="panel-head">Ship it</div>
					<div class="panel-body">
						<span class="small muted">A reviewer tries your project and approves the new hours you tracked on it. Approved hours pay stamps.</span>
						{#if isPending}
							<p class="callout info small">Waiting for review since {date(latest.submitted_at)}{priority ? ', with priority review from your Express stamp' : ''}. The result shows up below.</p>
						{:else if missing.length}
							<p class="callout warn small">Before you can ship, add {missing.join(', ')}. <a href="/projects/{project.id}/edit">Edit project</a></p>
						{:else if project.hackatime_error}
							<p class="callout warn small">{project.hackatime_error}</p>
						{:else if !project.unsubmitted_seconds}
							<p class="callout info small">No new time to ship. Keep building, then ship again.</p>
						{:else}
							<p><span class="big">{hours(project.unsubmitted_seconds)}</span> of new time goes to review.</p>
							{#if waitingExpress}<p class="small">Your Express stamp gives this ship priority review.</p>{/if}
							<div class="row"><button class="btn primary" disabled={busy} onclick={ship}>{busy ? 'Shipping…' : project.submissions.length ? 'Ship again' : 'Ship'}</button></div>
						{/if}
						{#if spiked}<p class="small">Golden Spike: approved ships of this project earn 10% more stamps.</p>{/if}
						{#if actionError}<p class="callout warn small">{actionError}</p>{/if}
					</div>
				</section>

				<section class="panel">
					<div class="panel-head">Hours</div>
					<div class="panel-body">
						<div class="grid-halves" style="gap: 12px">
							<div class="tile"><span class="label">Tracked</span><span class="value">{hours(project.hackatime_seconds ?? 0)}</span></div>
							<div class="tile"><span class="label">Approved</span><span class="value">{hours(project.approved_seconds)}</span></div>
						</div>
						{#if project.unsubmitted_seconds != null}<span class="small">{hours(project.unsubmitted_seconds)} not shipped yet.</span>{/if}
						<div class="pill-row">
							{#each project.hackatime_projects as h}<span class="badge">{h}</span>{:else}<span class="small muted">No Hackatime projects linked. <a href="/projects/{project.id}/edit">Link some</a></span>{/each}
						</div>
						<button class="link-btn small muted" onclick={load}>Refresh from Hackatime</button>
					</div>
				</section>
			</div>
		</div>

		<section class="stack ships">
			<h2>Ships</h2>
			{#each project.submissions as sub, i}
				{@const label = sub.status === 'pending' ? 'In review' : sub.status === 'approved' ? 'Approved' : 'Not approved'}
				<!-- Each ship is a postcard: the reviewer's note on the left, the
				     stamp (hours), postmark (status and date) and details on the right. -->
				<article class="postcard {sub.status}" aria-label="Ship #{project.submissions.length - i}: {label}">
					<div class="message">
						<h3>Ship #{project.submissions.length - i}</h3>
						{#if sub.feedback}
							<p class="note">“{sub.feedback}”</p>
							<span class="sign">Your reviewer</span>
						{:else if sub.status === 'pending'}
							<p class="note">A reviewer will try your project soon. Keep building while you wait.</p>
						{:else if sub.status === 'approved'}
							<p class="note">Approved. The reviewer left no notes.</p>
						{:else}
							<p class="note">Not approved this time. You can ship this time again.</p>
						{/if}
					</div>
					<div class="back">
						<div class="franking" aria-hidden="true">
							<img class="stamp" src={stampFor(sub)} alt="" />
							<div class="postmark">
								<span>{label}</span>
								<b>{hours(sub.approved_seconds ?? sub.new_seconds)}</b>
								<span>{shortDate(sub.reviewed_at ?? sub.submitted_at)}</span>
							</div>
						</div>
						<dl class="lines">
							<div><dt>Status</dt><dd>{label}</dd></div>
							<div><dt>Claimed</dt><dd>{hours(sub.new_seconds)}</dd></div>
							{#if sub.approved_seconds != null}<div><dt>Approved</dt><dd>{hours(sub.approved_seconds)}</dd></div>{/if}
							<div><dt>Hackatime</dt><dd>{sub.hackatime_projects.join(', ')}</dd></div>
							<div><dt>Shipped</dt><dd>{date(sub.submitted_at)}</dd></div>
						</dl>
					</div>
				</article>
			{:else}
				<p class="muted">Not shipped yet.</p>
			{/each}
		</section>
	</div>

	{#if canEdit}
		<StampBox stamps={inBox} bind:open={boxOpen} ondragstart={startBoxDrag} onpick={pickStamp} />
	{/if}

	<!-- The stamp following the pointer while it's dragged out of the box. -->
	{#if drag?.moved && drag.stamp.type.image_url}
		<img class="drag-ghost" class:over={drag.over} src={drag.stamp.type.image_url} alt="" style="left: {drag.cx}px; top: {drag.cy}px" />
	{/if}

	<dialog bind:this={placeDialog} class="place-dialog" aria-labelledby="place-title" onclose={() => (toPlace = null)}>
		{#if toPlace}
			<form class="stack" method="dialog" onsubmit={confirmPlace}>
				<div class="place-head">
					{#if toPlace.stamp.type.image_url}<img src={toPlace.stamp.type.image_url} alt="" />{/if}
					<div class="stack-sm">
						<h2 id="place-title">Stick {toPlace.stamp.type.name} on {project.name}?</h2>
						<p>{EFFECTS[toPlace.stamp.type.effect]?.long ?? ''}</p>
					</div>
				</div>
				<p class="callout warn">Once a stamp is on a project, it stays there. You can move it around this project, but you can't take it off or move it to a different project.</p>
				{#if placeError}<p class="callout warn" role="alert">{placeError}</p>{/if}
				<div class="row" style="justify-content: flex-end">
					<button type="button" class="btn ghost" onclick={() => placeDialog.close()}>Cancel</button>
					<button class="btn primary" disabled={placing}>{placing ? 'Sticking…' : 'Stick it on'}</button>
				</div>
			</form>
		{/if}
	</dialog>
{/if}

<style>
	/* The screenshot block from "project main", now a car on the track,
	   with the pills on it. Train draws the car. */
	.hero-well {
		position: relative;
		display: grid;
		place-items: center;
		min-height: 290px;
		padding: 16px 16px 64px;
		border-radius: var(--radius);
	}
	.hero-well img { border-radius: 12px; }
	.hero-well img { width: 100%; max-height: 520px; object-fit: contain; }
	.pills { position: absolute; left: 14px; bottom: 14px; }

	/* ---- Ships as postcards ---- */
	.ships { align-items: center; }
	.postcard {
		--paper: #efe9dc;
		--ink: #2b2522;
		--ink-soft: #5c524c;
		--mark: #2f4566; /* postmark ink: navy while in review */
		display: grid;
		grid-template-columns: minmax(0, 1.1fr) minmax(0, 1fr);
		gap: 24px 28px;
		width: min(780px, 100%);
		min-height: 300px;
		padding: 28px 32px;
		color: var(--ink);
		text-shadow: none;
		/* Cream card stock with a faint warmth from the light leak. */
		background: linear-gradient(rgb(239 233 220 / 0.94), rgb(239 233 220 / 0.94)), var(--leak);
		border-radius: 6px;
		box-shadow: 0 2px 4px rgb(0 0 0 / 0.3), 0 18px 40px rgb(0 0 0 / 0.5);
	}
	.postcard.approved { --mark: #3d6b2c; }
	.postcard.rejected { --mark: #9a3b26; }
	.message { display: flex; flex-direction: column; gap: 12px; }
	.message h3 { font-size: 36px; }
	.note { font-size: 30px; font-style: italic; line-height: 1.35; }
	.sign { margin-top: auto; font-size: 21px; color: var(--ink-soft); }
	.back {
		position: relative;
		display: flex;
		flex-direction: column;
		padding-left: 28px;
		border-left: 1px solid rgb(43 37 34 / 0.3);
	}
	/* Real postage (lib/assets/stamps), with its own perforated edge. */
	/* The stamp with the postmark hanging off its left edge, whatever its shape. */
	.franking { position: relative; align-self: flex-end; margin-left: 70px; }
	.stamp {
		display: block;
		width: auto;
		max-width: 190px;
		height: 132px;
		object-fit: contain;
		rotate: 2deg;
		filter: drop-shadow(0 1px 2px rgb(0 0 0 / 0.3));
	}
	.postmark {
		position: absolute;
		top: 30px;
		left: -82px; /* catches only the stamp's edge */
		display: grid;
		place-content: center;
		gap: 0;
		width: 116px;
		height: 116px;
		font-size: 17px;
		line-height: 1.1;
		letter-spacing: 0.06em;
		text-align: center;
		text-transform: uppercase;
		color: var(--mark);
		border: 3px solid var(--mark);
		border-radius: 50%;
		rotate: -14deg;
		opacity: 0.85;
	}
	.postmark b { font-weight: 400; font-size: 28px; letter-spacing: 0; text-transform: none; }
	.lines { display: flex; flex-direction: column; margin: 18px 0 0; }
	.lines div { display: flex; gap: 12px; padding: 6px 0 2px; font-size: 21px; border-bottom: 1px solid rgb(43 37 34 / 0.25); }
	.lines dt { flex: none; width: 96px; color: var(--ink-soft); }
	.lines dd { margin: 0; min-width: 0; overflow-wrap: anywhere; }
	@media (max-width: 640px) {
		.postcard { grid-template-columns: 1fr; padding: 22px 20px; }
		.back { padding: 20px 0 0; border-left: 0; border-top: 1px solid rgb(43 37 34 / 0.3); }
		.postmark { top: 20px; }
	}

	/* ---- The stamp board (the screenshot card) and stamps on it ---- */
	.hero-well.receiving { box-shadow: inset 0 0 0 3px rgb(255 255 255 / 0.35); }
	.hero-well.over { box-shadow: inset 0 0 0 3px rgb(255 255 255 / 0.9); background: rgb(255 255 255 / 0.05); }
	.placed {
		position: absolute;
		z-index: 2;
		translate: -50% -50%;
		padding: 0;
		border: 0;
		background: none;
		touch-action: none;
	}
	button.placed { cursor: grab; }
	button.placed:active { cursor: grabbing; }
	.placed img { display: block; height: 96px; width: auto; max-width: 128px; filter: drop-shadow(0 3px 6px rgb(0 0 0 / 0.6)); }
	.placed .blank { display: grid; place-items: center; width: 80px; height: 96px; font-size: 16px; border: 2px dashed rgb(255 255 255 / 0.6); }
	.drag-ghost {
		position: fixed;
		z-index: 40;
		height: 110px;
		width: auto;
		translate: -50% -50%;
		rotate: -6deg;
		pointer-events: none;
		opacity: 0.85;
		filter: drop-shadow(0 10px 18px rgb(0 0 0 / 0.6));
	}
	.drag-ghost.over { opacity: 1; rotate: 0deg; }

	.place-dialog {
		width: min(560px, calc(100vw - 32px));
		padding: 24px 28px;
		color: var(--text);
		background: var(--row-bg);
		border: 0;
		border-radius: var(--radius);
		box-shadow: 0 24px 60px rgb(0 0 0 / 0.6);
		text-shadow: none;
	}
	.place-dialog::backdrop { background: rgb(0 0 0 / 0.7); }
	.place-dialog .btn.ghost { box-shadow: inset 0 0 0 2px var(--head); }
	.place-head { display: flex; align-items: center; gap: 18px; }
	.place-head img { height: 110px; width: auto; flex: none; rotate: -4deg; filter: drop-shadow(0 4px 8px rgb(0 0 0 / 0.5)); }
	.place-head h2 { font-size: 34px; }
</style>
