-- +goose Up

-- A reviewer can approve more hours than Hackatime tracked (time Hackatime
-- couldn't see, like soldering or design work), but only with a reason, which
-- HQ sees before signing off and which goes to the Unified YSWS DB.
alter table submissions
    add column override_reason text,
    drop constraint submissions_approved_within_new,
    add constraint submissions_approved_within_new
        check (approved_seconds >= 0 and (approved_seconds <= new_seconds or override_reason is not null));

-- +goose Down
update submissions set approved_seconds = new_seconds where approved_seconds > new_seconds;
alter table submissions
    drop constraint submissions_approved_within_new,
    drop column override_reason,
    add constraint submissions_approved_within_new check (approved_seconds between 0 and new_seconds);
