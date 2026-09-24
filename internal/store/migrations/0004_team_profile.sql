-- A team carries a picture and a paragraph about itself, which is what its
-- profile page is for: what the team does, how to reach it.
--
-- Like a user's bio, it is text nothing reads. No behaviour derives from
-- either, and a team without them is drawn as it always was.
ALTER TABLE teams ADD COLUMN avatar TEXT;
ALTER TABLE teams ADD COLUMN description TEXT;
