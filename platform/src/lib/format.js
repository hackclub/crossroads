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

export function money(cents) {
	return cents == null ? '' : '$' + (cents / 100).toFixed(cents % 100 ? 2 : 0);
}

export const GOAL_HOURS = 60;

// The stamps-per-hour rate isn't exposed by the API; INTEGRATING.md says to
// derive it from the ticket, which costs 60 hours' worth.
export const stampsPerHour = (ticket) => (ticket ? ticket.price / GOAL_HOURS : 30);

export const approvedSeconds = (projects) => projects.reduce((t, p) => t + p.approved_seconds, 0);

export const pending = (project) => project.submissions?.[0]?.status === 'pending';

// The status a project shows: its latest submission, or "not shipped".
export function projectStatus(project) {
	const latest = project.submissions?.[0];
	if (!latest) return { label: 'Not shipped', tone: '' };
	return {
		pending: { label: 'In review', tone: 'info' },
		approved: { label: 'Approved', tone: 'good' },
		rejected: { label: 'Rejected', tone: 'warn' }
	}[latest.status];
}

export const RARITIES = ['common', 'uncommon', 'rare', 'epic', 'legendary'];
