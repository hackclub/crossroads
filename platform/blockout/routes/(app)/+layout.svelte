<script>
	import { page } from '$app/state';
	import { view } from '#lib/state.svelte.js';
	import { getData, PROGRAM } from '#lib/data.js';

	let { children } = $props();
	const d = $derived(getData(view.scenario));

	const nav = [
		{ href: '/home', label: 'Home' },
		{ href: '/projects', label: 'My projects' },
		{ href: '/explore', label: 'Explore' },
		{ href: '/shop', label: 'Shop' },
		{ href: '/trip', label: 'The trip' },
		{ href: '/leaderboard', label: 'Leaderboard' },
		{ href: '/guides', label: 'Guides & FAQ' }
	];

	const isActive = (href) => page.url.pathname === href || page.url.pathname.startsWith(href + '/');
</script>

<div class="shell">
	<nav class="sidebar" aria-label="Main">
		<a class="brand" href="/home">Crossroads</a>
		{#each nav as item}
			<a class="nav" class:active={isActive(item.href)} href={item.href}>
				{item.label}
				{#if item.href === '/shop'}<span class="small">{d.balance} {PROGRAM.currency}</span>{/if}
			</a>
		{/each}
		<a class="nav" class:active={isActive('/notifications')} href="/notifications">
			Notifications
			{#if d.notifications.length}<span class="badge info">{d.notifications.length}</span>{/if}
		</a>
		<div class="spacer"></div>
		<a class="me" href="/settings">
			<span class="avatar">{d.me.name[0]}</span>
			<span class="stack-sm">
				<strong class="small">{d.me.name}</strong>
				<span class="muted small">Settings</span>
			</span>
		</a>
	</nav>

	<main class="main">
		{@render children()}
	</main>
</div>
