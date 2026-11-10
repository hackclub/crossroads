// Daily totals for the review page's charts: every UTC day from the start of
// the ship's window to the last day with anything, so quiet days show as
// gaps. At most the last 45 days.
const DAY = 86400000;
const MAX_DAYS = 45;

export function dayColumns(days, windowStart) {
	const byDay = new Map((days ?? []).map((d) => [d.day, d]));
	const start = new Date(windowStart);
	let from = Date.UTC(start.getUTCFullYear(), start.getUTCMonth(), start.getUTCDate());
	const last = (days ?? []).reduce((m, d) => Math.max(m, Date.parse(d.day + 'T00:00:00Z')), from);
	from = Math.max(from, last - (MAX_DAYS - 1) * DAY);
	const out = [];
	for (let t = from; t <= last; t += DAY) {
		const day = new Date(t).toISOString().slice(0, 10);
		out.push(byDay.get(day) ?? { day, seconds: 0, commits: 0 });
	}
	return out;
}

// A date in UTC, like the days in the charts: "Nov 6".
export function shortDate(iso) {
	return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' });
}
