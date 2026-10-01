-- CSQL writes an empty field as none, beside the other values (SPEC.md §9.2),
-- where its first draft wrote `is empty`. Saved views written in that draft
-- are rewritten; no released version wrote it.
UPDATE saved_views
   SET query = replace(replace(query, ' is not empty', ' != none'), ' is empty', ' = none')
 WHERE query LIKE '% is %empty%';

-- The date fields a query filters on.
CREATE INDEX items_created ON items (created_at);
CREATE INDEX items_started ON items (actual_start) WHERE actual_start IS NOT NULL;
CREATE INDEX items_completed ON items (actual_end) WHERE actual_end IS NOT NULL;
CREATE INDEX items_due ON items (planned_end) WHERE planned_end IS NOT NULL;
