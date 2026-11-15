// Turns the Announce tab's editor content into a Slack message: Block Kit
// blocks and the notification text. The preview shows exactly these blocks,
// and the backend posts them as they are (see backend/api/announcements.go).
//
// The editor's content (the announcement's `doc`) is a list of blocks:
//   { type: 'text', content }       rich text, as Tiptap JSON
//   { type: 'heading', text }        a header block (plain text, 150 characters)
//   { type: 'image', url, alt, title }
//   { type: 'divider' }
//   { type: 'buttons', buttons }     each { id, label, style, target, page, stampTypeId, url }
// plus pingChannel (an @channel at the top) and notification (the
// notification text; empty works it out from the first words).
//
// A button's id is its action_id, which its clicks are counted by, so it
// never changes once made.

export const newId = (prefix) => prefix + '_' + crypto.randomUUID().replaceAll('-', '').slice(0, 10);

export const emptyDoc = () => ({ version: 1, pingChannel: false, notification: '', blocks: [newBlock('text')] });

export function newBlock(type) {
	const block = { id: newId('blk'), type };
	if (type === 'text') block.content = { type: 'doc', content: [{ type: 'paragraph' }] };
	if (type === 'heading') block.text = '';
	if (type === 'image') Object.assign(block, { url: '', alt: '', title: '' });
	if (type === 'buttons') block.buttons = [newButton()];
	return block;
}

export const newButton = () => ({ id: newId('btn'), label: '', style: '', target: 'page', page: '/home', stampTypeId: null, url: '' });

// Where a button can go on the platform.
export const PAGES = [
	['/home', 'Home'],
	['/shop', 'Shop'],
	['/projects/new', 'New project'],
	['/boxes', 'Boxes'],
	['/guides', 'Guides'],
	['/emporium', 'Emporium'],
	['/', 'Front page']
];

// Slack's limits.
const MAX_BLOCKS = 50;
const MAX_HEADER = 150;
const MAX_LABEL = 75;
const MAX_BUTTONS = 25;

const STYLES = { bold: 'bold', italic: 'italic', underline: 'underline', strike: 'strike', code: 'code' };
// :name: with at least one letter, so 12:30:45 stays text.
const EMOJI = /:([a-z0-9_+'-]*[a-z][a-z0-9_+'-]*):/g;

