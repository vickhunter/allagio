ALTER TABLE recommendations
DROP CONSTRAINT IF EXISTS recommendations_status_check;

ALTER TABLE recommendations
ADD CONSTRAINT recommendations_status_check
CHECK (status IN ('pending', 'approved', 'dismissed', 'inserted'));

CREATE OR REPLACE FUNCTION recommendation_status_transition_guard()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.status = OLD.status THEN
        RETURN NEW;
    END IF;

    IF OLD.status = 'pending' AND NEW.status IN ('approved', 'dismissed') THEN
        RETURN NEW;
    END IF;

    IF OLD.status = 'approved' AND NEW.status IN ('dismissed', 'inserted') THEN
        RETURN NEW;
    END IF;

    RAISE EXCEPTION 'invalid recommendation status transition: % -> %', OLD.status, NEW.status
        USING ERRCODE = '23514';
END;
$$;

DROP TRIGGER IF EXISTS trg_recommendation_status_transition_guard ON recommendations;

CREATE TRIGGER trg_recommendation_status_transition_guard
BEFORE UPDATE OF status ON recommendations
FOR EACH ROW
EXECUTE FUNCTION recommendation_status_transition_guard();
