// Periods for the review counts (the review page's Review stats tab and the
// admin page's Admin review stats): Eastern days as YYYY-MM-DD, both ends
// included, like the backend's ?from=&to= (backend/api/reviewstats.go).
export const ZONE = 'America/New_York';

// Today in New York as YYYY-MM-DD, and date arithmetic on such strings.
export const today = () => new Date().toLocaleDateString('en-CA', { timeZone: ZONE });
export const shift = (day, days) => {
	const d = new Date(day + 'T12:00:00Z');
	d.setUTCDate(d.getUTCDate() + days);
	return d.toISOString().slice(0, 10);
};
export const monday = (day) => shift(day, -((new Date(day + 'T12:00:00Z').getUTCDay() + 6) % 7));
const monthStart = (day) => day.slice(0, 8) + '01';

export const PRESETS = [
	['week', 'This week', () => [monday(today()), today()]],
	['last-week', 'Last week', () => [shift(monday(today()), -7), shift(monday(today()), -1)]],
	['month', 'This month', () => [monthStart(today()), today()]],
	['last-month', 'Last month', () => {
		const end = shift(monthStart(today()), -1);
		return [monthStart(end), end];
	}],
	['all', 'All time', () => ['2026-08-01', today()]]
];

// "Mon, Nov 9" for a YYYY-MM-DD day.
export const dayLabel = (day) => new Date(day + 'T12:00:00Z').toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric', timeZone: 'UTC' });

// A wait as "45m", "6h" or "2d 4h"; a dash for none.
export function duration(seconds) {
	if (seconds == null) return '–';
	const m = Math.round(seconds / 60);
	if (m < 60) return `${m}m`;
	const h = Math.round(m / 60);
	if (h < 48) return `${h}h`;
	return `${Math.floor(h / 24)}d ${h % 24}h`;
}
