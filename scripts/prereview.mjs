#!/usr/bin/env node
// Pre-reviews from a laptop with Claude Code signed in, like Ari's Jame
// Gam script: no server runs Claude. For each ship, it clones the repo, runs
// `claude -p` in it with the site's instructions and the ship's details, and
// collects the answers. Plain Node (18+), no packages.
//
// Two ways (see backend/api/prereview.go and the review queue's
// Pre-reviews tab):
//
//   Files, no token needed:
//     1. Review > Pre-reviews > "Download the ships" (crossroads-prereviews.json)
//     2. node scripts/prereview.mjs --from crossroads-prereviews.json --out answer.json
//     3. Choose answer.json under "Paste Claude's answer" and save.
//
//   Straight to the site, when it has PREREVIEW_TOKEN set:
//     PREREVIEW_TOKEN=… node scripts/prereview.mjs --base https://crossroads.hackclub.com
//
// Flags: --limit N (ships to ask the site for, default 10, at most 25),
//        --all (redo ships that already have one), --only <ship id>,
//        --model sonnet|opus (default sonnet), --concurrency N (default 2),
//        --dry (print the answers, don't upload or write them).
//
// It uses your Claude subscription: ANTHROPIC_API_KEY is dropped for claude.
import { spawn } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const args = process.argv.slice(2);
const flag = (name) => args.includes(name);
const opt = (name) => {
	const i = args.indexOf(name);
	return i >= 0 ? args[i + 1] : undefined;
};
const BASE = (opt('--base') ?? process.env.CROSSROADS_URL ?? '').replace(/\/$/, '');
const TOKEN = process.env.PREREVIEW_TOKEN ?? '';
const FROM = opt('--from');
const OUT = opt('--out');
const MODEL = opt('--model') ?? 'sonnet';
const CONCURRENCY = Math.max(1, Number(opt('--concurrency') ?? 2) || 2);
const LIMIT = Number(opt('--limit') ?? 10) || 10;
const ONLY = opt('--only');
const DRY = flag('--dry');

if (!FROM && !(BASE && TOKEN)) {
	console.error('Give it --from <file> (downloaded from Review > Pre-reviews),');
	console.error('or --base <site> with PREREVIEW_TOKEN set. See the top of this file.');
	process.exit(1);
}

// The shape of the answer, so claude -p returns exactly this.
const SCHEMA = {
	type: 'object',
	additionalProperties: false,
	required: ['summary', 'scale', 'features', 'ai_usage', 'readme', 'demo', 'contributions', 'notes'],
	properties: {
		summary: { type: 'string' },
		scale: { type: 'string' },
		features: { type: 'array', items: { type: 'string' } },
		ai_usage: {
			type: 'object',
			additionalProperties: false,
			required: ['level', 'evidence'],
			properties: {
				level: { type: 'string', enum: ['none', 'low', 'medium', 'high'] },
				evidence: { type: 'array', items: { type: 'string' } }
			}
		},
		readme: { type: 'string' },
		demo: { type: 'string' },
		contributions: {
			type: 'array',
			items: {
				type: 'object',
				additionalProperties: false,
				required: ['name', 'commits', 'areas'],
				properties: { name: { type: 'string' }, commits: { type: 'integer' }, areas: { type: 'string' } }
			}
		},
		notes: { type: 'array', items: { type: 'string' } }
	}
};
// Reading the repo only: no edits, no web, no shell beyond looking.
const TOOLS = [
	'Read', 'Grep', 'Glob',
	'Bash(git log *)', 'Bash(git shortlog *)', 'Bash(git show *)', 'Bash(git diff *)',
	'Bash(ls *)', 'Bash(wc *)', 'Bash(find *)', 'Bash(head *)', 'Bash(du *)', 'Bash(file *)', 'Bash(tree *)'
].join(',');

function run(cmd, argv, cwd, env = process.env) {
	return new Promise((resolve) => {
		const p = spawn(cmd, argv, { cwd, env, stdio: ['ignore', 'pipe', 'pipe'] });
		let out = '';
		let err = '';
		p.stdout.on('data', (d) => (out += d));
		p.stderr.on('data', (d) => (err += d));
		p.on('error', (e) => resolve({ code: 1, out, err: String(e) }));
		p.on('close', (code) => resolve({ code: code ?? 1, out, err }));
	});
}

