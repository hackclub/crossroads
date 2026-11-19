// What an item really costs in each region, for the admin page's Shop items
// (RegionalPrices.svelte). It's research for deciding prices and where items
// can go; the shop charges everyone the same. The delivered costs are in
// regional-prices.json; hours come from the item's own ticket price, so they
// stay right when prices change. Card grants and stipends are worth the same
// everywhere, so their value comes from the item itself.
import data from './regional-prices.json';

export const REGIONS = data.regions;

// "Blåhaj" and "BLAHAJ" are the same item; so are "RTL-SDR Blog v3" and "V3".
const key = (name) =>
	name
		.normalize('NFD')
		.replace(/[̀-ͯ]/g, '')
		.toLowerCase()
		.replace(/[^a-z0-9]/g, '');

const byName = new Map();
for (const entry of data.items) {
	for (const name of [entry.name, ...entry.aliases]) byName.set(key(name), entry);
}

// Everything the dropdown shows about item, or null for things that aren't
// bought anywhere (the rail pass, a price lock).
export function regionalPrices(item, ticketsPerHour) {
	if (item.kind === 'ticket' || item.kind === 'price_lock') return null;
	const hours = item.price / ticketsPerHour;
	const grant = item.stipend_cents ?? item.grant_cents;
	const entry = byName.get(key(item.name));

	let rows = [];
	if (entry) {
		rows = Object.entries(REGIONS).map(([code, label]) => ({ code, label, ...entry.regions[code] }));
	} else if (grant != null) {
		return { hours, rows, entry: null, grant, perHour: grant / 100 / hours };
	}
	for (const r of rows) r.perHour = r.total_usd == null ? null : r.total_usd / hours;
	const priced = rows.filter((r) => r.total_usd != null);
	const cheapest = priced.length > 1 ? priced.reduce((a, b) => (b.total_usd < a.total_usd ? b : a)) : null;
	const priciest = priced.length > 1 ? priced.reduce((a, b) => (b.total_usd > a.total_usd ? b : a)) : null;
	return { hours, rows, entry, grant: null, cheapest, priciest, missing: rows.length - priced.length };
}
