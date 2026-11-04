// Blockout-only view settings: which scenario to preview and whether the
// design notes are visible. Kept in localStorage so they survive reloads.
const KEY = 'crossroads-blockout';

function load() {
	try {
		return { scenario: 'building', notes: true, ...JSON.parse(localStorage.getItem(KEY) || '{}') };
	} catch {
		return { scenario: 'building', notes: true };
	}
}

export const view = $state(load());

export function saveView() {
	try {
		localStorage.setItem(KEY, JSON.stringify(view));
	} catch {
		// Private mode or blocked storage: settings just won't persist.
	}
}
