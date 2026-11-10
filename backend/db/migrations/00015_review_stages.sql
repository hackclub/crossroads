-- +goose Up

-- Ships now go through up to three stages before they count:
--   pending      a reviewer decides the hours (or rejects)
--   fraud_check  automatic checks flagged it (or a reviewer or the
--                "everything goes through fraud" setting sent it here), and a
--                fraud reviewer passes it, cuts hours or rejects it
--   awaiting_hq  HQ signs off (or sends it back, or rejects it)
--   approved     signed off: stamps and boxes are paid, and it goes to the
--                Unified YSWS DB
-- Ships approved before this migration were already paid and stay approved.

alter type submission_status rename to submission_status_v1;
create type submission_status as enum ('pending', 'fraud_check', 'awaiting_hq', 'approved', 'rejected');

drop index submissions_one_pending_per_project;
drop index submissions_review_queue;
drop index submissions_to_sync;
alter table submissions
    drop constraint submissions_check,
    drop constraint submissions_check1,
    drop constraint submissions_check2,
    alter column status drop default,
    alter column status type submission_status using status::text::submission_status,
    alter column status set default 'pending';
drop type submission_status_v1;

alter table submissions
    -- What the automatic checks said when the reviewer decided, and why it
    -- went to the fraud check: failed check IDs, 'manual' or 'everything'.
    add column checks jsonb,
    add column fraud_reasons text[] not null default '{}',
    add column fraud_reviewer_id bigint references users (id),
    add column fraud_reviewed_at timestamptz,
    add column hq_reviewer_id bigint references users (id),
    add column hq_reviewed_at timestamptz,
    -- The reviewer working on it right now. Claims run out, so a reviewer who
    -- walks away doesn't hold a ship forever.
    add column claimed_by bigint references users (id),
    add column claim_expires_at timestamptz,
    -- When the Airtable sync ticked "Automation - Submit to Unified YSWS" on
    -- its row. That waits for the person to be YSWS-eligible.
    add column unified_submitted_at timestamptz,
    -- reviewer_id and reviewed_at are the first reviewer's decision.
    add constraint submissions_reviewer_set check ((status = 'pending') = (reviewer_id is null)),
    add constraint submissions_reviewed_at_set check ((status = 'pending') = (reviewed_at is null)),
    -- approved_seconds is what the ship will pay once it's signed off: the
    -- reviewer's number, less any fraud cut, as HQ left it.
    add constraint submissions_approved_seconds_set
        check ((status in ('fraud_check', 'awaiting_hq', 'approved')) = (approved_seconds is not null)),
    add constraint submissions_claim_complete check ((claimed_by is null) = (claim_expires_at is null)),
    add constraint submissions_fraud_review_complete check ((fraud_reviewer_id is null) = (fraud_reviewed_at is null)),
    add constraint submissions_hq_review_complete check ((hq_reviewer_id is null) = (hq_reviewed_at is null));

-- One ship in review per project, at whatever stage.
create unique index submissions_one_open_per_project on submissions (project_id)
    where status in ('pending', 'fraud_check', 'awaiting_hq');
create index submissions_open on submissions (status, submitted_at)
    where status in ('pending', 'fraud_check', 'awaiting_hq');
create index submissions_to_sync on submissions (id)
    where status = 'approved' and airtable_record_id is null;
create index submissions_to_submit_unified on submissions (id)
    where status = 'approved' and airtable_record_id is not null and unified_submitted_at is null;

-- Everything that happened to a ship, in order: decisions at each stage and
-- comments. The submissions row is where a ship is now; this is how it got
-- there, including approvals that were later sent back.
create type ship_event_kind as enum (
    'comment',          -- for the participant (reviewers see it too)
    'internal_comment', -- reviewers only
    'flagged',          -- the automatic checks sent it to the fraud check
    'approval',         -- a reviewer approved hours
    'fraud_pass',       -- the fraud check passed it, maybe cutting hours
    'sent_back',        -- an approval was discarded; back to the review queue
    'final_approval',   -- HQ signed off; stamps were paid
    'rejection'         -- rejected, at any stage
);

create table ship_events (
    id bigint generated always as identity primary key,
    submission_id bigint not null references submissions (id),
    kind ship_event_kind not null,
    actor_id bigint references users (id), -- null for 'flagged'
    -- The ship's status when it happened.
    stage submission_status not null,
    -- Approvals: the hours from then on.
    seconds integer check (seconds >= 0),
    message text,  -- shown to the participant
    internal text, -- reviewers only: justification, notes, reasons
    created_at timestamptz not null default now(),
    edited_at timestamptz,
    check ((kind = 'flagged') = (actor_id is null)),
    check ((kind in ('approval', 'fraud_pass', 'final_approval')) = (seconds is not null))
);

