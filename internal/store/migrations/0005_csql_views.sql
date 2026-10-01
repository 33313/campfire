-- A saved view is a CSQL query (SPEC.md §9.4). The query is stored in its
-- canonical form, naming what it references by #id, so a rename changes
-- nothing and a deleted referent matches nothing.
ALTER TABLE saved_views ADD COLUMN query TEXT NOT NULL DEFAULT '';

-- Each view's clauses, written as the query they always meant: the values of
-- one dimension any-of, labels every-one-of, dimensions and-ed. An assignee
-- of 0 was "unassigned".
WITH parts AS (
    SELECT view_id, CASE dimension
        WHEN 'text' THEN 'text ~ "' || replace(replace(max(text_value), '\', '\\'), '"', '\"') || '"'
        WHEN 'priority' THEN 'priority in (' || group_concat(ref_id, ', ') || ')'
        WHEN 'label' THEN group_concat('label = #' || ref_id, ' and ')
        WHEN 'assignee' THEN 'assignee in ('
            || group_concat(CASE WHEN ref_id = 0 THEN 'none' ELSE '#' || ref_id END, ', ') || ')'
        ELSE dimension || ' in (' || group_concat('#' || ref_id, ', ') || ')'
    END AS part
    FROM saved_view_clauses GROUP BY view_id, dimension
)
UPDATE saved_views SET query = IFNULL(
    (SELECT group_concat(part, ' and ') FROM parts WHERE parts.view_id = saved_views.id), '');

-- What is left of the clause rows is what each query names by id. It is kept
-- beside the text because a label a view names MUST NOT be reaped, and a
-- deleted referent is named wherever the view is offered.
DELETE FROM saved_view_clauses
 WHERE dimension IN ('text', 'priority') OR (dimension = 'assignee' AND ref_id = 0);
ALTER TABLE saved_view_clauses RENAME TO saved_view_refs;
ALTER TABLE saved_view_refs DROP COLUMN text_value;
