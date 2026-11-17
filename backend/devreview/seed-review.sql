-- Extra example data for trying the review flow, loaded after db/seed.sql by
-- devreview/run.sh. Test branch only: it's for the disconnected review test
-- database, never a real one.
--
--   fern    reviewer: fraud check only
--   hugo    reviewer: HQ sign-off only
--   zoe     Night Train Tracker: 4h, waiting for review, nothing wrong with it
--   yusuf   Sprite Forge: 26h, waiting for review; approving it sends it to
--           the fraud check (over the 20h large-ship limit). Has a reviewer note.
--   mei     Pocket Synth: at the fraud check, a 16-hour day flagged it
--           Tiny Compiler: waiting for review again, after HQ sent rita's
--           approval back
--   liam    Weather Wall: waiting for HQ, after the fraud check cut 9h to 7h
--
-- The end of this file sets up links and profiles for the review checks.
--
-- Plus feedback templates. Rita (every stage) and adam (admin) come from
-- db/seed.sql.

begin;

create function pg_temp.uid(text) returns bigint language sql stable
    as $$ select id from users where hca_id = 'dev-' || $1 $$;
create function pg_temp.project(text) returns bigint language sql stable
    as $$ select projects.id from projects join users on users.id = projects.user_id
        where users.hca_id like 'dev-%' and projects.name = $1 $$;
create function pg_temp.ship(text) returns bigint language sql stable
    as $$ select max(id) from submissions where project_id = pg_temp.project($1) $$;
create function pg_temp.ago(hours numeric) returns timestamptz language sql stable
    as $$ select now() - make_interval(secs => hours * 3600) $$;

insert into users (hca_id, slack_id, email, name, first_name, last_name, verification_status, ysws_eligible, role,
    can_review, can_fraud_review, can_hq_review, address)
select 'dev-' || n, 'UDEV' || upper(n), n || '@dev.localhost', initcap(n) || ' (dev)', initcap(n), '(dev)',
    'verified', true, role::user_role, stages like '%review%', stages like '%fraud%', stages like '%hq%',
    jsonb_build_object('name', n || ' (test)', 'line_1', '1 Test Street', 'line_2', '', 'city', 'Testville',
        'state', 'VT', 'postal_code', '05401', 'country', 'US')
from (values
    ('fern', 'reviewer', 'fraud'),
    ('hugo', 'reviewer', 'hq'),
    ('zoe', 'participant', ''),
    ('yusuf', 'participant', ''),
    ('mei', 'participant', ''),
    ('liam', 'participant', '')
) as v (n, role, stages);

update users set reviewer_note = 'Hackatime was set up late, on Nov 5: earlier work on Sprite Forge isn''t tracked.'
where hca_id = 'dev-yusuf';

insert into projects (user_id, name, description, repo_url, demo_url, screenshot_url, created_at)
select pg_temp.uid(owner), name, description,
    'https://github.com/dev-' || owner || '/' || slug,
    'https://dev-' || owner || '.github.io/' || slug,
    'https://placehold.co/1280x800/16161a/e8e4d8/png?text=' || replace(name, ' ', '+'),
    pg_temp.ago(created)
from (values
    ('zoe', 'Night Train Tracker', 'night-trains', 'Live positions of night trains across Europe, on one map.', 120),
    ('yusuf', 'Sprite Forge', 'sprite-forge', 'A pixel art editor with layers, onion skinning and GIF export.', 200),
    ('mei', 'Pocket Synth', 'pocket-synth', 'A synthesizer that runs in the browser, with a step sequencer.', 150),
    ('mei', 'Tiny Compiler', 'tiny-compiler', 'Compiles a small language to WebAssembly.', 90),
    ('liam', 'Weather Wall', 'weather-wall', 'An e-ink weather display for the kitchen wall, on a Raspberry Pi.', 170)
) as v (owner, name, slug, description, created);

insert into project_hackatime_projects (user_id, hackatime_project, project_id)
select projects.user_id, hackatime_project, projects.id
from (values
    ('Night Train Tracker', 'night-trains'),
    ('Sprite Forge', 'sprite-forge'),
    ('Pocket Synth', 'pocket-synth'),
    ('Tiny Compiler', 'tiny-compiler'),
    ('Weather Wall', 'weather-wall')
) as v (project, hackatime_project)
join projects on projects.id = pg_temp.project(v.project);

insert into submissions (project_id, status, hackatime_projects, hackatime_seconds, new_seconds, approved_seconds,
    reviewer_id, reviewed_at, feedback, internal_notes, fraud_reasons, fraud_reviewer_id, fraud_reviewed_at, submitted_at)
select pg_temp.project(project), status::submission_status,
    array(select hackatime_project from project_hackatime_projects where project_id = pg_temp.project(project)),
    (hours * 3600)::integer, (hours * 3600)::integer, (approved * 3600)::integer,
    pg_temp.uid(reviewer), pg_temp.ago(reviewed), feedback, notes, reasons::text[], pg_temp.uid(fraud), pg_temp.ago(fraud_at),
    pg_temp.ago(submitted)
from (values
    ('Night Train Tracker', 'pending', 4, null, null, null, null, null, '{}', null, null, 6),
    ('Sprite Forge', 'pending', 26, null, null, null, null, null, '{}', null, null, 20),
    ('Pocket Synth', 'fraud_check', 11, 11, 'rita', 3, 'The sequencer is so much fun!', 'Plays well. Commits look steady.', '{long_day}', null, null, 30),
    ('Tiny Compiler', 'pending', 5, null, null, null, null, null, '{}', null, null, 16),
    ('Weather Wall', 'awaiting_hq', 9, 7, 'rita', 26, 'Lovely build log and wiring photos.', 'Hardware works in the video.', '{manual}', 'fern', 5, 40)
) as v (project, status, hours, approved, reviewer, reviewed, feedback, notes, reasons, fraud, fraud_at, submitted);