create index on ship_events (submission_id, id);

-- Decisions made before the stages existed.
insert into ship_events (submission_id, kind, actor_id, stage, seconds, message, internal, created_at)
select id, case when status = 'approved' then 'approval' else 'rejection' end::ship_event_kind,
    reviewer_id, 'pending', approved_seconds, feedback, internal_notes, reviewed_at
from submissions
where status <> 'pending'
order by id;

-- Per-person review permissions. Admins can do everything; reviewers can see
-- the queues and do the stages they have a flag for.
alter table users
    add column can_review boolean not null default false,
    add column can_fraud_review boolean not null default false,
    add column can_hq_review boolean not null default false,
    -- A note about the person that only reviewers see, across all their ships.
    add column reviewer_note text;
update users set can_review = true where role = 'reviewer';

-- A reviewer who skips a ship doesn't get it from "Next" again for a while.
create table review_skips (
    user_id bigint not null references users (id),
    submission_id bigint not null references submissions (id),
    skipped_at timestamptz not null default now(),
    primary key (user_id, submission_id)
);

-- Snippets reviewers can drop into feedback. Shared by everyone who reviews.
create table feedback_templates (
    id bigint generated always as identity primary key,
    title text not null check (length(title) between 1 and 100),
    body text not null check (length(body) between 1 and 5000),
    created_by bigint references users (id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create trigger feedback_templates_set_updated_at before update on feedback_templates
    for each row execute function set_updated_at();

-- One row of review settings, changed by admins.
create table review_settings (
    id boolean primary key default true check (id),
    -- Every approved ship goes through the fraud check, flagged or not.
    fraud_check_everything boolean not null default false,
    -- Ships claiming more than this go to the fraud check.
    large_ship_seconds integer not null default 72000 check (large_ship_seconds > 0),
    -- So do ships whose project has a UTC day with more than this on it.
    long_day_seconds integer not null default 50400 check (long_day_seconds between 1 and 86400),
    updated_at timestamptz not null default now()
);

create trigger review_settings_set_updated_at before update on review_settings
    for each row execute function set_updated_at();

insert into review_settings default values;

-- +goose Down
drop table review_settings;
drop table feedback_templates;
drop table review_skips;
alter table users
    drop column can_review,
    drop column can_fraud_review,
    drop column can_hq_review,
    drop column reviewer_note;
drop table ship_events;
drop type ship_event_kind;

drop index submissions_one_open_per_project;
drop index submissions_open;
drop index submissions_to_sync;
drop index submissions_to_submit_unified;
alter table submissions
    drop constraint submissions_reviewer_set,
    drop constraint submissions_reviewed_at_set,
    drop constraint submissions_approved_seconds_set,
    drop constraint submissions_claim_complete,
    drop constraint submissions_fraud_review_complete,
    drop constraint submissions_hq_review_complete,
    drop column checks,
    drop column fraud_reasons,
    drop column fraud_reviewer_id,
    drop column fraud_reviewed_at,
    drop column hq_reviewer_id,
    drop column hq_reviewed_at,
    drop column claimed_by,
    drop column claim_expires_at,
    drop column unified_submitted_at;

-- Ships part-way through the stages go back to waiting for a reviewer.
update submissions set status = 'pending', reviewer_id = null, reviewed_at = null, approved_seconds = null
where status in ('fraud_check', 'awaiting_hq');

alter type submission_status rename to submission_status_v2;
create type submission_status as enum ('pending', 'approved', 'rejected');
alter table submissions
    alter column status drop default,
    alter column status type submission_status using status::text::submission_status,
    alter column status set default 'pending';
drop type submission_status_v2;

alter table submissions
    add constraint submissions_check check ((status = 'pending') = (reviewer_id is null)),
    add constraint submissions_check1 check ((status = 'pending') = (reviewed_at is null)),
    add constraint submissions_check2 check ((status = 'approved') = (approved_seconds is not null));
create unique index submissions_one_pending_per_project on submissions (project_id)
    where status = 'pending';
create index submissions_review_queue on submissions (submitted_at)
    where status = 'pending';
create index submissions_to_sync on submissions (id)
    where status = 'approved' and airtable_record_id is null;
