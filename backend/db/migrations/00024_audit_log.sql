-- +goose Up

-- Superadmins are admins who can also read the audit log. Who they are is a
-- list of Slack IDs in main.go, applied at startup and at every sign-in; the
-- app never lets anyone grant it. Nothing here uses the new value, so it can
-- be added in the same transaction.
alter type user_role add value if not exists 'superadmin';

-- Everything that happens on the platform, for superadmins
-- (GET /api/superadmin/audit-log). Rows are never updated or deleted.
--
-- kind says what happened, as a group and an event: 'special_stamp.issued'
-- is in the 'special_stamp' group, so filtering on either finds it.
--   request              a write request to /api or /auth (package audit)
--   sign_in              someone signed in (auth.callback, auth.DevLogin)
--   special_stamp.issued a special stamp from a box, an admin or a claim link
--   special_stamp.used   a special stamp put on a submission
--   box.granted          a box at a milestone or from an admin
--   stamps.<kind>        a stamp_ledger entry: payout, bonus, purchase,
--                        refund or adjustment
--   role.changed         someone's role changed, by anyone or anything
--   order.<status>       an order became fulfilled or cancelled (by an admin,
--                        the Airtable sync, or a price lock refund)
-- Everything but requests and sign-ins is written by the triggers below, so
-- it's recorded whichever code path made the change.
create table audit_log (
    id bigint generated always as identity primary key,
    kind text not null check (kind ~ '^[a-z_]+(\.[a-z_]+)?$'),
    at timestamptz not null default now(),
    -- Who did it: whoever was signed in, the admin who gave it, the person
    -- who opened the box. Null for nobody signed in (Sidekick, a failed
    -- sign-in) and for things the server did on its own or that the trigger
    -- can't attribute (see the request rows around it).
    actor_id bigint references users (id),
    -- Who it happened to: whose stamps, box, role or order it is, or the
    -- {userID} a request was about.
    user_id bigint references users (id),
    -- What happened, by kind. Names (stamp types, box types, shop items) are
    -- copied in, so renaming them later doesn't rewrite history.
    details jsonb not null default '{}'
);

create index on audit_log (kind, id);
create index on audit_log (actor_id, id);
create index on audit_log (user_id, id);

-- +goose StatementBegin
create function reject_audit_log_change() returns trigger language plpgsql as $$
begin
    raise exception 'audit_log is append-only';
end;
$$;
-- +goose StatementEnd

create trigger audit_log_append_only before update or delete on audit_log
    for each row execute function reject_audit_log_change();

-- +goose StatementBegin
create function audit_special_stamp_issued() returns trigger language plpgsql as $$
declare
    stamp_type special_stamp_types;
    box boxes;
    box_type_name text;
    milestone_name text;
begin
    select * into stamp_type from special_stamp_types where id = new.special_stamp_type_id;
    if new.box_id is not null then
        select * into box from boxes where id = new.box_id;
        select name into box_type_name from box_types where id = box.box_type_id;
        select name into milestone_name from milestones where id = box.milestone_id;
    end if;
    insert into audit_log (kind, at, actor_id, user_id, details) values (
        'special_stamp.issued', new.acquired_at,
        -- The admin who gave it, or the person who opened the box or used the
        -- claim link.
        coalesce(new.granted_by, new.user_id), new.user_id,
        jsonb_build_object(
            'special_stamp_id', new.id,
            'special_stamp_type_id', stamp_type.id,
            'special_stamp_type', stamp_type.name,
            'rarity', stamp_type.rarity,
            'source', case when new.box_id is not null then 'box' when new.claimed then 'claim' else 'admin' end,
            'box_id', new.box_id,
            'box_type_id', box.box_type_id,
            'box_type', box_type_name,
            -- How they got the box: a milestone, or null for an admin's gift.
            'box_milestone', milestone_name,
            'box_granted_by', box.granted_by));
    return null;
end;
$$;
-- +goose StatementEnd

create trigger special_stamps_audit_issued after insert on special_stamps
    for each row execute function audit_special_stamp_issued();

