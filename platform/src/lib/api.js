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
	const res = await fetch('/api' + path, {
		method,
		headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
		body: body === undefined ? undefined : JSON.stringify(body)
	});
	if (res.status === 204) return undefined;
	const data = await res.json().catch(() => null);
	if (!res.ok) throw new ApiError(res.status, data?.error ?? `Something went wrong (${res.status}).`);
	return data;
}

export const get = (path) => api('GET', path);

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
	const res = await fetch('/dev/hackatime/mine', {
		method: add ? 'POST' : 'GET',
		headers: add ? { 'Content-Type': 'application/json' } : undefined,
		body: add ? JSON.stringify(add) : undefined
	});
	const data = await res.json().catch(() => null);
	if (!res.ok) throw new ApiError(res.status, data?.error ?? 'Dev Hackatime failed.');
	return data;
}
