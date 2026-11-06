-- Fake data for development: an admin setup (special stamps, boxes,
-- milestones, shop) and test users in every state the platform has to show.
-- How to load it is in planning/DEVELOPMENT.md, "Fake data".
--
-- It only fills a database with no admin setup yet, and refuses to run on
-- production. It's one transaction, so it adds everything or nothing.
--
-- The users are the ones DEV_MODE's /dev/login makes, so locally
-- /dev/login?name=maya signs in as Maya here. Pass role=reviewer for rita and
-- role=admin for adam, and verified=0 for nina, or signing in changes them.
--
--   adam    admin
--   rita    reviewer
--   alice   10h approved, a draft missing its demo and screenshot, an unopened box from an admin
--   bob     rejected once, shipped again and waiting for review
--   maya    58h approved, 60 stamps short of the ticket, an unopened box and an unused Express Lane
--   theo    64h approved, bought the ticket, got the only Last Train Home
--   sana    waiting for review with an Express Lane applied (top of the queue), a poster on order
--   jonas   12 of 18 hours approved (cut), shipped again for 3 more, an unopened box
--   lea     a cancelled and refunded hoodie, a travel stipend on order
--   kofi    negative balance after an admin clawback
--   omar    waiting for review, shares a repo link with kofi's project
--   ines    new: verified, nothing yet but a special stamp from an admin
--   nina    not verified yet, one project ready to ship
--
-- Stamps are paid at 30 per approved hour, like STAMPS_PER_HOUR. Times are
-- relative to when it's loaded, so the data always looks recent.

begin;

do $$
begin
    if current_setting('cluster_name') = 'pg-crossroads-db' then
        raise exception 'This is the production database. The seed only goes into development ones.';
    end if;
    if exists (select 1 from special_stamp_types) or exists (select 1 from box_types)
        or exists (select 1 from milestones) or exists (select 1 from shop_items)
        or exists (select 1 from projects join users on users.id = projects.user_id where users.hca_id like 'dev-%') then
        raise exception 'This database already has data. Reset it first (planning/DEVELOPMENT.md, "Fake data").';
    end if;
end;
$$;

-- Lookups by name. The arguments are only $1, because a column called name
-- would win over an argument called name.
create function pg_temp.uid(text) returns bigint language sql stable
    as $$ select id from users where hca_id = 'dev-' || $1 $$;
create function pg_temp.project(text) returns bigint language sql stable
    as $$ select projects.id from projects join users on users.id = projects.user_id
        where users.hca_id like 'dev-%' and projects.name = $1 $$;
create function pg_temp.stamp_type(text) returns bigint language sql stable
    as $$ select id from special_stamp_types where name = $1 $$;
create function pg_temp.box_type(text) returns bigint language sql stable
    as $$ select id from box_types where name = $1 $$;
create function pg_temp.milestone(text) returns bigint language sql stable
    as $$ select id from milestones where name = $1 $$;
create function pg_temp.item(text) returns bigint language sql stable
    as $$ select id from shop_items where name = $1 $$;
create function pg_temp.ago(hours numeric) returns timestamptz language sql stable
    as $$ select now() - make_interval(secs => hours * 3600) $$;

-- Users, shaped exactly like /dev/login makes them. Upserted, because local
-- databases often have alice already.
insert into users (hca_id, slack_id, email, name, first_name, last_name, verification_status, ysws_eligible, role, address)
select 'dev-' || n, 'UDEV' || upper(n), n || '@dev.localhost', initcap(n) || ' (dev)', initcap(n), '(dev)',
    status, status = 'verified', role::user_role,
    jsonb_build_object('name', n || ' (test)', 'line_1', '1 Test Street', 'line_2', '', 'city', 'Testville',
        'state', 'VT', 'postal_code', '05401', 'country', 'US')
