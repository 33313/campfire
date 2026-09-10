-- A user carries a short piece of prose about themselves, which is what a
-- profile page is for: how to reach them, what they work on, whatever they
-- want the rest of the instance to know.
--
-- It is text nothing reads. No behaviour derives from it, nothing is parsed
-- out of it, and an account without one is drawn as it always was.
ALTER TABLE users ADD COLUMN bio TEXT;
