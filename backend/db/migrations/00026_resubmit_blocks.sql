-- +goose Up

-- A hard reject, for fraud: the ship is rejected and its project can't be
-- shipped again until an admin allows it. (A soft reject is a plain
-- rejection: they change the project and ship it again.) resubmit_blocked_by
-- is who rejected it; resubmit_block_reason is why, for the admins.
alter table projects
    add column resubmit_blocked_at timestamptz,
    add column resubmit_blocked_by bigint references users (id),
    add column resubmit_block_reason text;

-- Marks the rejection that blocked it. It's still a rejection, so it counts
-- as one review like any other.
alter table ship_events add column blocks_resubmit boolean not null default false;

-- HQ or an admin put a rejected ship back in the review queue. Like a
-- comment, it isn't a review: it doesn't count toward anyone's pay. Nothing
-- here uses the new value, so it can be added in the same transaction.
alter type ship_event_kind add value if not exists 'reopened';

-- Every approved ship goes to the fraud check unless an admin turns it off.
alter table review_settings alter column fraud_check_everything set default true;
update review_settings set fraud_check_everything = true;

-- +goose Down
-- 'reopened' stays in ship_event_kind: Postgres can't drop an enum value.
alter table review_settings alter column fraud_check_everything set default false;
alter table ship_events drop column blocks_resubmit;
alter table projects
    drop column resubmit_blocked_at,
    drop column resubmit_blocked_by,
    drop column resubmit_block_reason;
