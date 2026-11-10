-- +goose Up

-- Two more things a reviewer writes, as on Ari, and both go into the Unified
-- YSWS justification: the technical features that account for the hours
-- (required to approve), and why fewer hours were approved than came in (a
-- "deflation", required whenever a stage cuts). Each stage's reason is kept on
-- its own timeline event; the ship keeps the latest.
alter table submissions
    add column technical_features text,
    add column deflation_reason text;
alter table ship_events add column deflation_reason text;

-- +goose Down
alter table ship_events drop column deflation_reason;
alter table submissions drop column technical_features, drop column deflation_reason;
