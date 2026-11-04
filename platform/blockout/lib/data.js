// Mock data for the blockout. Everything here is placeholder content; swap it for
// real API calls when the platform is built. Pick a scenario in the blockout bar
// to see the pages at different stages of a participant's journey.

const SHIP_DEADLINE = new Date('2026-12-15');

export const PROGRAM = {
	goalHours: 60,
	minShipHours: 3,
	spots: 30,
	tripDates: 'January 8–12',
	tripStart: 'Paris',
	shipDeadline: 'Dec 15',
	daysToDeadline: Math.max(Math.ceil((SHIP_DEADLINE - new Date()) / 86_400_000), 0),
	currency: 'tokens',
	tokensPerHour: 10
};

export const SCENARIOS = {
	new: 'Just signed up',
	building: 'Building (22h approved)',
	review: 'Waiting on a review',
	qualified: 'Qualified for the trip'
};

export const PROJECT_TYPES = ['Web app', 'Mobile app', 'Desktop app', 'Game', 'CLI tool', 'Bot', 'Hardware', 'Other'];

// Used for both projects and individual ships. `tone` is the badge colour.
export const STATUS = {
	building: { label: 'Not shipped', tone: 'muted' },
	in_review: { label: 'In review', tone: 'info' },
	changes_needed: { label: 'Changes needed', tone: 'warn' },
	approved: { label: 'Shipped', tone: 'good' },
	rejected: { label: 'Rejected', tone: 'danger' }
};

export const ORDER_STATUS = {
	pending: 'Pending',
	approved: 'Approved',
	fulfilled: 'Shipped',
	rejected: 'Rejected'
};

export const shopItems = [
	{ id: 'stickers', name: 'Sticker pack', category: 'Swag', cost: 30, stock: null },
	{ id: 'tshirt', name: 'Crossroads T-shirt', category: 'Swag', cost: 120, stock: 40, note: 'Pick your size in the order note.' },
	{ id: 'domain', name: 'Domain name (1 year)', category: 'Software', cost: 100, stock: null },
	{ id: 'jetbrains', name: 'JetBrains licence', category: 'Software', cost: 150, stock: null },
	{ id: 'pico', name: 'Raspberry Pi Pico kit', category: 'Hardware', cost: 90, stock: 25 },
	{ id: 'keyboard', name: 'Mechanical keyboard', category: 'Hardware', cost: 450, stock: 8 },
	{ id: 'grant', name: 'Project grant ($50)', category: 'Grants', cost: 400, stock: null, note: 'Paid as a card for parts or hosting for a shipped project.' },
	{ id: 'rail-pass', name: 'Extra day on the trip', category: 'Trip', cost: 300, stock: 10, tripOnly: true }
];

// Hackatime projects the signed-in user has tracked time on.
export const hackatimeProjects = [
	{ name: 'metro-map', hours: 14.2, linkedTo: 'metro-map' },
	{ name: 'metro-map-api', hours: 4.1, linkedTo: 'metro-map' },
	{ name: 'pixel-garden', hours: 11.5, linkedTo: 'pixel-garden' },
	{ name: 'study-bot', hours: 6.3, linkedTo: 'study-bot' },
	{ name: 'dotfiles', hours: 2.4, linkedTo: null },
	{ name: 'scratch', hours: 0.8, linkedTo: null }
];

const people = [
	{ username: 'ada', name: 'Ada', country: 'UK', approvedHours: 64, ships: 5 },
	{ username: 'lin', name: 'Lin', country: 'Singapore', approvedHours: 61, ships: 4 },
	{ username: 'kofi', name: 'Kofi', country: 'Ghana', approvedHours: 47, ships: 3 },
	{ username: 'maria', name: 'María', country: 'Spain', approvedHours: 33, ships: 3 },
	{ username: 'sam', name: 'Sam', country: 'USA', approvedHours: 18, ships: 1 },
	{ username: 'noor', name: 'Noor', country: 'Jordan', approvedHours: 9, ships: 1 }
];

const others = [
	other('rail-planner', 'ada', 'Rail Planner', 'Web app', 'Plans multi-city train trips across Europe and shows the cheapest route by day.', 22, 41),
	other('pocket-synth', 'kofi', 'Pocket Synth', 'Hardware', 'A four-voice synth on a Pico with a 3D-printed case and capacitive keys.', 19, 33),
	other('tiny-tower', 'lin', 'Tiny Tower', 'Game', 'Stack falling blocks to build the tallest tower you can. Made in Godot.', 15, 27),
	other('commit-haiku', 'maria', 'Commit Haiku', 'CLI tool', 'Turns your git log into haiku. Surprisingly good at it.', 8, 19),
	other('shelfie', 'sam', 'Shelfie', 'Mobile app', 'Scan the barcode on a book and add it to a shared shelf with friends.', 12, 8),
	other('lunch-bot', 'noor', 'Lunch Bot', 'Bot', 'A Slack bot that tells your school what is for lunch every morning.', 9, 5)
];

