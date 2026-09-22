package repository

import (
	"context"

	"gorm.io/gorm"
)

func RunInTx(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	return db.WithContext(ctx).Transaction(fn)
}
