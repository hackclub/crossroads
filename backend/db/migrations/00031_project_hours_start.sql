-- +goose Up

-- When a project's Hackatime time starts counting, if not from the program's
-- start (hoursStart in main.go). On Nov 19 that moved back from Oct 30 to
-- Aug 1, but not for projects that already had a ship approved or in review:
-- their ships were weighed on time since Oct 30, so they keep counting from
-- there, and time before it can't be shipped on top of what was approved.
-- Projects never shipped, or only ever rejected, count from Aug 1.
alter table projects add column hours_start timestamptz;

update projects set hours_start = '2026-10-30 00:00:00+00'
where exists (
    select 1 from submissions
    where submissions.project_id = projects.id and submissions.status <> 'rejected'
);

-- +goose Down
alter table projects drop column hours_start;
