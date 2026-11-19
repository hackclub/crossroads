<script>
	// What people can choose for themselves (PATCH /me). Each setting saves as
	// soon as it's changed. Their name is in the top bar: clicking it changes it.
	import { api } from '#lib/api.js';
	import { session, refresh } from '#lib/session.svelte.js';

	let saving = $state('');
	let error = $state('');
	async function save(setting, value) {
		error = '';
		saving = setting;
		try {
			await api('PATCH', '/me', { [setting]: value });
			await refresh();
		} catch (e) {
			error = e.message;
		} finally {
			saving = '';
		}
	}
</script>

<div class="page-head"><h1>Settings</h1></div>

<div class="stack settings">
	{#if error}<p class="callout warn" role="alert">{error}</p>{/if}
	<div class="card">
		<label class="check">
			<input
				type="checkbox"
				checked={session.me.dms_enabled}
				disabled={saving === 'dms_enabled'}
				onchange={(e) => save('dms_enabled', e.currentTarget.checked)}
			/>
			<span class="setting">DMs from Tickets, Please</span>
		</label>
		<p class="small muted">A Slack DM when a project of yours is approved, isn't, or can be shipped again.</p>
	</div>
	<div class="card">
		<label class="check">
			<input
				type="checkbox"
				checked={session.me.on_landing_page === true}
				disabled={saving === 'on_landing_page'}
				onchange={(e) => save('on_landing_page', e.currentTarget.checked)}
			/>
			<span class="setting">Ride the trains on the homepage</span>
		</label>
		<p class="small muted">
			Your Slack picture in a train on <a href="/">the Crossroads homepage</a>, once you've had a project approved. Just the picture, not your name. It
			needs a picture you've put up yourself, not Slack's default.
		</p>
	</div>
</div>

<style>
	.settings { max-width: 760px; }
	.settings p { margin: 0; }
	.setting { font-size: 28px; }
</style>