function other(id, owner, title, type, description, approvedHours, likes) {
	return {
		id, owner, title, type, description, approvedHours, likes,
		status: 'approved',
		trackedHours: approvedHours + 2,
		codeUrl: `https://github.com/${owner}/${id}`,
		demoUrl: `https://${id}.example.com`,
		readmeUrl: `https://github.com/${owner}/${id}/blob/main/README.md`,
		hackatime: [{ name: id, hours: approvedHours + 2 }],
		ships: [{ n: 1, status: 'approved', submitted: 'Oct 20', approvedHours }],
		devlogs: [{ date: 'Oct 18', hours: 4, text: 'Got the core working end to end. Next up: polish and a proper README.', images: 2 }]
	};
}

const me = {
	username: 'jordan',
	name: 'Jordan',
	country: 'Germany',
	slack: true,
	hackatime: true,
	github: true,
	verified: true,
	address: true,
	birthday: true,
	profileComplete: true,
	streak: 4,
	hackatimeTodayMin: 45
};

const repo = (id) => ({
	codeUrl: `https://github.com/jordan-codes/${id}`,
	readmeUrl: `https://github.com/jordan-codes/${id}/blob/main/README.md`
});

// My projects, per scenario.
function myProjects(scenario) {
	if (scenario === 'new') return [];

	const metro = {
		id: 'metro-map', owner: 'jordan', title: 'Metro Map Maker', type: 'Web app', ...repo('metro-map'),
		description: 'Draw your own transit maps in the browser and export them as SVG. Snaps lines to 45° like real metro maps.',
		demoUrl: 'https://metro-map.example.com', hasBanner: true, likes: 12,
		aiUse: 'Copilot autocomplete, and asked ChatGPT to explain an SVG path bug.',
		hackatime: [{ name: 'metro-map', hours: 14.2 }, { name: 'metro-map-api', hours: 4.1 }],
		trackedHours: 18.3, approvedHours: 14, status: 'approved',
		ships: [{ n: 1, status: 'approved', submitted: 'Oct 12', approvedHours: 14, feedback: 'Great demo and a clear README. Removed some idle time.' }],
		devlogs: [
			{ date: 'Oct 28', hours: 4.3, text: 'Added an API so maps can be saved and shared by link.', images: 1 },
			{ date: 'Oct 10', hours: 6, text: 'Line snapping works! Export to SVG is next.', images: 2 }
		]
	};
	const garden = {
		id: 'pixel-garden', owner: 'jordan', title: 'Pixel Garden', type: 'Game', ...repo('pixel-garden'),
		description: 'A cosy idle game where you grow pixel-art plants. Plants keep growing while you code.',
		demoUrl: 'https://jordan.itch.io/pixel-garden', hasBanner: true, likes: 4,
		hackatime: [{ name: 'pixel-garden', hours: 11.5 }],
		trackedHours: 11.5, approvedHours: 8, status: 'changes_needed',
		ships: [
			{ n: 2, status: 'changes_needed', submitted: 'Oct 30', feedback: "The web build doesn't load in Firefox. Fix it and ship again." },
			{ n: 1, status: 'approved', submitted: 'Oct 5', approvedHours: 8 }
		],
		devlogs: [{ date: 'Oct 29', hours: 3.5, text: 'Added watering and three new plants.', images: 2 }]
	};
	const bot = {
		id: 'study-bot', owner: 'jordan', title: 'Study Bot', type: 'Bot', codeUrl: 'https://github.com/jordan-codes/study-bot',
		description: 'A Discord bot that runs pomodoro timers for study groups.',
		hasBanner: false,
		hackatime: [{ name: 'study-bot', hours: 6.3 }],
		trackedHours: 6.3, approvedHours: 0, status: 'building',
		ships: [],
		devlogs: []
	};

	if (scenario === 'review') {
		return [
			metro,
			{ ...garden, status: 'in_review', ships: [{ n: 2, status: 'in_review', submitted: 'Nov 2' }, garden.ships[1]] },
			bot
		];
	}
	if (scenario === 'qualified') {
		return [
			{ ...metro, approvedHours: 18, ships: [{ n: 2, status: 'approved', submitted: 'Nov 1', approvedHours: 4 }, ...metro.ships] },
			{ ...garden, approvedHours: 11, status: 'approved', ships: [{ n: 2, status: 'approved', submitted: 'Nov 2', approvedHours: 3 }, garden.ships[1]] },
			{ ...bot, approvedHours: 6, status: 'approved', hasBanner: true, demoUrl: 'https://discord.com/invite/example', readmeUrl: 'https://github.com/jordan-codes/study-bot/blob/main/README.md', ships: [{ n: 1, status: 'approved', submitted: 'Oct 25', approvedHours: 6 }] },
			{
				id: 'trainspotter', owner: 'jordan', title: 'Trainspotter', type: 'Mobile app', ...repo('trainspotter'),
				description: 'Log every train you ride and see your journeys on a map.', demoUrl: 'https://expo.dev/@jordan/trainspotter', hasBanner: true, likes: 9,
				hackatime: [{ name: 'trainspotter', hours: 27 }], trackedHours: 27, approvedHours: 26, status: 'approved',
				ships: [{ n: 1, status: 'approved', submitted: 'Oct 20', approvedHours: 26 }], devlogs: []
			}
		];
	}
	return [metro, garden, bot];
}