from (values
    ('adam', 'admin', 'verified'),
    ('rita', 'reviewer', 'verified'),
    ('alice', 'participant', 'verified'),
    ('bob', 'participant', 'verified'),
    ('maya', 'participant', 'verified'),
    ('theo', 'participant', 'verified'),
    ('sana', 'participant', 'verified'),
    ('jonas', 'participant', 'verified'),
    ('lea', 'participant', 'verified'),
    ('kofi', 'participant', 'verified'),
    ('omar', 'participant', 'verified'),
    ('ines', 'participant', 'verified'),
    ('nina', 'participant', 'needs_submission')
) as v (n, role, status)
on conflict (hca_id) do update set
    role = excluded.role,
    verification_status = excluded.verification_status,
    ysws_eligible = excluded.ysws_eligible;

-- The admin setup.
insert into special_stamp_types (name, description, rarity, effect, supply_cap) values
    ('Ticket Punch', 'A clean hole through your first ticket.', 'common', 'none', null),
    ('Window Seat', 'The best view on the train.', 'common', 'none', null),
    ('Sleeper Car', 'For the overnight stretch.', 'uncommon', 'none', 150),
    ('Express Lane', 'Puts one submission at the front of the review queue.', 'rare', 'priority_review', 40),
    ('Golden Rail', 'Rarely seen on any line.', 'epic', 'none', 10),
    ('Last Train Home', 'There was only ever one.', 'legendary', 'none', 1);

insert into box_types (name, description) values
    ('Station Box', 'Given at 10 and 30 approved hours.'),
    ('Terminus Box', 'Given at 50 approved hours, with better odds.');

insert into box_drops (box_type_id, special_stamp_type_id, weight) values
    (pg_temp.box_type('Station Box'), pg_temp.stamp_type('Ticket Punch'), 50),
    (pg_temp.box_type('Station Box'), pg_temp.stamp_type('Window Seat'), 30),
    (pg_temp.box_type('Station Box'), pg_temp.stamp_type('Sleeper Car'), 15),
    (pg_temp.box_type('Station Box'), pg_temp.stamp_type('Express Lane'), 5),
    (pg_temp.box_type('Terminus Box'), pg_temp.stamp_type('Window Seat'), 30),
    (pg_temp.box_type('Terminus Box'), pg_temp.stamp_type('Sleeper Car'), 30),
    (pg_temp.box_type('Terminus Box'), pg_temp.stamp_type('Express Lane'), 25),
    (pg_temp.box_type('Terminus Box'), pg_temp.stamp_type('Golden Rail'), 12),
    (pg_temp.box_type('Terminus Box'), pg_temp.stamp_type('Last Train Home'), 3);

insert into milestones (name, approved_seconds, box_type_id) values
    ('First stop', 10 * 3600, pg_temp.box_type('Station Box')),
    ('Halfway', 30 * 3600, pg_temp.box_type('Station Box')),
    ('Almost there', 50 * 3600, pg_temp.box_type('Terminus Box'));

-- Stock is what's left after the orders below; the ones not cancelled took one each.
insert into shop_items (kind, name, description, price, stipend_cents, stock, max_per_user, active) values
    ('ticket', 'Crossroads ticket', 'Your seat on the train: Paris, January 8 to 12, 2027.', 1800, null, 29, 1, true),
    ('physical', 'Sticker pack', 'Ten stickers from the trip.', 60, null, null, null, true),
    ('physical', 'Rail map poster', 'Every stop on the route, printed A2.', 150, null, 24, 1, true),
    ('physical', 'Night train hoodie', 'Warm enough for the overnight stretch.', 600, null, 15, 1, true),
    ('stipend', 'Travel stipend', 'Help getting to Paris, sent before the trip.', 300, 5000, null, 4, true),
    ('physical', 'Early-bird pin', 'Only for the first week. Sold out.', 90, null, 0, 1, false);