// toSlack returns { blocks, text, problems }. Blocks leave out anything not
// finished (an image with no link, a button with no label), so a draft can
// always be saved; problems says what, and posting waits until there are
// none. stampTypes are GET /admin/special-stamp-types, for claim links.
export function toSlack(doc, { publicUrl, stampTypes = [] }) {
	const blocks = [];
	const problems = [];
	for (const [i, block] of doc.blocks.entries()) {
		const where = `Block ${i + 1}`;
		if (block.type === 'text') {
			const elements = richText(block.content);
			if (elements.length) blocks.push({ type: 'rich_text', elements });
		} else if (block.type === 'heading') {
			const text = block.text.trim();
			if (!text) problems.push(`${where}: the heading is empty.`);
			else if (text.length > MAX_HEADER) problems.push(`${where}: headings stop at ${MAX_HEADER} characters.`);
			else blocks.push({ type: 'header', text: { type: 'plain_text', text, emoji: true } });
		} else if (block.type === 'image') {
			const url = block.url.trim();
			if (!isWebUrl(url)) problems.push(`${where}: the image needs a picture.`);
			else
				blocks.push({
					type: 'image',
					image_url: url,
					alt_text: block.alt.trim() || block.title.trim() || 'Image',
					...(block.title.trim() && { title: { type: 'plain_text', text: block.title.trim(), emoji: true } })
				});
		} else if (block.type === 'divider') {
			blocks.push({ type: 'divider' });
		} else if (block.type === 'buttons') {
			const elements = [];
			for (const [j, button] of block.buttons.entries()) {
				const name = `${where}, button ${j + 1}`;
				const url = buttonUrl(button, { publicUrl, stampTypes });
				const label = button.label.trim();
				if (!label) problems.push(`${name} needs a label.`);
				else if (label.length > MAX_LABEL) problems.push(`${name}: labels stop at ${MAX_LABEL} characters.`);
				else if (url.problem) problems.push(`${name}: ${url.problem}`);
				else
					elements.push({
						type: 'button',
						action_id: button.id,
						text: { type: 'plain_text', text: label, emoji: true },
						url: url.url,
						...(button.style && { style: button.style })
					});
			}
			if (block.buttons.length > MAX_BUTTONS) problems.push(`${where}: a row holds up to ${MAX_BUTTONS} buttons.`);
			if (elements.length) blocks.push({ type: 'actions', elements: elements.slice(0, MAX_BUTTONS) });
		}
	}

	if (doc.pingChannel) {
		const broadcast = { type: 'broadcast', range: 'channel' };
		const first = blocks[0]?.type === 'rich_text' && blocks[0].elements[0];
		if (first?.type === 'rich_text_section') first.elements.unshift(broadcast, { type: 'text', text: ' ' });
		else blocks.unshift({ type: 'rich_text', elements: [{ type: 'rich_text_section', elements: [broadcast] }] });
	}
	if (!blocks.length) problems.push('The message is empty.');
	if (blocks.length > MAX_BLOCKS) problems.push(`Slack takes up to ${MAX_BLOCKS} blocks, and this is ${blocks.length}.`);

	const words = doc.notification.trim() || firstWords(doc) || 'New announcement';
	const text = (doc.pingChannel ? '<!channel> ' : '') + escape(words);
	return { blocks, text, problems };
}

// buttonUrl returns { url } or { problem }.
export function buttonUrl(button, { publicUrl, stampTypes }) {
	if (button.target === 'page') {
		const path = button.page.trim();
		if (!path.startsWith('/')) return { problem: 'the page has to start with /.' };
		return { url: publicUrl + path };
	}
	if (button.target === 'stamp') {
		const type = stampTypes.find((t) => t.id === button.stampTypeId);
		if (!type) return { problem: 'pick the stamp it gives.' };
		if (!type.claim_code) return { problem: `${type.name} has no claim code. Give it one under Special stamp types.` };
		return { url: `${publicUrl}/claim/${type.claim_code}` };
	}
	const url = button.url.trim();
	if (!isWebUrl(url)) return { problem: 'the link has to start with https:// or http://.' };
	return { url };
}

function isWebUrl(s) {
	try {
		const u = new URL(s);
		return (u.protocol === 'https:' || u.protocol === 'http:') && s.length <= 2048;
	} catch {
		return false;
	}
}

// richText turns Tiptap JSON into a rich_text block's elements. Paragraphs
// are lines, as they are in Slack, so consecutive ones share a section.
function richText(content) {
	const elements = [];
	let lines = null;
	const flush = () => {
		const section = lines && trimEnd(lines);
		if (section?.length) elements.push({ type: 'rich_text_section', elements: section });
		lines = null;
	};
	for (const node of content?.content ?? []) {
		if (node.type === 'paragraph') {
			if (lines) pushText(lines, '\n');
			else lines = [];
			lines.push(...inline(node.content));
		} else if (node.type === 'bulletList' || node.type === 'orderedList') {
			flush();
			list(node, 0, elements);
		} else if (node.type === 'blockquote') {
			flush();
			const quoted = joinLines(node.content);
			if (quoted.length) elements.push({ type: 'rich_text_quote', elements: quoted });
		} else if (node.type === 'codeBlock') {
			flush();
			const code = textOf(node);
			if (code) elements.push({ type: 'rich_text_preformatted', elements: [{ type: 'text', text: code }] });
		}
	}
	flush();
	return elements;
}

