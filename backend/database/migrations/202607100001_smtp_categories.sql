-- +goose Up
ALTER TABLE smtp_settings
	ADD COLUMN categories TEXT[] NOT NULL DEFAULT ARRAY['alerts']::TEXT[];

-- +goose Down
ALTER TABLE smtp_settings
	DROP COLUMN categories;
