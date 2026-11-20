-- +goose Up

-- Paying reviewers per review: in tickets, as a stamp_ledger adjustment to
-- the reviewer, or in US dollars, which only keeps track (the money is sent
-- some other way, and only superadmins can see or record it). The reviews a
-- payment covers are listed in review_payment_events, so none can be paid
-- twice, in either. A review is a ship event of the kinds the review stats
-- count (approval, rejection, fraud_pass, final_approval, sent_back). Neither
-- table is ever changed after it's written.
create table review_payments (
    id bigint generated always as identity primary key,
    reviewer_id bigint not null references users (id),
    paid_by bigint not null references users (id),
    currency text not null check (currency in ('tickets', 'usd')),
    reviews integer not null check (reviews > 0),
    rate integer not null check (rate > 0), -- per review: tickets, or US cents
    amount integer not null,                -- tickets, or US cents
    ledger_id bigint unique references stamp_ledger (id), -- tickets only
    note text not null,
    created_at timestamptz not null default now(),
    check (amount = reviews * rate),
    check ((currency = 'tickets') = (ledger_id is not null)),
    check (reviewer_id <> paid_by)
);

create index on review_payments (reviewer_id);

create table review_payment_events (
    ship_event_id bigint primary key references ship_events (id),
    payment_id bigint not null references review_payments (id)
);

create index on review_payment_events (payment_id);

-- +goose StatementBegin
create function reject_review_payment_change() returns trigger language plpgsql as $$
begin
    raise exception 'review payments are never changed';
end;
$$;
-- +goose StatementEnd

create trigger review_payments_append_only before update or delete on review_payments
    for each row execute function reject_review_payment_change();
create trigger review_payment_events_append_only before update or delete on review_payment_events
    for each row execute function reject_review_payment_change();

-- The audit log (migration 24) gets every payment as review_pay.tickets or
-- review_pay.usd. A ticket payment's stamps.adjustment is there too.
-- +goose StatementBegin
create function audit_review_payment() returns trigger language plpgsql as $$
begin
    insert into audit_log (kind, at, actor_id, user_id, details) values (
        'review_pay.' || new.currency, new.created_at, new.paid_by, new.reviewer_id,
        jsonb_build_object(
            'review_payment_id', new.id,
            'reviews', new.reviews,
            'rate', new.rate,
            'amount', new.amount,
            'note', new.note));
    return null;
end;
$$;
-- +goose StatementEnd

create trigger review_payments_audit after insert on review_payments
    for each row execute function audit_review_payment();

-- +goose Down
drop table review_payment_events;
drop table review_payments;
drop function audit_review_payment();
drop function reject_review_payment_change();