// Slack has no nested lists; it has lists with an indent, one after another.
// An ordered list that picks up after a nested one says where with offset.
function list(node, indent, out) {
	const style = node.type === 'orderedList' ? 'ordered' : 'bullet';
	let current = null;
	let count = (node.attrs?.start ?? 1) - 1;
	for (const item of node.content ?? []) {
		const own = joinLines(item.content?.filter((c) => c.type === 'paragraph'));
		if (own.length) {
			current ??= { type: 'rich_text_list', style, indent: Math.min(indent, 8), elements: [], ...(style === 'ordered' && count && { offset: count }) };
			current.elements.push({ type: 'rich_text_section', elements: own });
			count++;
		}
		const nested = item.content?.filter((c) => c.type === 'bulletList' || c.type === 'orderedList') ?? [];
		if (nested.length) {
			if (current) out.push(current);
			current = null;
			for (const n of nested) list(n, indent + 1, out);
		}
	}
	if (current) out.push(current);
}

// The inline elements of paragraphs, a line each. Anything else in them
// (a list in a quote) comes along as plain text.
function joinLines(nodes = []) {
	const out = [];
	for (const [i, node] of nodes.entries()) {
		if (i) pushText(out, '\n');
		out.push(...(node.type === 'paragraph' ? inline(node.content) : textElements(textOf(node))));
	}
	return trimEnd(out);
}

function inline(nodes = []) {
	const out = [];
	for (const node of nodes) {
		if (node.type === 'hardBreak') pushText(out, '\n');
		if (node.type !== 'text' || !node.text) continue;
		const style = {};
		let href = null;
		for (const mark of node.marks ?? []) {
			if (mark.type === 'link') href = mark.attrs?.href;
			else if (STYLES[mark.type]) style[STYLES[mark.type]] = true;
		}
		const styled = Object.keys(style).length ? { style } : {};
		if (href) out.push({ type: 'link', url: href, text: node.text, ...styled });
		else if (style.code) out.push({ type: 'text', text: node.text, ...styled });
		else out.push(...textElements(node.text, styled));
	}
	return out;
}

// Text, with :emoji: as emoji elements: Slack shows rich text as typed.
function textElements(text, styled = {}) {
	const out = [];
	let at = 0;
	for (const m of text.matchAll(EMOJI)) {
		if (m.index > at) out.push({ type: 'text', text: text.slice(at, m.index), ...styled });
		out.push({ type: 'emoji', name: m[1] });
		at = m.index + m[0].length;
	}
	if (at < text.length) out.push({ type: 'text', text: text.slice(at), ...styled });
	return out;
}

// Adds plain text, joining it to plain text before it.
function pushText(out, text) {
	const last = out.at(-1);
	if (last?.type === 'text' && !last.style) last.text += text;
	else out.push({ type: 'text', text });
}

// Drops trailing blank lines, which Slack shows as space.
function trimEnd(elements) {
	const out = elements.map((e) => ({ ...e }));
	while (out.length) {
		const last = out.at(-1);
		if (last.type !== 'text' || last.style) break;
		last.text = last.text.replace(/\n+$/, '');
		if (last.text) break;
		out.pop();
	}
	return out;
}

function textOf(node) {
	if (node.type === 'text') return node.text ?? '';
	if (node.type === 'hardBreak') return '\n';
	const sep = node.type === 'doc' || node.type === 'blockquote' || node.type.endsWith('List') || node.type === 'listItem' ? '\n' : '';
	return (node.content ?? []).map(textOf).join(sep);
}

// The notification: the first text or heading, up to 200 characters.
function firstWords(doc) {
	for (const block of doc.blocks) {
		const text = (block.type === 'text' ? textOf(block.content) : block.type === 'heading' ? block.text : '').replace(/\s+/g, ' ').trim();
		if (text) return text.length > 200 ? text.slice(0, 199) + '…' : text;
	}
	return '';
}

// Slack reads the notification text as mrkdwn.
const escape = (s) => s.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
