// The signed-in person, shared by every page. refresh() after anything that
// changes their stamps or role.
import { ApiError, get } from './api.js';

export const session = $state({ me: null, loaded: false });

export async function refresh() {
	try {
		session.me = await get('/me');
	} catch (err) {
		if (!(err instanceof ApiError && err.status === 401)) throw err;
		session.me = null;
	}
	session.loaded = true;
	return session.me;
}

// Superadmins are admins who can also read the audit log.
export const isSuperadmin = () => session.me?.role === 'superadmin';
export const isAdmin = () => session.me?.role === 'admin' || isSuperadmin();
export const isReviewer = () => session.me?.role === 'reviewer' || isAdmin();
