<script>
	// Reviewing the whole shop at once, on the admin Shop items page.
	//
	// Export downloads one JSON file: every item (price, what it's worth in
	// hours, where it can go, whether it's sold) and how many participants
	// live in each country, as counts only. No one's address or name is in
	// it, so it can be handed to someone (or Claude) to research.
	//
	// Import reads a file of proposed changes back and shows each one before
	// and after. Nothing changes until Apply, which saves them one item at a
	// time through PATCH /admin/shop-items/{id}, so the backend's checks run
	// and each one lands in the audit log (shop_item.changed). Prices stay
	// the same for everyone: a change sets an item's one price, where it can
	// go, or whether it's sold.
	//
	// The changes file:
	//   {"changes": [{"id": 24, "name": "Flipper Zero", "price": 950,
	//                 "regions": {"rule": "except", "countries": ["IN", "BR"]},
	//                 "active": true, "why": "Brazil seizes them"}]}
	// name is optional; if it's there and doesn't match item id, the change
	// is skipped, in case the file was made from another copy of the shop.
	import { api, get } from '../api.js';
	import { session } from '../session.svelte.js';

	let { items, onapplied } = $props();

	let busy = $state(false);
	let msg = $state(null);
	let plan = $state(null); // { rows, skipped }: the changes read from a file
	let results = $state(null); // per row: { ok, text }
	/** @type {HTMLInputElement} */
	let fileInput;

	const rate = $derived(session.me.stamps_per_hour);
	const hours = (price) => Math.round((price / rate) * 10) / 10;
	const regionNames = new Intl.DisplayNames(['en'], { type: 'region' });
	const countryName = (c) => {
		try {
			return /^[A-Z]{2}$/.test(c) ? (regionNames.of(c) ?? c) : c;
		} catch {
			return c;
		}
	};
	const sameRegions = (a, b) => a.rule === b.rule && [...a.countries].sort().join() === [...b.countries].sort().join();
	const regionText = (r) => (r.rule === 'everywhere' ? 'Everywhere' : `${r.rule === 'only' ? 'Only' : 'Except'} ${r.countries.join(', ')}`);

	async function exportShop() {
		busy = true;
		msg = null;
		try {
			const [all, stats] = await Promise.all([get('/admin/shop-items'), get('/admin/activity?days=1')]);
			const participants = stats.countries.reduce((t, c) => t + c.people, 0);
			const file = {
				about:
					'Crossroads shop export: every item and where participants live (counts only). Prices are in tickets, the same for everyone; hours is the approved work an item costs. Send changes back as {"changes": [{"id", "name", "price"?, "regions"?, "active"?, "why"}]} and import them on the admin Shop items page.',
				exported_at: new Date().toISOString(),
				tickets_per_hour: rate,
				participants,
				countries: stats.countries.map((c) => ({ country: c.country, name: c.country ? countryName(c.country) : 'No address', people: c.people, shipped: c.shipped })),
				items: all.map((i) => ({
					id: i.id,
					name: i.name,
					kind: i.kind,
					description: i.description,
					price: i.price,
					hours: hours(i.price),
					active: i.active,
					emporium: i.emporium,
					stock: i.stock,
					stipend_cents: i.stipend_cents,
					grant_cents: i.grant_cents,
					grant_cents_per_hour: i.grant_cents_per_hour,
					regions: i.regions
				}))
			};
			const url = URL.createObjectURL(new Blob([JSON.stringify(file, null, '\t')], { type: 'application/json' }));
			const a = Object.assign(document.createElement('a'), { href: url, download: `crossroads-shop-${file.exported_at.slice(0, 10)}.json` });
			a.click();
			URL.revokeObjectURL(url);
			msg = { tone: 'good', text: `Exported ${file.items.length} items and ${file.countries.length} countries.` };
		} catch (e) {
			msg = { tone: 'warn', text: e.message };
		} finally {
			busy = false;
		}
	}

	// Reads a changes file into rows to preview, leaving out what wouldn't
	// change anything and saying why anything else was skipped.
	function readPlan(text) {
		let parsed;
		try {
			parsed = JSON.parse(text);
		} catch {
			throw new Error("That file isn't JSON.");
		}
		const changes = Array.isArray(parsed) ? parsed : parsed?.changes;
		if (!Array.isArray(changes)) throw new Error('The file needs a "changes" list.');
		const byId = new Map(items.map((i) => [i.id, i]));
		const rows = [];
		const skipped = [];
		for (const [n, c] of changes.entries()) {
			const label = `Change ${n + 1}${c?.name ? ` (${c.name})` : ''}`;
			const item = byId.get(c?.id);
			if (!item) {
				skipped.push(`${label}: no item #${c?.id}.`);
				continue;
			}
			if (c.name != null && c.name.trim().toLowerCase() !== item.name.trim().toLowerCase()) {
				skipped.push(`${label}: item #${item.id} is “${item.name}” here, so it was left alone.`);
				continue;
			}
			/** @type {{price?: number, active?: boolean, regions?: {rule: string, countries: string[]}}} */
			const body = {};
			if (c.price != null && c.price !== item.price) {
				if (item.kind === 'grant') skipped.push(`${label}: a grant's price comes from its rate, so its price was left alone.`);
				else if (!Number.isInteger(c.price) || c.price <= 0) skipped.push(`${label}: price has to be a whole number above 0.`);
				else body.price = c.price;
			}
			if (c.active != null && c.active !== item.active) body.active = !!c.active;
			if (c.regions != null) {
				const regions = { rule: c.regions.rule, countries: (c.regions.countries ?? []).map((x) => String(x).trim().toUpperCase()) };
				if (!sameRegions(regions, item.regions)) body.regions = regions;
			}
			if (!Object.keys(body).length) continue;
			rows.push({ item, body, why: c.why ?? '' });
		}
		return { rows, skipped };
	}

	async function openFile(e) {
		const file = e.currentTarget.files?.[0];
		e.currentTarget.value = '';
		if (!file) return;
		msg = null;
		results = null;
		try {
			plan = readPlan(await file.text());
			if (!plan.rows.length) msg = { tone: 'info', text: 'Nothing in that file would change anything.' };
		} catch (err) {
			plan = null;
			msg = { tone: 'warn', text: err.message };
		}
	}

	async function apply() {
		busy = true;
		results = [];
		for (const row of plan.rows) {
			try {
				await api('PATCH', `/admin/shop-items/${row.item.id}`, row.body);
				results.push({ ok: true, text: 'Saved' });
			} catch (err) {
				results.push({ ok: false, text: err.message });
			}
		}
		const failed = results.filter((r) => !r.ok).length;
		msg = failed
			? { tone: 'warn', text: `Saved ${results.length - failed} of ${results.length}. The rest are marked below.` }
			: { tone: 'good', text: `Saved all ${results.length} changes. Each is in the audit log.` };
		busy = false;
		onapplied?.();
	}
