package database

import "context"

type Operator struct {
	ID      string
	Name    string
	OwnerID string
}

func (d *DB) OperatorByKeyHash(ctx context.Context, keyHash string) (*Operator, error) {
	var operator Operator
	var ownerID *string
	err := d.Pool.QueryRow(ctx, `SELECT id, name, owner_id FROM operators WHERE key_hash=$1`, keyHash).Scan(&operator.ID, &operator.Name, &ownerID)
	if ownerID != nil {
		operator.OwnerID = *ownerID
	}
	return &operator, err
}

func (d *DB) OperatorIDByOwner(ctx context.Context, ownerID string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `SELECT id FROM operators WHERE owner_id=$1`, ownerID).Scan(&id)
	return id, err
}