-- +goose StatementBegin
create function audit_special_stamp_used() returns trigger language plpgsql as $$
begin
    insert into audit_log (kind, actor_id, user_id, details)
    select 'special_stamp.used', new.user_id, new.user_id, jsonb_build_object(
        'special_stamp_id', new.id,
        'special_stamp_type_id', special_stamp_types.id,
        'special_stamp_type', special_stamp_types.name,
        'effect', special_stamp_types.effect,
        'submission_id', new.submission_id,
        'project_id', projects.id,
        'project', projects.name)
    from special_stamp_types, submissions
    join projects on projects.id = submissions.project_id
    where special_stamp_types.id = new.special_stamp_type_id and submissions.id = new.submission_id;
    return null;
end;
$$;
-- +goose StatementEnd

create trigger special_stamps_audit_used after update of submission_id on special_stamps
    for each row when (old.submission_id is null and new.submission_id is not null)
    execute function audit_special_stamp_used();

-- +goose StatementBegin
create function audit_box_granted() returns trigger language plpgsql as $$
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

create trigger boxes_audit_granted after insert on boxes
    for each row execute function audit_box_granted();

-- Runs after stamp_ledger_apply (triggers go in name order), so users.stamps
-- is already the balance after this entry.
-- +goose StatementBegin
create function audit_stamp_entry() returns trigger language plpgsql as $$
declare
    ship submissions;
begin
    if new.submission_id is not null then
        select * into ship from submissions where id = new.submission_id;
    end if;
    insert into audit_log (kind, at, actor_id, user_id, details) values (
        'stamps.' || new.kind, new.created_at,
        case new.kind
            when 'adjustment' then new.created_by
            when 'purchase' then new.user_id
            -- Payouts and bonuses: whoever signed the ship off.
            else coalesce(ship.hq_reviewer_id, ship.fraud_reviewer_id, ship.reviewer_id)
        end,
        new.user_id,
        jsonb_build_object(
            'ledger_id', new.id,
            'amount', new.amount,
            'balance', (select stamps from users where id = new.user_id),
            'note', new.note,
            'submission_id', new.submission_id,
            'project', (select name from projects where id = ship.project_id),
            'approved_seconds', ship.approved_seconds,
            'order_id', new.order_id,
            'item', (select shop_items.name from orders join shop_items on shop_items.id = orders.shop_item_id
                where orders.id = new.order_id)));
    return null;
end;
$$;
-- +goose StatementEnd

create trigger stamp_ledger_audit after insert on stamp_ledger
    for each row execute function audit_stamp_entry();

-- Who changed it isn't on the row; the request rows at the same moment say.
-- +goose StatementBegin
create function audit_role_change() returns trigger language plpgsql as $$
begin
    insert into audit_log (kind, user_id, details) values (
        'role.changed', new.id, jsonb_build_object('from', old.role, 'to', new.role));
    return null;
end;
$$;
-- +goose StatementEnd

create trigger users_audit_role after update of role on users
    for each row when (old.role is distinct from new.role)
    execute function audit_role_change();

-- +goose StatementBegin
create function audit_order_status() returns trigger language plpgsql as $$
begin
    insert into audit_log (kind, user_id, details) values (
        'order.' || new.status, new.user_id, jsonb_build_object(
            'order_id', new.id,
            'item', (select name from shop_items where id = new.shop_item_id),
            'price', new.price,
            'from', old.status,
            'to', new.status));
    return null;
end;
$$;
-- +goose StatementEnd

create trigger orders_audit_status after update of status on orders
    for each row when (old.status is distinct from new.status)
    execute function audit_order_status();

-- +goose Down

drop trigger orders_audit_status on orders;
drop function audit_order_status;
drop trigger users_audit_role on users;
drop function audit_role_change;
drop trigger stamp_ledger_audit on stamp_ledger;
drop function audit_stamp_entry;
drop trigger boxes_audit_granted on boxes;
drop function audit_box_granted;
drop trigger special_stamps_audit_used on special_stamps;
drop function audit_special_stamp_used;
drop trigger special_stamps_audit_issued on special_stamps;
drop function audit_special_stamp_issued;
drop table audit_log;
drop function reject_audit_log_change;
-- Superadmins go back to being admins. Postgres can't drop a value from an
-- enum; left in, it's harmless, and migration 1's down drops the whole type.
update users set role = 'admin' where role = 'superadmin';
