-- +goose Up

-- Whether reviewers with only the base Review permission see AI pre-reviews
-- (api/prereview.go). The fraud check, HQ and admins always do. Off to start.
alter table review_settings add column prereviews_for_reviewers boolean not null default false;

-- +goose Down

alter table review_settings drop column prereviews_for_reviewers;
