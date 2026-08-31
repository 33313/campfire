-- A bucket carries a colour as well as an icon, so its tickets are recognisable
-- on a board at a glance rather than by reading a title on every card.
--
-- Like the icon it is buckets only, one of a fixed set, and nothing reads it or
-- derives behaviour from it. A bucket without one is drawn as it always was.
ALTER TABLE items ADD COLUMN color TEXT;