-- Projects. Everything shipped has every field filled in, as shipping requires.
insert into projects (user_id, name, description, repo_url, demo_url, screenshot_url, created_at)
select pg_temp.uid(owner), name, description,
    'https://github.com/dev-' || owner || '/' || slug,
    case when shipped then 'https://dev-' || owner || '.github.io/' || slug end,
    case when shipped then 'https://placehold.co/1280x800/16161a/e8e4d8/png?text=' || replace(name, ' ', '+') end,
    pg_temp.ago(created)
from (values
    ('alice', 'Pixel Platformer', 'pixel-platformer', 'A platformer with a double jump and twelve levels.', true, 46),
    ('alice', 'Portfolio Site', 'portfolio', 'My projects in one place. Still needs a demo and a screenshot.', false, 10),
    ('bob', 'Study Buddy Bot', 'study-buddy', 'A Discord bot that runs pomodoro timers for a whole server.', true, 40),
    ('maya', 'Synth in the Browser', 'browser-synth', 'A polyphonic synthesizer built on the Web Audio API.', true, 47),
    ('maya', 'Tiny OS', 'tiny-os', 'A hobby operating system that boots to a shell on x86.', true, 46),
    ('maya', 'Weather Station', 'weather-station', 'An ESP32 that logs temperature and humidity to a web page.', true, 30),
    ('theo', 'Tram Tracker', 'tram-tracker', 'Live tram positions on a map, from the city''s open data.', true, 47),
    ('theo', 'Mesh Radio', 'mesh-radio', 'Text messages over LoRa radios with no internet.', true, 20),
    ('sana', 'Plant Watering Bot', 'plant-bot', 'Waters plants when the soil is dry and posts to Slack.', true, 38),
    ('sana', 'Language Flashcards', 'flashcards', 'Spaced-repetition flashcards that work offline.', true, 20),
    ('jonas', 'Chess Engine', 'chess-engine', 'A chess engine with alpha-beta search and a UCI interface.', true, 44),
    ('lea', 'Bus Times Display', 'bus-display', 'An e-ink screen that shows when the next buses leave.', true, 47),
    ('kofi', 'Quiz Night', 'quiz-night', 'A multiplayer quiz game for phones on the same Wi-Fi.', true, 40),
    ('omar', 'Quiz Night Online', 'quiz-night-online', 'Quiz Night, playable over the internet.', true, 16),
    ('nina', 'Recipe Scaler', 'recipe-scaler', 'Scales a recipe to any number of people and converts units.', true, 12)
) as v (owner, name, slug, description, shipped, created);

-- Omar's project shares Kofi's repo, so reviewers see the shared-link warning.
update projects set repo_url = 'https://github.com/dev-kofi/quiz-night'
where id = pg_temp.project('Quiz Night Online');

-- Alice and Bob use the Hackatime projects every dev user starts with in
-- DEV_MODE, so their time lines up with the fake locally.
insert into project_hackatime_projects (user_id, hackatime_project, project_id)
select projects.user_id, hackatime_project, projects.id
from (values
    ('Pixel Platformer', 'crossroads-game'),
    ('Portfolio Site', 'personal-site'),
    ('Study Buddy Bot', 'discord-bot'),
    ('Synth in the Browser', 'browser-synth'),
    ('Tiny OS', 'tiny-os'),
    ('Weather Station', 'weather-station'),
    ('Tram Tracker', 'tram-tracker'),
    ('Mesh Radio', 'mesh-radio'),
    ('Plant Watering Bot', 'plant-bot'),
    ('Language Flashcards', 'flashcards'),
    ('Chess Engine', 'chess-engine'),
    ('Bus Times Display', 'bus-display'),
    ('Quiz Night', 'quiz-night'),
    ('Quiz Night Online', 'quiz-night'),
    ('Recipe Scaler', 'recipe-scaler')
) as v (project, hackatime_project)
join projects on projects.id = pg_temp.project(v.project);

-- Submissions, in hours. new_seconds follows the backend's rule: time no
-- approved submission accounted for, which includes hours a reviewer cut.
insert into submissions (project_id, status, hackatime_projects, hackatime_seconds, new_seconds, approved_seconds,
    reviewer_id, reviewed_at, feedback, internal_notes, submitted_at)
