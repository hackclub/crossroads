<script>
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { signIn, isDevMode, devLoginUrl } from '#lib/api.js';
	import { refresh } from '#lib/session.svelte.js';
	import TopBar from '#lib/TopBar.svelte';

	const next = page.url.searchParams.get('next') || '/home';
	let dev = $state(false);
	let error = $state('');
	let name = $state('alice');
	let role = $state('participant');

	onMount(async () => {
		try {
			if (await refresh()) return goto(next, { replaceState: true });
		} catch (e) {
			error = `Couldn't reach the backend: ${e.message} Is it running on :8080?`;
		}
		dev = await isDevMode();
	});
</script>

<TopBar />
<main class="main narrow stack-lg">
	<div class="stack" style="align-items: center; text-align: center">
		<h1 class="page-title" style="margin-bottom: 0">Crossroads</h1>
		<p class="muted" style="max-width: 640px; font-size: 28px">
			Build for 60 approved hours, then travel across Europe by train with other teens, January 8–12, leaving from Paris.
		</p>
		{#if error}<p class="callout warn small">{error}</p>{/if}
		<button class="btn primary" onclick={() => signIn(next)}>Sign in with Hack Club</button>
		<span class="small muted">New here? Signing in creates your account.</span>
	</div>

	{#if dev}
		<form class="panel" onsubmit={(e) => { e.preventDefault(); location.href = devLoginUrl(name, role, next); }}>
			<div class="panel-head">Dev sign-in</div>
			<div class="panel-body">
				<span class="small muted">The backend is in DEV_MODE, so you can sign in as a test user without Hack Club Auth. Each name is its own account.</span>
				<div class="row">
					<input bind:value={name} pattern="[a-z0-9-]+" required style="max-width: 200px; background: var(--deep)" aria-label="Test user name" />
					<select bind:value={role} style="max-width: 200px; background: var(--deep)" aria-label="Role">
						<option>participant</option><option>reviewer</option><option>admin</option>
					</select>
					<button class="btn primary">Sign in</button>
				</div>
			</div>
		</form>
	{/if}
</main>
