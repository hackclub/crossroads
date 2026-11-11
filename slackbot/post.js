// Posts a Block Kit message to Slack as you. Design it in Block Kit Builder
// (https://app.slack.com/block-kit-builder), copy the JSON into a file, and:
//
//   bun post.js <channel ID> <message.json>
//   bun post.js <channel ID> <message.json> --edit <message ts>
//
// The file is the builder's {"blocks": [...]} or just the blocks array. Add
// "text" next to "blocks" to choose the notification text; otherwise it's the
// message's first bit of text. --edit rewrites a message posted before (its
// ts is printed when it's posted), keeping it where it is. bot.js counts the
// clicks on any button with an action_id; see example.json.
import { readFileSync } from 'node:fs';

const args = process.argv.slice(2);
let editTs = null;
const edit = args.indexOf('--edit');
if (edit !== -1) [, editTs] = args.splice(edit, 2);
const [channel, file] = args;
if (!channel || !file || (edit !== -1 && !editTs)) {
	console.error('Usage: bun post.js <channel ID> <message.json> [--edit <message ts>]');
	process.exit(1);
}
const token = process.env.SLACK_USER_TOKEN;
if (!token?.startsWith('xoxp-')) {
	console.error('SLACK_USER_TOKEN has to be your user token (xoxp-...), so the message comes from you');
	process.exit(1);
}

const message = JSON.parse(readFileSync(file, 'utf8'));
const blocks = Array.isArray(message) ? message : message.blocks;
if (!Array.isArray(blocks) || !blocks.length) {
	console.error(`${file} has no blocks`);
	process.exit(1);
}
const text = message.text ?? firstText(blocks);

const res = await fetch(`https://slack.com/api/${editTs ? 'chat.update' : 'chat.postMessage'}`, {
	method: 'POST',
	headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json; charset=utf-8' },
	body: JSON.stringify(editTs ? { channel, ts: editTs, text, blocks } : { channel, text, blocks, unfurl_links: false })
});
const data = await res.json();
if (!data.ok) {
	console.error(`Slack said: ${data.error}${data.response_metadata?.messages ? '\n' + data.response_metadata.messages.join('\n') : ''}`);
	process.exit(1);
}
console.log(`${editTs ? 'Edited' : 'Posted'} in ${data.channel}, ts ${data.ts}`);

// The notification and screen reader text: the first header, section or
// rich text in the message.
function firstText(blocks) {
	for (const block of blocks) {
		if (block.text?.text) return block.text.text;
		if (block.type === 'rich_text') {
			const words = JSON.stringify(block.elements).match(/"text":"((?:[^"\\]|\\.)*)"/g) ?? [];
			const joined = words.map((w) => JSON.parse(w.slice(7))).join('');
			if (joined) return joined;
		}
	}
	return 'New message';
}
