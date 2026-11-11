// Hears every click on the buttons of messages posted with post.js and
// records it on the Crossroads backend (POST /slack/clicks) by Slack ID. Both
// buttons are links, so Slack opens them by itself; this only counts.
//
// It holds just the app-level token, which can do nothing but receive events.
// Run it with: SLACK_APP_TOKEN=xapp-... BACKEND_URL=... SLACK_CLICKS_SECRET=... bun bot.js
import { SocketModeClient } from '@slack/socket-mode';

const { SLACK_APP_TOKEN, BACKEND_URL, SLACK_CLICKS_SECRET } = process.env;
if (!SLACK_APP_TOKEN || !BACKEND_URL || !SLACK_CLICKS_SECRET) {
	console.error('SLACK_APP_TOKEN, BACKEND_URL and SLACK_CLICKS_SECRET have to be set');
	process.exit(1);
}

const client = new SocketModeClient({ appToken: SLACK_APP_TOKEN });

client.on('interactive', async ({ ack, body }) => {
	// Slack puts a warning on the button unless it's acked within 3 seconds.
	await ack();
	if (body.type !== 'block_actions') return;
	for (const action of body.actions) {
		await record({
			slack_id: body.user.id,
			button: action.action_id,
			channel_id: body.channel?.id ?? null,
			message_ts: body.container?.message_ts ?? null
		});
	}
});

// A click that can't be recorded is logged and lost; the link still opened.
async function record(click) {
	try {
		const res = await fetch(new URL('/slack/clicks', BACKEND_URL), {
			method: 'POST',
			headers: { Authorization: `Bearer ${SLACK_CLICKS_SECRET}`, 'Content-Type': 'application/json' },
			body: JSON.stringify(click)
		});
		if (!res.ok) console.error(`recording ${click.button} by ${click.slack_id}: ${res.status} ${await res.text()}`);
	} catch (err) {
		console.error(`recording ${click.button} by ${click.slack_id}: ${err}`);
	}
}

await client.start();
console.log('Listening for clicks');