async function site(method, path, body) {
	const res = await fetch(BASE + '/api' + path, {
		method,
		headers: { authorization: `Bearer ${TOKEN}`, 'content-type': 'application/json' },
		body: body ? JSON.stringify(body) : undefined
	});
	const text = await res.text();
	if (!res.ok) throw new Error(`${method} ${path}: ${res.status} ${text.slice(0, 200)}`);
	return JSON.parse(text);
}

async function prereview(instructions, ship) {
	if (!/^https?:\/\//.test(ship.repo_url ?? '')) throw new Error('no repo link to clone');
	const dir = mkdtempSync(join(tmpdir(), `crossroads-prereview-${ship.id}-`));
	try {
		const url = ship.repo_url.replace(/\/$/, '').replace(/\/tree\/.*$/, '').replace(/\.git$/, '') + '.git';
		const clone = await run('git', ['clone', '-q', '--no-tags', '--filter=blob:limit=2m', url, dir], tmpdir(), { ...process.env, GIT_TERMINAL_PROMPT: '0' });
		if (clone.code !== 0) throw new Error(`couldn't clone ${url}: ${clone.err.trim().slice(0, 200)}`);
		const env = { ...process.env };
		for (const k of ['CLAUDECODE', 'CLAUDE_CODE_ENTRYPOINT', 'ANTHROPIC_API_KEY', 'ANTHROPIC_AUTH_TOKEN']) delete env[k];
		const prompt = `${instructions}\n<ship id=${ship.id}>\n${ship.context}</ship>\n`;
		const r = await run('claude', ['-p', prompt, '--model', MODEL, '--output-format', 'json', '--json-schema', JSON.stringify(SCHEMA), '--allowedTools', TOOLS], dir, env);
		if (r.code !== 0) throw new Error(`claude exited ${r.code}: ${(r.err || r.out).trim().slice(0, 300)}`);
		const parsed = JSON.parse(r.out);
		if (parsed.is_error) throw new Error(`claude: ${String(parsed.result).slice(0, 300)}`);
		let result = parsed.structured_output;
		if (!result && typeof parsed.result === 'string') {
			const m = parsed.result.match(/\{[\s\S]*\}/);
			if (m) result = JSON.parse(m[0]);
		}
		if (!result) throw new Error("claude didn't answer in JSON");
		return { ship_id: ship.id, ...result };
	} finally {
		rmSync(dir, { recursive: true, force: true });
	}
}

const batch = FROM
	? JSON.parse(readFileSync(FROM, 'utf8'))
	: await site('GET', `/prereviews?limit=${LIMIT}${flag('--all') ? '&all=1' : ''}`);
let ships = batch.ships ?? [];
if (ONLY) ships = ships.filter((s) => String(s.id) === ONLY);
if (!ships.length) {
	console.log('No ships to pre-review.');
	process.exit(0);
}
console.log(`Pre-reviewing ${ships.length} ship${ships.length === 1 ? '' : 's'} with ${MODEL}, ${CONCURRENCY} at a time…`);

const answers = [];
let failed = 0;
let next = 0;
await Promise.all(
	Array.from({ length: Math.min(CONCURRENCY, ships.length) }, async () => {
		while (next < ships.length) {
			const ship = ships[next++];
			try {
				answers.push(await prereview(batch.instructions, ship));
				console.log(`✓ ship ${ship.id} (${ship.name})`);
			} catch (e) {
				failed++;
				console.error(`✗ ship ${ship.id} (${ship.name}): ${e.message}`);
			}
		}
	})
);

const body = { prereviews: answers, model: MODEL, source: 'script' };
if (DRY) {
	console.log(JSON.stringify(body, null, 2));
} else if (OUT || !TOKEN) {
	writeFileSync(OUT ?? 'answer.json', JSON.stringify(body, null, 2));
	console.log(`Wrote ${answers.length} to ${OUT ?? 'answer.json'}: choose it on Review > Pre-reviews and save.`);
} else if (answers.length) {
	const res = await site('POST', '/prereviews', body);
	console.log(`Uploaded ${res.saved.length}.`);
	for (const f of res.failed) console.error(`✗ ship ${f.ship_id}: ${f.error}`);
}
if (failed) console.error(`${failed} couldn't be pre-reviewed.`);
