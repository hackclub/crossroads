-- name: RecordSlackClick :exec
insert into slack_clicks (slack_id, button, channel_id, message_ts) values ($1, $2, $3, $4);
