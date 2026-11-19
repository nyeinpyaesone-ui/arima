package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// PostgresAuthorizer resolves permissions from tenant-scoped membership and role tables.
// It deliberately starts a transaction and sets app.tenant_id before querying so the
// same forced-RLS boundary protects authorization data as business data.
type PostgresAuthorizer struct {
	db *sql.DB
}

func NewPostgresAuthorizer(db *sql.DB) (*PostgresAuthorizer, error) {
	if db == nil {
		return nil, errors.New("authorization database connection is required")
	}
	return &PostgresAuthorizer{db: db}, nil
}

func (a *PostgresAuthorizer) Authorize(ctx context.Context, principal Principal, permission string) error {
	if a == nil || a.db == nil {
		return errors.New("authorization database is not initialized")
	}
	subject := strings.TrimSpace(principal.Subject)
	tenantID := strings.TrimSpace(principal.TenantID)
	permission = strings.TrimSpace(permission)
	if subject == "" || len(subject) > 255 || tenantID == "" || permission == "" || len(permission) > 129 {
		return ErrPermissionDenied
	}
	tx, err := a.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("begin authorization transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return fmt.Errorf("set authorization tenant context: %w", err)
	}
	var allowed bool
	err = tx.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM tenant_memberships membership
    JOIN tenant_role_permissions role_permission
      ON role_permission.tenant_id = membership.tenant_id
     AND role_permission.role_key = membership.role_key
    WHERE membership.tenant_id = $1::uuid
      AND membership.subject = $2
      AND membership.status = 'active'
      AND role_permission.permission = $3
)`, tenantID, subject, permission).Scan(&allowed)
	if err != nil {
		return fmt.Errorf("evaluate tenant permission: %w", err)
	}
	if !allowed {
		return ErrPermissionDenied
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit authorization transaction: %w", err)
	}
	return nil
}
