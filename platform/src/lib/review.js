// Shared by the review pages. The rules themselves live in the backend
// (backend/api/stages.go); this only names and explains them.

// The three queues, keyed by the API's stage names.
export const STAGES = {
	review: {
		label: 'Review',
		status: 'pending',
		perm: 'can_review',
		blurb: 'New ships. Try the project and decide how many of its hours to approve.'
	},
	fraud: {
		label: 'Fraud check',
		status: 'fraud_check',
		perm: 'can_fraud_review',
		blurb: 'Approved ships the automatic checks flagged. Pass them, cut hours, or reject them.'
	},
	hq: {
		label: 'HQ sign-off',
		status: 'awaiting_hq',
		perm: 'can_hq_review',
		blurb: 'The last step. Signing off pays the stamps and sends the ship to the Unified YSWS DB.'
	}
};

export const canDecideStage = (me, stage) => !!me?.[STAGES[stage]?.perm];

// Why a ship went to the fraud check (submission.fraud_reasons).
const REASONS = {
	hackatime_flagged: 'Hackatime flag',
	hackatime_dropped: 'Hackatime time went down',
	shared_links: 'Shared repo or demo',
	large_ship: 'Big ship',
	long_day: 'Very long day',
	unified_double_dip: 'Paid by another program',
	manual: 'Reviewer asked',
	everything: 'All ships'
};
export const reasonLabel = (id) => REASONS[id] ?? id;

// Ship statuses as reviewers see them.
export const STATUS_LABELS = {
	pending: 'Waiting for review',
	fraud_check: 'Fraud check',
	awaiting_hq: 'Waiting for HQ',
	approved: 'Approved',
	rejected: 'Rejected'
};
export const statusTone = (status) =>
	({ pending: 'info', fraud_check: 'warn', awaiting_hq: 'info', approved: 'good', rejected: 'warn' })[status] ?? '';

// What each timeline event says, for the timeline on the review page.
export const EVENT_LABELS = {
	comment: 'commented',
	internal_comment: 'left an internal note',
	flagged: 'Sent to the fraud check',
	approval: 'approved',
	fraud_pass: 'passed the fraud check',
	sent_back: 'sent it back to review',
	final_approval: 'signed off',
	rejection: 'rejected it'
};

// "14:05" in the viewer's time.
export const time = (iso) => new Date(iso).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });

// Hours typed by a reviewer → whole seconds, capped at max.
export const toSeconds = (h, max) => Math.max(0, Math.min(Math.round(Number(h) * 3600), max));
export const toHours = (s) => +(s / 3600).toFixed(2);