select pg_temp.project(project), status::submission_status,
    array(select hackatime_project from project_hackatime_projects where project_id = pg_temp.project(project)),
    (hackatime * 3600)::integer, (new * 3600)::integer, (approved * 3600)::integer,
    pg_temp.uid(reviewer), pg_temp.ago(reviewed), feedback, notes, pg_temp.ago(submitted)
from (values
    ('Pixel Platformer', 'approved', 10, 10, 10, 'rita', 26, 'Fun to play! The double jump feels great.', null, 30),
    ('Study Buddy Bot', 'rejected', 1.5, 1.5, null, 'rita', 24,
        'The demo link goes to a page that doesn''t exist, and the README doesn''t say how to run the bot. Fix those and ship it again.', null, 28),
    ('Study Buddy Bot', 'pending', 2.5, 2.5, null, null, null, null, null, 3),
    ('Synth in the Browser', 'approved', 20, 20, 20, 'rita', 36, 'Really polished. The filter sweep is lovely.', null, 40),
    ('Tiny OS', 'approved', 26, 26, 26, 'adam', 20, null, 'Booted it in QEMU, works.', 22),
    ('Weather Station', 'approved', 12, 12, 12, 'rita', 6, 'Great write-up of the wiring.', null, 8),
    ('Tram Tracker', 'approved', 34, 34, 34, 'rita', 42, null, null, 44),
    ('Mesh Radio', 'approved', 30, 30, 30, 'adam', 12, 'Love that it works with no internet at all.', null, 14),
    ('Plant Watering Bot', 'approved', 22, 22, 22, 'rita', 31, null, null, 34),
    ('Language Flashcards', 'pending', 6, 6, null, null, null, null, null, 2),
    ('Chess Engine', 'approved', 18, 18, 12, 'adam', 33,
        'We approved 12 of your 18 hours. About 6 hours went into a tutorial project that was copied into the repo, and copied work can''t count. The engine itself is great!',
        'Commits from the first day match the tutorial repo line for line.', 38),
    ('Chess Engine', 'pending', 21, 3, null, null, null, null, null, 1),
    ('Bus Times Display', 'approved', 26, 26, 26, 'rita', 43, null, null, 45),
    ('Quiz Night', 'approved', 8, 8, 8, 'rita', 32, null, null, 36),
    ('Quiz Night Online', 'pending', 9, 9, null, null, null, null, null, 4)
) as v (project, status, hackatime, new, approved, reviewer, reviewed, feedback, notes, submitted);

-- Payouts on each person's running total of approved time, like the backend,
-- so rounding never loses a stamp.
insert into stamp_ledger (user_id, kind, amount, submission_id, created_at)
select user_id, 'payout', amount, id, reviewed_at
from (
    select projects.user_id, submissions.id, submissions.reviewed_at,
        (sum(submissions.approved_seconds) over w) * 30 / 3600
            - (sum(submissions.approved_seconds) over w - submissions.approved_seconds) * 30 / 3600 as amount
    from submissions
    join projects on projects.id = submissions.project_id
    join users on users.id = projects.user_id
    where submissions.status = 'approved' and users.hca_id like 'dev-%'
    window w as (partition by projects.user_id order by submissions.reviewed_at, submissions.id)
) as payouts
where amount > 0;

-- A box for every milestone, given when the approval that passed it happened.
insert into boxes (user_id, box_type_id, milestone_id, granted_at)
select distinct on (totals.user_id, milestones.id) totals.user_id, milestones.box_type_id, milestones.id, totals.reviewed_at
from (
    select projects.user_id, submissions.id, submissions.reviewed_at,
        sum(submissions.approved_seconds) over (partition by projects.user_id order by submissions.reviewed_at, submissions.id) as total
    from submissions
    join projects on projects.id = submissions.project_id
    join users on users.id = projects.user_id
    where submissions.status = 'approved' and users.hca_id like 'dev-%'
) as totals
join milestones on milestones.approved_seconds <= totals.total
order by totals.user_id, milestones.id, totals.reviewed_at, totals.id;