export function getData(scenario = 'building') {
	const projects = myProjects(scenario);
	const meNow = scenario === 'new'
		? { ...me, hackatime: false, github: false, verified: false, address: false, profileComplete: false, streak: 0, hackatimeTodayMin: 0 }
		: me;

	const approved = projects.reduce((t, p) => t + p.approvedHours, 0);
	const pending = (p) => Math.max(p.trackedHours - p.approvedHours, 0);
	const hours = {
		approved,
		review: round(projects.filter((p) => p.status === 'in_review').reduce((t, p) => t + pending(p), 0)),
		unshipped: round(projects.filter((p) => p.status !== 'in_review').reduce((t, p) => t + pending(p), 0))
	};

	const orders = scenario === 'new' ? [] : [
		{ id: 1042, item: 'Sticker pack', cost: 30, date: 'Oct 14', status: 'fulfilled' },
		{ id: 1088, item: 'Domain name (1 year)', cost: 100, date: 'Nov 1', status: 'pending' }
	].filter((o) => scenario === 'qualified' || o.id === 1042);

	const earned = approved * PROGRAM.tokensPerHour;
	const spent = orders.filter((o) => o.status !== 'rejected').reduce((t, o) => t + o.cost, 0);

	const notifications = {
		new: [],
		building: [
			{ date: 'Oct 31', text: 'Pixel Garden needs changes before it can be approved.', href: '/projects/pixel-garden' },
			{ date: 'Oct 15', text: 'Your sticker pack has shipped.', href: '/shop/orders' }
		],
		review: [{ date: 'Nov 2', text: 'Pixel Garden is in the review queue.', href: '/projects/pixel-garden' }],
		qualified: [
			{ date: 'Nov 3', text: "You've qualified for the trip! Confirm your spot.", href: '/trip' },
			{ date: 'Nov 2', text: 'Pixel Garden ship #2 was approved: +3h.', href: '/projects/pixel-garden' }
		]
	}[scenario] ?? [];

	const week = {
		new: [0, 0, 0, 0, 0, 0, 0],
		building: [35, 80, 0, 120, 45, 0, 0],
		review: [60, 20, 95, 30, 0, 0, 0],
		qualified: [90, 140, 60, 180, 75, 30, 0]
	}[scenario] ?? [0, 0, 0, 0, 0, 0, 0];

	return {
		me: meNow,
		projects,
		allProjects: [...projects, ...others],
		people,
		hours,
		qualified: approved >= PROGRAM.goalHours,
		earned,
		balance: earned - spent,
		orders,
		notifications,
		week,
		announcements: [
			{ title: 'Ship deadline is Dec 15', date: 'Nov 1', text: 'Ship by then so there is time to review it.' },
			{ title: 'Hardware guide is up', date: 'Oct 24', text: 'How to ship a hardware project with a video demo.' }
		]
	};
}

export function findProject(d, id) {
	return d.allProjects.find((p) => p.id === id);
}

export function personByUsername(d, username) {
	if (username === d.me.username) {
		return {
			...d.me,
			approvedHours: d.hours.approved,
			ships: d.projects.reduce((n, p) => n + p.ships.length, 0),
			you: true
		};
	}
	return d.people.find((p) => p.username === username);
}

// Why a project can't be shipped yet. An empty list means it's ready.
export function shipBlockers(p, me) {
	const out = [];
	if (p.status === 'in_review') out.push('It is already in review.');
	if (!p.codeUrl) out.push('Add a link to the code repository.');
	if (!p.demoUrl) out.push('Add a demo people can try.');
	if (!p.readmeUrl) out.push('Add a README link.');
	if (!p.hasBanner) out.push('Add a screenshot or banner.');
	if ((p.description?.length ?? 0) < 30) out.push('Write a description of at least 30 characters.');
	if (!p.hackatime?.length) out.push('Link at least one Hackatime project.');
	const newHours = (p.trackedHours ?? 0) - p.approvedHours;
	if (newHours < PROGRAM.minShipHours)
		out.push(`Track at least ${PROGRAM.minShipHours}h since your last ship (you have ${round(Math.max(newHours, 0))}h).`);
	if (!me.profileComplete) out.push('Verify your ID and add your address in settings.');
	return out;
}

function round(n) {
	return Math.round(n * 10) / 10;
}
