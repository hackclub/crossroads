-- name: CreateAnnouncement :one
insert into announcements (doc, blocks, text, buttons, pings, created_by)
values ($1, $2, $3, $4, $5, $6)
returning *;

-- name: GetAnnouncement :one
select * from announcements where id = $1;

-- name: LockAnnouncement :one
-- While posting, editing or deleting it, so two superadmins can't post it
-- twice.
select * from announcements where id = $1 for update;

-- name: UpdateAnnouncement :one
update announcements set doc = $2, blocks = $3, text = $4, buttons = $5, pings = $6
where id = $1
returning *;

-- name: MarkAnnouncementPosted :one
update announcements
set posted_by = $2, posted_at = now(), channel_id = $3, message_ts = $4, permalink = $5
where id = $1 and posted_at is null
returning *;

-- name: MarkAnnouncementDeleted :one
update announcements set deleted_at = now() where id = $1 returning *;

-- name: DeleteAnnouncementDraft :execrows
delete from announcements where id = $1 and posted_at is null;

-- name: ListAnnouncements :many
-- Drafts first, then the newest posts, without their content, and with how
-- many clicks each has had and from how many people.
select announcements.id, announcements.text, announcements.pings,
    announcements.created_at, announcements.updated_at, announcements.posted_at,
    announcements.channel_id, announcements.permalink, announcements.deleted_at,
    creators.name as created_by_name, posters.name as posted_by_name,
    coalesce(clicks.clicks, 0)::integer as clicks, coalesce(clicks.people, 0)::integer as people
from announcements
join users as creators on creators.id = announcements.created_by
left join users as posters on posters.id = announcements.posted_by
left join lateral (
    select count(*) as clicks, count(distinct slack_clicks.slack_id) as people
    from slack_clicks
    where slack_clicks.channel_id = announcements.channel_id
        and slack_clicks.message_ts = announcements.message_ts
) as clicks on true
order by announcements.posted_at desc nulls first, announcements.id desc;

-- name: ListAnnouncementClicks :many
-- Clicks on a posted message's buttons, by person and button, earliest first,
-- with the person's account if they have one.
select slack_clicks.slack_id, slack_clicks.button,
    count(*)::integer as clicks, min(slack_clicks.clicked_at)::timestamptz as first_click,
    users.id as user_id, users.name as user_name, users.created_at as signed_up_at
from slack_clicks
left join users on users.slack_id = slack_clicks.slack_id
where slack_clicks.channel_id = sqlc.arg(channel_id) and slack_clicks.message_ts = sqlc.arg(message_ts)
group by slack_clicks.slack_id, slack_clicks.button, users.id
order by first_click, slack_clicks.slack_id;

-- name: CountStampClaimsSince :one
-- Stamps of a type claimed with its claim link since a time, for an
-- announcement's claim button.
select count(*)::integer from special_stamps
where special_stamp_type_id = $1 and claimed and acquired_at >= sqlc.arg(since);
