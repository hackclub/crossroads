// Time is seconds everywhere in the API; show hours.
export function hours(seconds) {
	if (seconds == null) return '–';
	const h = seconds / 3600;
	return (Number.isInteger(h) ? h : h.toFixed(1)) + 'h';
}

export function date(iso) {
	if (!iso) return '–';
	return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
}

// A UTC day from the API ("2026-11-06") as "Nov 6".
export function utcDay(day) {
	return new Date(day + 'T00:00:00Z').toLocaleDateString(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' });
}

export function money(cents) {
	return cents == null ? '' : '$' + (cents / 100).toFixed(cents % 100 ? 2 : 0);
}

// The ticket's first batch; later batches cost more (the ticket's price in
// GET /shop/items says what it costs now).
export const GOAL_HOURS = 60;

// Stamps as approved hours at rate (session.me.stamps_per_hour): "65h".
export function stampHours(stamps, rate) {
	const h = Math.round((stamps / rate) * 10) / 10;
	return h + 'h';
}

export const approvedSeconds = (projects) => projects.reduce((t, p) => t + p.approved_seconds, 0);

// A ship still in review: waiting for a reviewer, or approved by one and
// waiting for its final check (final_check) before stamps are paid.
export const inReview = (status) => status === 'pending' || status === 'final_check';

export const pending = (project) => inReview(project.submissions?.[0]?.status);

// The status a project shows: its latest submission, or "not shipped".
export function projectStatus(project) {
	const latest = project.submissions?.[0];
	if (!latest) return { label: 'Not shipped', tone: '' };
	return {
		pending: { label: 'In review', tone: 'info' },
		final_check: { label: 'Approved, final check', tone: 'info' },
		approved: { label: 'Approved', tone: 'good' },
		rejected: { label: 'Rejected', tone: 'warn' }
	}[latest.status];
}

export const RARITIES = ['common', 'uncommon', 'rare', 'epic', 'legendary'];

// What a special stamp does once it's stuck on a project, in a few words and
// in a sentence.
export const EFFECTS = {
	none: { short: 'Collectible', long: "It's a collectible. It shows on your project for anyone who views it." },
	priority_review: { short: 'Priority review', long: 'The ship of this project in review gets priority review. If nothing is in review, the next ship does.' },
	stamp_multiplier: { short: '+10% stamps', long: 'Every ship of this project approved from now on earns 10% more stamps.' }
};
export const effectShort = (effect) => EFFECTS[effect]?.short ?? effect;
