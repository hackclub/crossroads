<script>
	// The nav from the Figma "home" frame: a baked film strip (node 110:371) with the
	// flag and name in its left window, links in the middle window, and the
	// signed-in person in the right one.
	import { page } from '$app/state';
	import film from './assets/film-nav.webp';
	import flag from './assets/hack-club-flag.png';
	import { signOut } from './api.js';
	import { session, isReviewer, isAdmin } from './session.svelte.js';

	const links = [
		['/home', 'Home'],
		['/shop', 'Shop'],
		['/boxes', 'Boxes'],
		['/guides', 'Docs']
	];
	const active = (href) => page.url.pathname === href || page.url.pathname.startsWith(href + '/');
</script>

<header class="topbar" style="background-image: url({film})">
	<div class="window w-left">
		<a class="brand" href="/"><img src={flag} alt="Hack Club" width="60" height="60" />Crossroads</a>
	</div>
	<nav class="window w-mid" aria-label="Main">
		{#each links as [href, label]}
			<a class="nav" class:active={active(href)} aria-current={active(href) ? 'page' : undefined} {href}>{label}</a>
		{/each}
	</nav>
	{#if session.me}
		<div class="window w-right">
			<div class="who">
				<span>{session.me.name} · {session.me.stamps} stamps</span>
				<span class="links">
					<a href="/shop/orders">Orders</a>
					{#if isReviewer()}<a href="/review">Review</a>{/if}
					{#if isAdmin()}<a href="/admin">Admin</a>{/if}
					<button class="link-btn" onclick={signOut}>Sign out</button>
				</span>
			</div>
		</div>
	{/if}
</header>
