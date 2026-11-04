-- +goose Up
create table projects (
    id bigint generated always as identity primary key,
    user_id bigint not null references users (id),
    name text not null,
    description text not null default '',
    repo_url text,
    demo_url text,
    screenshot_url text,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (id, user_id) -- target for project_hackatime_projects' foreign key
);

create index on projects (user_id);

create trigger projects_set_updated_at before update on projects
    for each row execute function set_updated_at();

-- The Hackatime projects whose time counts toward a project. Each Hackatime
-- project counts toward at most one of its owner's projects, so the same hours
-- can't be paid out twice.
create table project_hackatime_projects (
    user_id bigint not null,
    hackatime_project text not null,
    project_id bigint not null,
    primary key (user_id, hackatime_project),
    foreign key (project_id, user_id) references projects (id, user_id) on delete cascade
);

create index on project_hackatime_projects (project_id);

create type submission_status as enum ('pending', 'approved', 'rejected');

-- Each time a project is sent for review. A project can be submitted again
-- after an approval or rejection, and is only paid for time nobody has claimed.
create table submissions (
    id bigint generated always as identity primary key,
    project_id bigint not null references projects (id),
    status submission_status not null default 'pending',
    -- The Hackatime projects this submission counted. Once it's approved,
    -- they can't be unlinked from the project.
    hackatime_projects text[] not null,
    -- The project's Hackatime time since the event started, as of submitting.
    hackatime_seconds integer not null check (hackatime_seconds >= 0),
    -- The time this submission claims: what its approved submissions hadn't
    -- covered, capped at what its owner hadn't claimed on any project.
    new_seconds integer not null check (new_seconds >= 0),
    -- How much of new_seconds the reviewer accepted. This is what pays out and
    -- counts toward milestones.
    approved_seconds integer constraint submissions_approved_within_new
        check (approved_seconds between 0 and new_seconds),
    reviewer_id bigint references users (id),
    reviewed_at timestamptz,
    feedback text, -- shown to the participant
    internal_notes text, -- reviewers only
    submitted_at timestamptz not null default now(),
    check ((status = 'pending') = (reviewer_id is null)),
    check ((status = 'pending') = (reviewed_at is null)),
    check ((status = 'approved') = (approved_seconds is not null))
);

create index on submissions (project_id);
create unique index submissions_one_pending_per_project on submissions (project_id)
    where status = 'pending';
create index submissions_review_queue on submissions (submitted_at)
    where status = 'pending';

-- +goose Down
drop table submissions;
drop type submission_status;
drop table project_hackatime_projects;
drop table projects;