insert into boxes (user_id, box_type_id, granted_by, granted_at)
values (pg_temp.uid('alice'), pg_temp.box_type('Station Box'), pg_temp.uid('adam'), pg_temp.ago(5));

-- Open some boxes. The rest stay closed so opening one can be tried.
with opened (owner, milestone, stamp_type) as (values
    ('alice', 'First stop', 'Ticket Punch'),
    ('maya', 'First stop', 'Window Seat'),
    ('maya', 'Halfway', 'Express Lane'),
    ('theo', 'First stop', 'Ticket Punch'),
    ('theo', 'Halfway', 'Sleeper Car'),
    ('theo', 'Almost there', 'Last Train Home'),
    ('sana', 'First stop', 'Express Lane'),
    ('lea', 'First stop', 'Window Seat')
), marked as (
    update boxes set opened_at = boxes.granted_at + interval '20 minutes'
    from opened
    where boxes.user_id = pg_temp.uid(opened.owner) and boxes.milestone_id = pg_temp.milestone(opened.milestone)
    returning boxes.*, opened.stamp_type
)
insert into special_stamps (user_id, special_stamp_type_id, box_id, acquired_at)
select user_id, pg_temp.stamp_type(stamp_type), id, opened_at from marked;

insert into special_stamps (user_id, special_stamp_type_id, granted_by, acquired_at)
values (pg_temp.uid('ines'), pg_temp.stamp_type('Window Seat'), pg_temp.uid('adam'), pg_temp.ago(2));

-- Sana used her Express Lane on the submission waiting for review.
update special_stamps set submission_id = (
    select id from submissions where project_id = pg_temp.project('Language Flashcards') and status = 'pending'
)
where user_id = pg_temp.uid('sana') and special_stamp_type_id = pg_temp.stamp_type('Express Lane');

update special_stamp_types set issued_count = (
    select count(*) from special_stamps where special_stamps.special_stamp_type_id = special_stamp_types.id
);

-- Orders, and what they cost. Physical items keep a copy of the address.
insert into orders (user_id, shop_item_id, price, status, shipping_address, notes, created_at)
select pg_temp.uid(owner), shop_items.id, shop_items.price, status::order_status,
    case when shop_items.kind = 'physical' then users.address end, notes, pg_temp.ago(ordered)
from (values
    ('maya', 'Sticker pack', 'fulfilled', 'Shipped with the first batch.', 5),
    ('theo', 'Crossroads ticket', 'fulfilled', 'Seat confirmed.', 11),
    ('sana', 'Rail map poster', 'pending', null, 25),
    ('lea', 'Night train hoodie', 'cancelled', 'Out of stock in that size, so it was refunded.', 40),
    ('lea', 'Travel stipend', 'pending', null, 18)
) as v (owner, item, status, notes, ordered)
join shop_items on shop_items.id = pg_temp.item(v.item)
join users on users.id = pg_temp.uid(v.owner);

-- In time order, since a purchase can't take a balance below zero: Lea can
-- only afford the stipend because the hoodie was refunded first.
insert into stamp_ledger (user_id, kind, amount, order_id, created_at)
select user_id, kind::stamp_entry_kind, amount, id, at
from (
    select user_id, 'purchase' as kind, -price as amount, id, created_at as at from orders
    union all
    select user_id, 'refund', price, id, created_at + interval '10 hours' from orders where status = 'cancelled'
) as entries
where user_id in (select id from users where hca_id like 'dev-%')
order by at;

insert into stamp_ledger (user_id, kind, amount, created_by, note, created_at)
values (pg_temp.uid('kofi'), 'adjustment', -300, pg_temp.uid('adam'),
    'Clawback: these hours were also shipped to another YSWS.', pg_temp.ago(10));

commit;
