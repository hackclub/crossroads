<script>
	import Note from '#lib/components/Note.svelte';
	import { view } from '#lib/state.svelte.js';
	import { getData } from '#lib/data.js';
	const d = $derived(getData(view.scenario));

	const accounts = $derived([
		{ name: 'Hack Club account', ok: true, detail: 'Signed in' },
		{ name: 'Slack', ok: d.me.slack, detail: d.me.slack ? '@jordan' : 'Not linked' },
		{ name: 'Hackatime', ok: d.me.hackatime, detail: d.me.hackatime ? 'Tracking' : 'Not connected' },
		{ name: 'GitHub', ok: d.me.github, detail: d.me.github ? '@jordan-codes' : 'Not linked (optional)' }
	]);
	const profile = $derived([
		{ name: 'ID verified', ok: d.me.verified },
		{ name: 'Mailing address', ok: d.me.address },
		{ name: 'Birthday', ok: d.me.birthday }
	]);
</script>

<div class="main narrow" style="padding: 0">
	<div class="page-head"><h1>Settings</h1></div>
	<div class="stack">
		<fieldset>
			<legend>Profile</legend>
			<label class="field"><span>Display name</span><input type="text" value={d.me.name} /></label>
			<label class="field"><span>Username</span><input type="text" value={d.me.username} /></label>
			<label class="field"><span>Country</span><input type="text" value={d.me.country} /></label>
			<div class="row"><button class="btn primary small">Save</button></div>
		</fieldset>

		<fieldset>
			<legend>Connected accounts</legend>
			<ul class="list">
				{#each accounts as a}
					<li>
						<span class={a.ok ? 'ok' : 'no'} style="width: 16px">{a.ok ? '✓' : '✗'}</span>
						<span style="flex: 1"><strong>{a.name}</strong> <span class="small muted">{a.detail}</span></span>
						<button class="btn small {a.ok ? 'ghost' : ''}">{a.ok ? 'Manage' : 'Connect'}</button>
					</li>
				{/each}
			</ul>
		</fieldset>

		<fieldset>
			<legend>Needed to ship and to order</legend>
			<ul class="checklist">
				{#each profile as p}<li><span class={p.ok ? 'ok' : 'no'}>{p.ok ? '✓' : '✗'}</span>{p.name}</li>{/each}
			</ul>
			<p class="small muted">These live on your Hack Club account so you only enter them once.</p>
			<div class="row"><a class="btn small" href="https://auth.hackclub.com" target="_blank" rel="noreferrer">Update on Hack Club</a><button class="btn small ghost">Check again</button></div>
			<Note from="Beest">✓/✗ readiness gate with a re-check button.</Note>
		</fieldset>

		<fieldset>
			<legend>Notifications</legend>
			<label class="check"><input type="checkbox" checked /> Slack DM when a review is done</label>
			<label class="check"><input type="checkbox" checked /> Slack DM when an order ships</label>
			<label class="check"><input type="checkbox" /> Daily streak reminder</label>
			<label class="check"><input type="checkbox" checked /> Show me on the leaderboard</label>
		</fieldset>

		<div class="row between">
			<button class="btn ghost">Sign out</button>
			<button class="link-btn small muted">Delete my account</button>
		</div>
	</div>
</div>
