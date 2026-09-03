-- Audit events table for security logging
CREATE TABLE IF NOT EXISTS audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(50) NOT NULL,
    severity VARCHAR(10) DEFAULT 'info',
    actor VARCHAR(255),
    ip_address VARCHAR(45),
    endpoint VARCHAR(255),
    method VARCHAR(10),
    details JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_events_type_created ON audit_events(event_type, created_at);
CREATE INDEX IF NOT EXISTS idx_audit_events_ip ON audit_events(ip_address, created_at);
CREATE INDEX IF NOT EXISTS idx_audit_events_severity ON audit_events(severity, created_at);
