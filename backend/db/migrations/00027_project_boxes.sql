-- +goose Up

-- A per-project milestone counts one project's approved hours instead of
-- everything its owner has had approved: each project that passes it comes
-- with its own box. Each kind gets one milestone at a given number of hours.
alter table milestones add column per_project boolean not null default false;
alter table milestones drop constraint milestones_approved_seconds_key;
alter table milestones add constraint milestones_approved_seconds_key unique (per_project, approved_seconds);

-- The project a box came with. Only shipped projects earn one, and those
-- can't be deleted; a dev-mode box on a project that's deleted becomes a
-- plain box.
alter table boxes add column project_id bigint references projects (id) on delete set null;

-- One box per person for a milestone, or one per project for a per-project
-- milestone.
alter table boxes drop constraint boxes_user_id_milestone_id_key;
create unique index boxes_user_id_milestone_id_key on boxes (user_id, milestone_id) where project_id is null;
create unique index boxes_project_id_milestone_id_key on boxes (project_id, milestone_id) where project_id is not null;

-- +goose StatementBegin
create or replace function audit_box_granted() returns trigger language plpgsql as $$
begin
    insert into audit_log (kind, at, actor_id, user_id, details) values (
        'box.granted', new.granted_at, new.granted_by, new.user_id,
        jsonb_build_object(
            'box_id', new.id,
            'box_type_id', new.box_type_id,
            'box_type', (select name from box_types where id = new.box_type_id),
            'source', case when new.milestone_id is not null then 'milestone' else 'admin' end,
            'milestone_id', new.milestone_id,
            'milestone', (select name from milestones where id = new.milestone_id),
            'project_id', new.project_id,
            'project', (select name from projects where id = new.project_id)));
    return null;
end;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
create or replace function audit_box_granted() returns trigger language plpgsql as $$
begin
    insert into audit_log (kind, at, actor_id, user_id, details) values (
        'box.granted', new.granted_at, new.granted_by, new.user_id,
        jsonb_build_object(
            'box_id', new.id,
            'box_type_id', new.box_type_id,
            'box_type', (select name from box_types where id = new.box_type_id),
            'source', case when new.milestone_id is not null then 'milestone' else 'admin' end,
            'milestone_id', new.milestone_id,
            'milestone', (select name from milestones where id = new.milestone_id)));
    return null;
end;
$$;
-- +goose StatementEnd

-- Fails if someone opened the boxes of two of their projects from one
-- milestone: those can't go back to one box per person.
delete from boxes where project_id is not null and milestone_id is not null and opened_at is null;
drop index boxes_project_id_milestone_id_key;
drop index boxes_user_id_milestone_id_key;
alter table boxes drop column project_id;
alter table boxes add constraint boxes_user_id_milestone_id_key unique (user_id, milestone_id);
delete from milestones where per_project and not exists (select 1 from boxes where boxes.milestone_id = milestones.id);
alter table milestones drop constraint milestones_approved_seconds_key;
alter table milestones add constraint milestones_approved_seconds_key unique (approved_seconds);
alter table milestones drop column per_project;
