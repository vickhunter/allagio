package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Telegram link codes are one-time, short-lived secrets an owner generates in
// the workspace and sends to the bot as /link <code>. Only the hash is stored.
func (d *DB) CreateTelegramLinkCode(ctx context.Context, ownerID, codeHash string, expires time.Time) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// One live code per owner: a fresh request replaces the old one.
	if _, err = tx.Exec(ctx, `DELETE FROM owner_telegram_codes WHERE owner_id=$1`, ownerID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO owner_telegram_codes(code_hash,owner_id,expires_at) VALUES($1,$2,$3)`, codeHash, ownerID, expires); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ClaimTelegramCode deletes the code row and returns the owner it was issued
// for. ok is false when the code is unknown or expired.
func (d *DB) ClaimTelegramCode(ctx context.Context, codeHash string) (ownerID string, ok bool, err error) {
	err = d.Pool.QueryRow(ctx, `DELETE FROM owner_telegram_codes WHERE code_hash=$1 AND expires_at>NOW() RETURNING owner_id::text`, codeHash).Scan(&ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return ownerID, true, nil
}

func (d *DB) BindTelegramChat(ctx context.Context, ownerID string, chatID int64, username string) error {
	_, err := d.Pool.Exec(ctx, `UPDATE owners SET telegram_chat_id=$2, telegram_username=$3 WHERE id=$1`, ownerID, chatID, username)
	return err
}

// OwnerByTelegramChat resolves a bound chat back to its owner; nil when the
// chat is not linked (e.g. after /unlink).
func (d *DB) OwnerByTelegramChat(ctx context.Context, chatID int64) (*Owner, error) {
	var o Owner
	err := d.Pool.QueryRow(ctx, `SELECT id::text, COALESCE(issuer,''), COALESCE(subject,''), COALESCE(email,''), COALESCE(name,''), COALESCE(site_id::text,''), COALESCE(maker_id::text,''), COALESCE(source,'')
FROM owners WHERE telegram_chat_id=$1`, chatID).Scan(&o.ID, &o.Issuer, &o.Subject, &o.Email, &o.Name, &o.SiteID, &o.MakerID, &o.Source)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// OwnerTelegram returns the chat currently bound to an owner; chatID 0 means
// not linked.
func (d *DB) OwnerTelegram(ctx context.Context, ownerID string) (chatID int64, username string, err error) {
	err = d.Pool.QueryRow(ctx, `SELECT COALESCE(telegram_chat_id,0), telegram_username FROM owners WHERE id=$1`, ownerID).Scan(&chatID, &username)
	return chatID, username, err
}

func (d *DB) UnlinkTelegram(ctx context.Context, ownerID string) error {
	_, err := d.Pool.Exec(ctx, `UPDATE owners SET telegram_chat_id=NULL, telegram_username='' WHERE id=$1`, ownerID)
	return err
}
