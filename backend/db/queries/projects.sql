-- name: CreateProject :one
insert into projects (user_id, name, description, repo_url, demo_url, screenshot_url)
values ($1, $2, $3, $4, $5, $6)
returning *;

-- name: GetProject :one
select * from projects where id = $1 and user_id = $2;

-- name: LockProject :one
select * from projects where id = $1 and user_id = $2 for update;

-- name: ListProjects :many
select * from projects where user_id = $1 order by id desc;

-- name: UpdateProject :one
update projects set name = $3, description = $4, repo_url = $5, demo_url = $6, screenshot_url = $7
where id = $1 and user_id = $2
returning *;

-- name: DeleteProject :exec
delete from projects where id = $1 and user_id = $2;

-- name: ListHackatimeLinks :many
select * from project_hackatime_projects where user_id = $1 order by hackatime_project;

-- name: ListProjectHackatimeLinks :many
select hackatime_project from project_hackatime_projects where project_id = $1 order by hackatime_project;

-- name: LinkHackatimeProject :exec
insert into project_hackatime_projects (user_id, hackatime_project, project_id) values ($1, $2, $3);

-- name: UnlinkHackatimeProject :exec
delete from project_hackatime_projects where user_id = $1 and hackatime_project = $2;
