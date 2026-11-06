-- +goose Up

-- Golden Spike bonuses are ledger entries tied to the ship they were paid
-- for, alongside its payout. (A separate migration from 00009 because a new
-- enum value can't be used in the transaction that adds it.)
alter table stamp_ledger
    drop constraint stamp_ledger_check,
    add constraint stamp_ledger_submission_kinds
        check ((kind in ('payout', 'bonus')) = (submission_id is not null)),
    drop constraint stamp_ledger_check4,
    add constraint stamp_ledger_positive_kinds
        check (kind not in ('payout', 'refund', 'bonus') or amount > 0);

drop index stamp_ledger_one_payout_per_submission;
create unique index stamp_ledger_one_entry_per_submission_kind on stamp_ledger (submission_id, kind);

-- +goose Down

delete from stamp_ledger where kind = 'bonus';
drop index stamp_ledger_one_entry_per_submission_kind;
create unique index stamp_ledger_one_payout_per_submission on stamp_ledger (submission_id);
alter table stamp_ledger
    drop constraint stamp_ledger_positive_kinds,
    add constraint stamp_ledger_check4 check (kind not in ('payout', 'refund') or amount > 0),
    drop constraint stamp_ledger_submission_kinds,
    add constraint stamp_ledger_check check ((kind = 'payout') = (submission_id is not null));
