package services

import (
	"context"
	"encoding/json"
	"log"

	"affilio-api/internal/database"
)

// AuditLogger writes security audit events to the database asynchronously.
type AuditLogger struct {
	DB *database.DB
	ch chan database.AuditEvent
}

func NewAuditLogger(db *database.DB) *AuditLogger {
	a := &AuditLogger{
		DB: db,
		ch: make(chan database.AuditEvent, 256),
	}
	go a.drain()
	return a
}

func (a *AuditLogger) drain() {
	for ev := range a.ch {
		if err := a.DB.InsertAuditEvent(context.Background(), ev); err != nil {
			detailsJSON, _ := json.Marshal(ev.Details)
			log.Printf("audit: failed to persist event type=%s ip=%s err=%v details=%s", ev.EventType, ev.IPAddress, err, detailsJSON)
		}
	}
}

func (a *AuditLogger) Log(ev database.AuditEvent) {
	select {
	case a.ch <- ev:
	default:
		log.Printf("audit: channel full, dropping event type=%s ip=%s", ev.EventType, ev.IPAddress)
	}
}

func (a *AuditLogger) Close() {
	close(a.ch)
}
