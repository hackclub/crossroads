<script>
	// The fraud check settings (GET/PATCH /api/admin/review-settings). They
	// apply to approvals from now on.
	import { onMount } from 'svelte';
	import { api, get } from '../api.js';

	let settings = $state(null);
	let form = $state({ everything: false, largeHours: 20, longDayHours: 14 });
	let msg = $state(null);
	let busy = $state(false);

	function fill(s) {
		settings = s;
		form = { everything: s.fraud_check_everything, largeHours: s.large_ship_seconds / 3600, longDayHours: s.long_day_seconds / 3600 };
	}
	onMount(async () => {
		try {
			fill(await get('/admin/review-settings'));
		} catch (e) {
			msg = { tone: 'warn', text: e.message };
		}
	});

	async function save(e) {
		e.preventDefault();
		msg = null;
		busy = true;
		try {
			fill(
				await api('PATCH', '/admin/review-settings', {
					fraud_check_everything: form.everything,
					large_ship_seconds: Math.round(form.largeHours * 3600),
					long_day_seconds: Math.round(form.longDayHours * 3600)
				})
			);
			msg = { tone: 'good', text: 'Saved. This applies to approvals from now on.' };
		} catch (err) {
			msg = { tone: 'warn', text: err.message };
		} finally {
			busy = false;
		}
	}
</script>

<form class="card stack" onsubmit={save}>
	<div class="card-title"><h2>Review settings</h2></div>
	<p class="small muted">
		A ship goes Review → Fraud check → HQ sign-off. The fraud check only gets ships the automatic checks flag (or that a reviewer
		sends there), unless you turn it on for everything. Who can do each stage is set per person under People.
	</p>
	{#if settings}
		<label class="check"><input type="checkbox" bind:checked={form.everything} /> Send every ship through the fraud check</label>
		<div class="grid-halves">
			<label class="field">
				<span>Big ship <em>hours</em></span>
				<input type="number" min="0.01" step="0.25" bind:value={form.largeHours} required />
				<small>Ships claiming more than this always get a fraud check.</small>
			</label>
			<label class="field">
				<span>Long day <em>hours</em></span>
				<input type="number" min="0.01" max="24" step="0.25" bind:value={form.longDayHours} required />
				<small>So do ships whose project has a day (UTC) with more than this.</small>
			</label>
		</div>
		<div class="row"><button class="btn primary" disabled={busy}>{busy ? 'Saving…' : 'Save'}</button></div>
	{/if}
	<div role="status" aria-live="polite">{#if msg}<p class="callout {msg.tone} small">{msg.text}</p>{/if}</div>
</form>
