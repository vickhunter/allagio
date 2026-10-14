-- Remember the mode a kill switch replaced so a signed-in human can turn it back off.
ALTER TABLE approval_policies ADD COLUMN kill_switch BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE approval_policies ADD COLUMN mode_before_kill TEXT;
ALTER TABLE approval_policies ADD CONSTRAINT approval_policies_mode_before_kill_check
    CHECK (mode_before_kill IS NULL OR mode_before_kill IN ('manual', 'auto', 'yolo'));
