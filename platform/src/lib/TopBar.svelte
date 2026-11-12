<script>
	// The nav from the Figma "home" frame: a baked film strip (node 110:371) with the
	// Hack Club crossing sign and name in its left window, links in the middle window, and the
	// signed-in person in the right one.
	import { page } from '$app/state';
	import film from './assets/film-nav.webp';
	// The yellow "H C" railroad-crossing sign. The Figma's white crossbuck
	// ("Group 5" on the assets page), also tried, is in ./assets/crossbuck.svg.
	import logo from './assets/crossing-hc.webp';
	import { api, signOut } from './api.js';
	import StampIcon from './StampIcon.svelte';
	import { session, refresh, isReviewer, isAdmin } from './session.svelte.js';

	const links = [
		['/home', 'Home'],
		['/shop', 'Shop'],
		['/boxes', 'Boxes'],
		['/guides', 'Docs']
	];
	// Just the first name, without the "(dev)" test users carry.
	const firstName = $derived(session.me ? session.me.name.replace(/\s*\(dev\)$/, '').trim().split(/\s+/)[0] : '');
	// A name they chose shows in full; otherwise the first of their Hack Club name.
	const shownName = $derived(session.me?.name_customized ? session.me.name : firstName);

	// Clicking the name changes it (PATCH /me). Their legal name, on their Hack
	// Club account, stays as it is for prizes and grants.
	let nameDialog;
	let nameDraft = $state('');
	let nameError = $state('');
	let savingName = $state(false);
	function editName() {
		nameDraft = session.me.name;
		nameError = '';
		nameDialog.showModal();
	}
	async function saveName(name) {
		nameError = '';
		savingName = true;
		try {
			await api('PATCH', '/me', { name });
			await refresh();
			nameDialog.close();
		} catch (e) {
			nameError = e.message;
		} finally {
			savingName = false;
		}
	}
	const active = (href) => page.url.pathname === href || page.url.pathname.startsWith(href + '/');
</script>

<header class="topbar" style="background-image: url({film})">
	<div class="window w-left">
		<!-- The logo goes to Hack Club; the name goes back to the landing page. -->
		<a class="brand-logo" href="https://hackclub.com"><img src={logo} alt="Hack Club" width="60" height="60" /></a>
		<a class="brand" href="/">Crossroads</a>
	</div>
	<nav class="window w-mid" aria-label="Main">
		{#each links as [href, label]}
			<a class="nav" class:active={active(href)} aria-current={active(href) ? 'page' : undefined} {href}>{label}</a>
		{/each}
	</nav>
	{#if session.me}
		<div class="window w-right">
			<div class="who">
				<span class="wholine"><button class="name-btn" onclick={editName} title="Change your name" aria-haspopup="dialog">{shownName}</button> · {session.me.stamps}<StampIcon label="stamps" /></span>
				<span class="links">
					<a href="/shop/orders">Orders</a>
					{#if session.me.emporium}<a href="/emporium" title="The Smugglers' Emporium">Emporium</a>{/if}
					{#if isReviewer()}<a href="/review">Review</a>{/if}
					{#if isAdmin()}<a href="/admin">Admin</a>{/if}
					<button class="link-btn" onclick={signOut}>Sign out</button>
				</span>
			</div>
		</div>
	{/if}
</header>

<dialog class="name-dialog" bind:this={nameDialog} aria-labelledby="name-title">
	<form class="stack" onsubmit={(e) => { e.preventDefault(); saveName(nameDraft); }}>
		<h2 id="name-title">Your name</h2>
		<label class="field">
			<span>What people see on Crossroads</span>
			<!-- svelte-ignore a11y_autofocus -->
			<input bind:value={nameDraft} maxlength="50" required autocomplete="nickname" autofocus />
		</label>
		<p class="small muted">Your legal name stays on your Hack Club account, for prizes and grants.</p>
		{#if nameError}<p class="callout warn small" role="alert">{nameError}</p>{/if}
		<div class="row actions">
			{#if session.me?.name_customized}
				<button type="button" class="link-btn small" disabled={savingName} onclick={() => saveName(null)}>Use my Hack Club name</button>
			{/if}
			<span class="spacer"></span>
			<button type="button" class="btn ghost" onclick={() => nameDialog.close()}>Cancel</button>
			<button class="btn primary" disabled={savingName || !nameDraft.trim()}>{savingName ? 'Saving…' : 'Save'}</button>
		</div>
	</form>
</dialog>

<style>
	/* The name reads as the name, not a link, until you point at it. */
	.topbar .who .name-btn {
		padding: 0;
		font: inherit;
		color: var(--text);
		background: none;
		border: 0;
		cursor: pointer;
		max-width: 12em;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		vertical-align: bottom;
		text-decoration: underline transparent;
		text-underline-offset: 3px;
		transition: text-decoration-color 0.15s;
	}
	.topbar .who .name-btn:hover { text-decoration-color: currentColor; }
	.wholine { display: inline-flex; align-items: baseline; gap: 0.3em; }
	.name-dialog {
		width: min(480px, calc(100vw - 32px));
		padding: 24px 28px;
		color: var(--text);
		background: var(--row-bg);
		border: 0;
		border-radius: var(--radius);
		box-shadow: 0 24px 60px rgb(0 0 0 / 0.6);
		font-family: 'Crimson Pro', Georgia, serif;
	}
	.name-dialog::backdrop { background: rgb(0 0 0 / 0.7); }
	.name-dialog h2 { font-size: 36px; }
	.name-dialog p { margin: 0; }
	.name-dialog .btn.ghost { box-shadow: inset 0 0 0 2px var(--head); }
	.actions { align-items: center; }
	.spacer { flex: 1; }
</style>
