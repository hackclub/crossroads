-- +goose Up
create type stamp_entry_kind as enum ('payout', 'purchase', 'refund', 'adjustment');

-- Every change to anyone's stamps. Rows are never updated or deleted, and
-- inserting one updates users.stamps.
create table stamp_ledger (
    id bigint generated always as identity primary key,
    user_id bigint not null references users (id),
    kind stamp_entry_kind not null,
    amount integer not null check (amount <> 0),
    submission_id bigint references submissions (id),
    order_id bigint references orders (id),
    created_by bigint references users (id), -- the admin who made an adjustment
    note text,
    created_at timestamptz not null default now(),
    check ((kind = 'payout') = (submission_id is not null)),
    check ((kind in ('purchase', 'refund')) = (order_id is not null)),
    check ((kind = 'adjustment') = (created_by is not null)),
    check (kind <> 'adjustment' or note is not null),
    check (kind not in ('payout', 'refund') or amount > 0),
    check (kind <> 'purchase' or amount < 0)
);

create index on stamp_ledger (user_id);
create unique index stamp_ledger_one_payout_per_submission on stamp_ledger (submission_id);
create unique index stamp_ledger_one_entry_per_order_kind on stamp_ledger (order_id, kind);

-- Purchases can't take a balance below zero. Adjustments can, so an admin can
-- claw back stamps from a fraudulent project that have already been spent.
-- +goose StatementBegin
create function apply_stamp_entry() returns trigger language plpgsql as $$
declare
    balance integer;
begin
    update users set stamps = stamps + new.amount where id = new.user_id
        returning stamps into balance;
    if new.kind = 'purchase' and balance < 0 then
        raise exception 'not enough stamps'
            using errcode = 'check_violation', constraint = 'stamps_not_negative';
    end if;
    return null;
end;
$$;
-- +goose StatementEnd

create trigger stamp_ledger_apply after insert on stamp_ledger
    for each row execute function apply_stamp_entry();

-- +goose StatementBegin
create function reject_stamp_ledger_change() returns trigger language plpgsql as $$
begin
    raise exception 'stamp_ledger is append-only; add an adjustment instead';
end;
$$;
-- +goose StatementEnd

create trigger stamp_ledger_append_only before update or delete on stamp_ledger
    for each row execute function reject_stamp_ledger_change();

-- +goose Down
drop table stamp_ledger;
drop function reject_stamp_ledger_change;
drop function apply_stamp_entry;
drop type stamp_entry_kind;