-- Pocket Synth's 16-hour day, which is what flagged it.
insert into project_activity (project_id, day, seconds)
values (pg_temp.project('Pocket Synth'), (now() - interval '2 days')::date, 16 * 3600);

insert into ship_events (submission_id, kind, actor_id, stage, seconds, message, internal, created_at)
values
    (pg_temp.ship('Pocket Synth'), 'approval', pg_temp.uid('rita'), 'pending', 11 * 3600,
        'The sequencer is so much fun!', 'Plays well. Commits look steady.', pg_temp.ago(3)),
    (pg_temp.ship('Pocket Synth'), 'flagged', null, 'fraud_check', null, null,
        'No day over 14h: Hackatime counted 16h on this project on one day (UTC). Check for keep-alive scripts or autotyping.', pg_temp.ago(3)),
    (pg_temp.ship('Pocket Synth'), 'internal_comment', pg_temp.uid('rita'), 'fraud_check', null, null,
        'The 16h day lines up with a single giant commit. Worth a look.', pg_temp.ago(2.5)),

    (pg_temp.ship('Tiny Compiler'), 'approval', pg_temp.uid('rita'), 'pending', 5 * 3600,
        'Neat language! Love the error messages.', null, pg_temp.ago(12)),
    (pg_temp.ship('Tiny Compiler'), 'sent_back', pg_temp.uid('hugo'), 'awaiting_hq', null, null,
        'Day 2 only has commits for about 3h, but Hackatime has 5h. Can someone look again?', pg_temp.ago(8)),

    (pg_temp.ship('Weather Wall'), 'comment', pg_temp.uid('rita'), 'pending', null,
        'Could you add a photo of it on the wall to the README?', null, pg_temp.ago(30)),
    (pg_temp.ship('Weather Wall'), 'approval', pg_temp.uid('rita'), 'pending', 9 * 3600,
        'Lovely build log and wiring photos.', 'Hardware works in the video.', pg_temp.ago(26)),
    (pg_temp.ship('Weather Wall'), 'flagged', null, 'fraud_check', null, null, 'The reviewer asked for a fraud check.', pg_temp.ago(26)),
    (pg_temp.ship('Weather Wall'), 'fraud_pass', pg_temp.uid('fern'), 'fraud_check', 7 * 3600, null,
        'About 2h was the IDE left open overnight (one long span at 3am). Cut to 7h.', pg_temp.ago(5));

insert into feedback_templates (title, body, created_by) values
    ('Demo link broken', 'Your demo link doesn''t load for us. Fix it (or deploy it somewhere new) and ship again!', pg_temp.uid('rita')),
    ('README missing setup', 'Your README doesn''t say how to run the project. Add the steps someone needs to try it, then ship again.', pg_temp.uid('rita')),
    ('Great work', 'This is great work. Thank you for shipping it!', pg_temp.uid('rita')),
    ('Hours cut: tutorial code', 'We cut some hours because part of the code follows a tutorial line for line. Copied work can''t count, but everything you built yourself does.', pg_temp.uid('adam'));


-- For the review checks (backend/api/checks.go). Real-looking names and
-- birthdays, so the Unified YSWS checks pass, except where a ship is meant to
-- show a problem. Dev mode fakes GitHub and link checks, and words in a link
-- act out a case (planning/DEV_MODE.md).
update users set last_name = v.last, birthday = '2009-05-01'
from (values
    ('alice', 'Moreau'), ('bob', 'test'), ('maya', 'Okafor'), ('theo', 'Lindqvist'), ('sana', 'Haddad'),
    ('jonas', 'Becker'), ('lea', 'Fontaine'), ('kofi', 'Mensah'), ('omar', 'Farouk'), ('nina', 'Petrova'),
    ('rita', 'Alvarez'), ('adam', 'Novak'), ('fern', 'Gallagher'), ('hugo', 'Dupont'), ('zoe', 'Martin'),
    ('yusuf', 'Demir'), ('mei', 'Tanaka'), ('liam', 'Walsh')
) as v (n, last)
where users.hca_id = 'dev-' || v.n;
-- Liam has no birthday on file: the Unified YSWS DB needs one.
update users set birthday = null where hca_id = 'dev-liam';

-- Study Buddy Bot (bob, waiting for review): a private repo, a demo that 404s,
-- and "test" as a last name.
update projects set repo_url = 'https://github.com/dev-bob/study-buddy-private',
    demo_url = 'https://dev-bob.github.io/study-buddy-broken' where id = pg_temp.project('Study Buddy Bot');
-- Language Flashcards (sana, waiting for review): already in the Unified YSWS
-- DB from another program, so approving it sends it to the fraud check.
update projects set repo_url = 'https://github.com/dev-sana/flashcards-double-dip' where id = pg_temp.project('Language Flashcards');
-- Tiny Compiler (mei, waiting for review): no commits since the event started.
update projects set repo_url = 'https://github.com/dev-mei/tiny-compiler-stale' where id = pg_temp.project('Tiny Compiler');
-- Pocket Synth (mei, at the fraud check): the README is Vite's template.
update projects set repo_url = 'https://github.com/dev-mei/pocket-synth-template' where id = pg_temp.project('Pocket Synth');
-- Weather Wall (liam, waiting for HQ): a friend made a third of the commits.
update projects set repo_url = 'https://github.com/dev-liam/weather-wall-team' where id = pg_temp.project('Weather Wall');

commit;
