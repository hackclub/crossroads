# crossroads slackbot

Posts Block Kit messages to Slack **as you**: your name and avatar, with a small "Sent using Crossroads Announcements" under it. Design a message in [Block Kit Builder](https://app.slack.com/block-kit-builder) (headers, sections, images, buttons, anything) and post its JSON.

Any button with an `action_id` gets its clicks counted. `example.json` starts you off with the two Crossroads ones:

- **Open Crossroads** (`open_platform`) goes to https://crossroads.hackclub.com/.
- **Claim your stamp** (`claim_stamp`) goes to `https://crossroads.hackclub.com/claim/<code>`, which signs people in and gives them that stamp type, once each. The code is the stamp type's claim code, set on the admin page's special stamp types.

Link buttons open by themselves, but Slack still tells the app about every click, and `bot.js` records each one on the backend (`slack_clicks`, by Slack ID and `action_id`), including clicks from people who never sign in.

There are two halves:

- `post.js` posts or edits a message with your user token (`xoxp-`). Run it from your own machine; that token can post as you anywhere, so it doesn't go on a server.
- `bot.js` runs all the time (on Orchard) with only the app-level token (`xapp-`), which can do nothing but receive the clicks. It forwards them to the backend's `POST /slack/clicks`.

## Setting up the Slack app (once)

1. https://api.slack.com/apps → Create New App → From a manifest → the Hack Club workspace → paste `manifest.json`.
2. Basic Information → App-Level Tokens → generate one with the `connections:write` scope. That's `SLACK_APP_TOKEN` (`xapp-…`) for the bot.
3. Install App → install to the workspace. The User OAuth Token is `SLACK_USER_TOKEN` (`xoxp-…`) for posting.

## Posting

1. Design the message in Block Kit Builder, or start from `example.json` (put the real claim code in its URL).
2. Copy the builder's JSON (`{"blocks": [...]}`) into a file. Add `"text"` next to `"blocks"` for the notification preview; otherwise it's the message's first text.
3. Post it:

```sh
SLACK_USER_TOKEN=xoxp-... bun post.js C0CDHHFABAR message.json
```

`C0CDHHFABAR` is #crossroads; try it in a test channel or your own DM (your user ID as the channel) first. It prints the message's `ts`; to fix something after posting, edit the file and run the same command with `--edit <ts>`.

## Running the bot

```sh
bun install
SLACK_APP_TOKEN=xapp-... BACKEND_URL=http://localhost:8080 SLACK_CLICKS_SECRET=... bun bot.js
```

The backend needs the same `SLACK_CLICKS_SECRET`; without it, `/slack/clicks` isn't mounted. If the bot is down when someone clicks, the link still opens but the click isn't counted.

On Orchard it runs in the Production environment next to the backend and reaches it inside the cluster, at `BACKEND_URL=http://crossroads.ysws-crossroads.svc.cluster.local:8080`, since the public ingress only sends `/api`, `/auth` and `/uploads` to the backend. It serves no HTTP, so it needs no ingress.

## Reading the clicks

```sql
select button, count(*) as clicks, count(distinct slack_id) as people
from slack_clicks group by button;

-- who clicked, and whether they have an account
select slack_clicks.*, users.name
from slack_clicks left join users on users.slack_id = slack_clicks.slack_id
order by clicked_at desc;
```
