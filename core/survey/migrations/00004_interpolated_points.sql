-- +goose Up
-- A continuous walk records readings between the operator's marks and works out
-- where they were from the marks on either side. That is a claim about a
-- position, not a record of one, and a survey that could not tell the two apart
-- would assert a precision nobody measured. Pin-drops and imports are never
-- interpolated, which is why the default is false.
ALTER TABLE survey_points ADD COLUMN interpolated INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE survey_points DROP COLUMN interpolated;