</script>

<section class="card review">
	<div class="head">
		<div>
			<h2>Review the whole shop</h2>
			<p class="small muted">
				Export every item and where participants live (counts only, no addresses) to research prices. Import the changes to see them before
				and after; nothing is saved until you apply them.
			</p>
		</div>
		<div class="row">
			<button class="btn small" disabled={busy} onclick={exportShop}>Export shop</button>
			<button class="btn small" disabled={busy} onclick={() => fileInput.click()}>Import changes</button>
			<input bind:this={fileInput} type="file" accept="application/json,.json" hidden onchange={openFile} />
		</div>
	</div>

	{#if msg}<p class="callout {msg.tone} small">{msg.text}</p>{/if}

	{#if plan}
		{#if plan.skipped.length}
			<details class="small" open={plan.skipped.length <= 5}>
				<summary>{plan.skipped.length} skipped</summary>
				<ul>{#each plan.skipped as s}<li>{s}</li>{/each}</ul>
			</details>
		{/if}
		{#if plan.rows.length}
			<div class="table-wrap">
				<table>
					<thead><tr><th>Item</th><th>Price</th><th>Where it can go</th><th>Sold</th><th>Why</th>{#if results}<th></th>{/if}</tr></thead>
					<tbody>
						{#each plan.rows as row, n (row.item.id)}
							{@const b = row.body}
							<tr>
								<td>{row.item.name}<div class="small muted">#{row.item.id}</div></td>
								<td>
									{#if b.price != null}
										<s class="muted">{row.item.price}</s> → <b>{b.price}</b>
										<div class="small muted">{hours(row.item.price)}h → {hours(b.price)}h</div>
									{:else}<span class="muted">{row.item.price}</span>{/if}
								</td>
								<td>
									{#if b.regions}
										<s class="muted">{regionText(row.item.regions)}</s> → <b title={b.regions.countries.map(countryName).join(', ')}>{regionText(b.regions)}</b>
									{:else}<span class="muted">{regionText(row.item.regions)}</span>{/if}
								</td>
								<td>
									{#if b.active != null}<b>{b.active ? 'Yes' : 'No'}</b>{:else}<span class="muted">{row.item.active ? 'Yes' : 'No'}</span>{/if}
								</td>
								<td class="small">{row.why}</td>
								{#if results}
									<td class="small">{#if results[n]}<span class:good={results[n].ok} class:bad={!results[n].ok}>{results[n].text}</span>{:else}…{/if}</td>
								{/if}
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
			<div class="row">
				{#if !results}
					<button class="btn primary" disabled={busy} onclick={apply}>Apply {plan.rows.length} change{plan.rows.length === 1 ? '' : 's'}</button>
				{/if}
				<button class="btn ghost" disabled={busy} onclick={() => { plan = null; results = null; msg = null; }}>{results ? 'Done' : 'Cancel'}</button>
			</div>
		{/if}
	{/if}
</section>

<style>
	.review { gap: 12px; }
	.head { display: flex; flex-wrap: wrap; align-items: flex-start; justify-content: space-between; gap: 12px 24px; }
	.head p { margin: 4px 0 0; max-width: 60ch; }
	details ul { margin: 6px 0 0; padding-left: 1.2em; }
	s { text-decoration-thickness: 1px; }
	.good { color: var(--good); }
	.bad { color: var(--warn); }
</style>
