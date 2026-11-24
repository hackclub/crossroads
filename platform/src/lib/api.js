// Talks to the backend through the dev server's proxy (see vite.config.js).
// Every error the backend returns is written for people, so pages show
// err.message as is. See planning/INTEGRATING.md, section 3.

export class ApiError extends Error {
	constructor(status, message) {
		super(message);
		this.status = status;
	}
}

export async function api(method, path, body) {
	// Anything but a GET changes something, so nothing remembered from before
	// it can be trusted (see forget). Before as well as after: a GET sent
	// while it's on its way could carry either answer.
	const changes = method !== 'GET';
	if (changes) forget();
	try {
		const res = await fetch('/api' + path, {
			method,
			headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
			body: body === undefined ? undefined : JSON.stringify(body)
		});
		if (res.status === 204) return undefined;
		const data = await res.json().catch(() => null);
		if (!res.ok) throw new ApiError(res.status, data?.error ?? `Something went wrong (${res.status}).`);
		return data;
	} finally {
		if (changes) forget();
	}
}

// The last answer to each GET. A page opened again starts from what it showed
// last time (peek) while it asks for the new answer, so it doesn't flash
// zeros or "nothing here yet" on the way. Only in memory: signing in, out or
// as someone else reloads the page, which empties it.
const seen = new Map(); // path → the last answer
const asked = new Map(); // path → when that answer came, in ms
const flying = new Map(); // path → the request for it that's still out
let era = 0; // goes up whenever something changes; answers from before don't count
export const peek = (path) => seen.get(path);

// Drops everything remembered, and stops requests already out from being
// shared or remembered when they land: they may describe how things were
// before the change. Any request that isn't a GET does this, so a page that
// reloads after saving gets a new answer, not one already on its way, and a
// page opened after an edit doesn't start from the version before it.
export function forget() {
	era++;
	seen.clear();
	asked.clear();
	flying.clear();
}

// Always asks the backend, so what a page gets back is new. Callers asking
// for the same thing at the same moment (a page and a car of the train, say)
// share one request.
export function get(path) {
	let request = flying.get(path);
	if (!request) {
		const began = era;
		request = api('GET', path)
			.then((data) => {
				if (began === era) {
					seen.set(path, data);
					asked.set(path, Date.now());
				}
				return data;
			})
			.finally(() => {
				if (flying.get(path) === request) flying.delete(path);
			});
		flying.set(path, request);
	}
	return request;
}

// Asks ahead of a visit that looks likely (see ahead.js), so peek has an
// answer when the page opens. Nothing happens if it was asked moments ago,
// which is what moving the pointer across a link does many times over.
const WARM_FOR_MS = 5000;
export function warm(path) {
	if (flying.has(path) || Date.now() - (asked.get(path) ?? 0) < WARM_FOR_MS) return;
	get(path).catch(() => {}); // the page itself will say what went wrong
}

// Uploads an image file and returns its URL. Not JSON, so not through api().
export async function uploadImage(file) {
	const form = new FormData();
	form.append('file', file);
	const res = await fetch('/api/uploads', { method: 'POST', body: form });
	const data = await res.json().catch(() => null);
	if (!res.ok) throw new ApiError(res.status, data?.error ?? `Upload failed (${res.status}).`);
	return data.url;
}

// Sign-in is a full-page redirect through Hack Club Auth.
export function signIn(next = '/home') {
	location.href = '/auth/login?next=' + encodeURIComponent(next);
}

export async function signOut() {
	await fetch('/auth/logout', { method: 'POST' });
	location.href = '/';
}

// The backend only serves /dev when it runs with DEV_MODE=1. Without a name,
// /dev/login answers 400 in dev mode and 404 otherwise.
let devMode;
export async function isDevMode() {
	devMode ??= fetch('/dev/login', { redirect: 'manual' }).then((r) => r.status === 400).catch(() => false);
	return devMode;
}

export function devLoginUrl(name, role, next = '/home') {
	return `/dev/login?name=${encodeURIComponent(name)}&role=${role}&next=${encodeURIComponent(next)}`;
}

export async function devHackatime(add) {
	if (add) forget();
	const res = await fetch('/dev/hackatime/mine', {
		method: add ? 'POST' : 'GET',
		headers: add ? { 'Content-Type': 'application/json' } : undefined,
		body: add ? JSON.stringify(add) : undefined
	});
	const data = await res.json().catch(() => null);
	if (!res.ok) throw new ApiError(res.status, data?.error ?? 'Dev Hackatime failed.');
	return data;
}

// Dev mode: give the signed-in person a box to open (POST /dev/box). With
// { project: true } it comes with their newest project and takes over Home.
export async function devGiveBox(body) {
	forget();
	const res = await fetch('/dev/box', {
		method: 'POST',
		headers: body ? { 'Content-Type': 'application/json' } : undefined,
		body: body ? JSON.stringify(body) : undefined
	});
	const data = await res.json().catch(() => null);
	if (!res.ok) throw new ApiError(res.status, data?.error ?? `Couldn't add a box (${res.status}).`);
	return data;
}

// Dev mode: give the signed-in person a stamp the way an admin does (POST
// /dev/stamp). It takes over Home until they've seen it.
export async function devGiveStamp() {
	forget();
	const res = await fetch('/dev/stamp', { method: 'POST' });
	const data = await res.json().catch(() => null);
	if (!res.ok) throw new ApiError(res.status, data?.error ?? `Couldn't add a stamp (${res.status}).`);
	return data;
}
