// Getting ahead of the next click, so a page is there the moment it's opened.
//
// - Resting the pointer on a link (or focusing it, or touching it) asks the
//   backend for what that page shows. Pages start from the last answer to
//   each request (peek in api.js), so by the time the click lands the page
//   has its numbers.
//   SvelteKit already fetches the page's code on hover (app.html).
// - Links inside an element marked data-press go on the press of the mouse
//   button, not its release: about a tenth of a second sooner. Only for plain
//   moves between pages (the top bar, project rows), because a press can't be
//   taken back by dragging off the link the way a click can. Buttons that buy,
//   ship or open something are buttons, not links, and still wait for a click.
import { goto } from '$app/navigation';
import { peek, warm } from './api.js';

// What each page asks for when it opens (its GETs, as paths under /api).
const NEEDS = [
	[
		/^\/home$/,
		() => [
			'/projects',
			'/boxes',
			'/special-stamps',
			'/shop/items',
			// Unshipped time, once the projects are known.
			...(peek('/projects') ?? []).filter((p) => p.hackatime_projects.length).map((p) => `/projects/${p.id}`)
		]
	],
	[/^\/shop$/, () => ['/shop/items', '/boxes']],
	[/^\/boxes$/, () => ['/boxes', '/box-types', '/special-stamps', '/projects']],
	[/^\/projects\/(\d+)$/, (m) => [`/projects/${m[1]}`, '/special-stamps', '/box-types']],
	[/^\/shop\/orders$/, () => ['/orders']],
	// The project form lists their Hackatime projects.
	[/^\/projects\/(new|\d+\/edit)$/, () => ['/hackatime/projects']],
	[/^\/emporium$/, () => ['/emporium/items']],
	[/^\/review$/, () => ['/review/queue?stage=review']],
	[/^\/review\/templates$/, () => ['/review/templates']],
];

export function warmFor(pathname) {
	for (const [route, needs] of NEEDS) {
		const m = route.exec(pathname);
		if (m) for (const path of needs(m)) warm(path);
	}
}

// The link an event is on, if it's one that moves within the site in this tab.
function linkAt(e) {
	const a = e.target instanceof Element ? e.target.closest('a[href]') : null;
	if (!a || a.origin !== location.origin || a.hasAttribute('download')) return null;
	if (a.target && a.target !== '_self') return null;
	return a;
}

// Starts listening on the whole page. Returns a function that stops.
export function getAhead() {
	let pressed = null; // the link a press has already followed

	// Focus and touch mean it; a pointer may only be passing over on its way
	// somewhere else, so it has to stay on the link a moment first.
	const DWELL_MS = 60;
	let dwell;
	const onPoint = (e) => {
		const a = linkAt(e);
		if (a) warmFor(a.pathname);
	};
	const onOver = (e) => {
		clearTimeout(dwell);
		const a = linkAt(e);
		if (a) dwell = setTimeout(() => warmFor(a.pathname), DWELL_MS);
	};
	const onPress = (e) => {
		pressed = null;
		// A plain left press of a mouse: touches scroll, and a modifier or
		// another button opens a new tab or a menu.
		if (e.pointerType !== 'mouse' || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
		const a = linkAt(e);
		if (!a || !a.closest('[data-press]')) return;
		if (a.pathname === location.pathname && a.search === location.search) return;
		pressed = a;
		goto(a.href);
	};
	// The click that ends the press has nothing left to do.
	const onClick = (e) => {
		if (pressed && e.target instanceof Element && e.target.closest('a') === pressed) e.preventDefault();
		pressed = null;
	};

	const quiet = { passive: true };
	addEventListener('pointerover', onOver, quiet);
	addEventListener('focusin', onPoint, quiet);
	addEventListener('touchstart', onPoint, quiet);
	addEventListener('pointerdown', onPress);
	addEventListener('click', onClick, true); // before SvelteKit's own click handler
	return () => {
		clearTimeout(dwell);
		removeEventListener('pointerover', onOver);
		removeEventListener('focusin', onPoint);
		removeEventListener('touchstart', onPoint);
		removeEventListener('pointerdown', onPress);
		removeEventListener('click', onClick, true);
	};
}
