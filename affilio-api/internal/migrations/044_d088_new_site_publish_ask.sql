-- D-088 new sites ask before a placement is published. Apply by hand.
-- AUTO_MIGRATE stays off. Safe to run more than once.
-- This changes the column default only. It does not rewrite existing rows.
-- A site with no agent_step_modes row still reads as auto in the API.
-- New site creation writes placement_publish ask itself, including before
-- this file is applied. Turning CLAIMS_CHECK to real does not change that write.

ALTER TABLE agent_step_modes ALTER COLUMN placement_publish SET DEFAULT 'ask';
