-- +goose Up

-- A hard reject forfeits every ticket its project has earned. Each of its
-- paid ships gets a 'forfeit' entry taking back what it still holds (its
-- payout and bonus, less what's already been taken back and not given back).
-- Undoing the hard reject can give them back, as a 'restore' entry per ship.
-- created_by is whoever hard-rejected it or gave them back. A ship can be
-- forfeited and restored more than once, so only payouts and bonuses stay one
-- per ship.
alter table stamp_ledger
    drop constraint stamp_ledger_submission_kinds,
    add constraint stamp_ledger_submission_kinds
        check ((kind in ('payout', 'bonus', 'forfeit', 'restore')) = (submission_id is not null)),
    drop constraint stamp_ledger_check2,
    add constraint stamp_ledger_adjustment_by check (kind <> 'adjustment' or created_by is not null),
    add constraint stamp_ledger_created_by_kinds check (created_by is null or kind in ('adjustment', 'forfeit', 'restore')),
    drop constraint stamp_ledger_positive_kinds,
    add constraint stamp_ledger_positive_kinds
        check (kind not in ('payout', 'refund', 'bonus', 'restore') or amount > 0),
    add constraint stamp_ledger_forfeit_negative check (kind <> 'forfeit' or amount < 0);
drop index stamp_ledger_one_entry_per_submission_kind;
create unique index stamp_ledger_one_entry_per_submission_kind on stamp_ledger (submission_id, kind)
    where kind in ('payout', 'bonus');
create index on stamp_ledger (submission_id);

-- Forfeits and restores are by whoever did them, like adjustments.
-- +goose StatementBegin
create or replace function audit_stamp_entry() returns trigger language plpgsql as $$
declare
    ship submissions;
begin
    if new.submission_id is not null then
        select * into ship from submissions where id = new.submission_id;
    end if;
    insert into audit_log (kind, at, actor_id, user_id, details) values (
        'stamps.' || new.kind, new.created_at,
        case
            when new.kind in ('adjustment', 'forfeit', 'restore') then new.created_by
            when new.kind = 'purchase' then new.user_id
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

-- Projects hard-rejected before this forfeit theirs now.
insert into stamp_ledger (user_id, kind, amount, submission_id, created_by)
select stamp_ledger.user_id, 'forfeit'::stamp_entry_kind, -sum(stamp_ledger.amount), stamp_ledger.submission_id,
    projects.resubmit_blocked_by
from stamp_ledger
join submissions on submissions.id = stamp_ledger.submission_id
join projects on projects.id = submissions.project_id
where projects.resubmit_blocked_at is not null
group by stamp_ledger.user_id, stamp_ledger.submission_id, projects.resubmit_blocked_by
having sum(stamp_ledger.amount) > 0;

-- +goose Down
-- The ledger is append-only, so once a forfeit or restore has been made this
-- can't go down: the old checks below refuse those rows.
-- +goose StatementBegin
create or replace function audit_stamp_entry() returns trigger language plpgsql as $$
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
drop index stamp_ledger_submission_id_idx;
drop index stamp_ledger_one_entry_per_submission_kind;
create unique index stamp_ledger_one_entry_per_submission_kind on stamp_ledger (submission_id, kind);
alter table stamp_ledger
    drop constraint stamp_ledger_forfeit_negative,
    drop constraint stamp_ledger_positive_kinds,
    add constraint stamp_ledger_positive_kinds
        check (kind not in ('payout', 'refund', 'bonus') or amount > 0),
    drop constraint stamp_ledger_created_by_kinds,
    drop constraint stamp_ledger_adjustment_by,
    add constraint stamp_ledger_check2 check ((kind = 'adjustment') = (created_by is not null)),
    drop constraint stamp_ledger_submission_kinds,
    add constraint stamp_ledger_submission_kinds
        check ((kind in ('payout', 'bonus')) = (submission_id is not null));
