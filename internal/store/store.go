package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"
	_ "modernc.org/sqlite"
)

type DB struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db := &DB{db: sqlDB}
	if err := db.init(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() error {
	return db.db.Close()
}

func (db *DB) UpsertProducts(ctx context.Context, products []appsumo.Product) error {
	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `insert into products (
		id, uuid, invoice_uuid, name, slug, status, plan_name, support_email,
		purchase_date, redeem_date, raw_json, updated_at
	) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	on conflict(uuid) do update set
		id=excluded.id,
		invoice_uuid=excluded.invoice_uuid,
		name=excluded.name,
		slug=excluded.slug,
		status=excluded.status,
		plan_name=excluded.plan_name,
		support_email=excluded.support_email,
		purchase_date=excluded.purchase_date,
		redeem_date=excluded.redeem_date,
		raw_json=excluded.raw_json,
		updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	updatedAt := time.Now().UTC().Format(time.RFC3339)
	for _, product := range products {
		raw, marshalErr := json.Marshal(product)
		if marshalErr != nil {
			err = marshalErr
			return err
		}
		if _, err = stmt.ExecContext(ctx,
			fmt.Sprint(product.ID),
			product.UUID,
			product.InvoiceUUID,
			product.Name,
			product.Slug,
			product.Status,
			product.PlanName,
			product.SupportEmail,
			product.PurchaseDate,
			stringValue(product.RedeemDate),
			string(raw),
			updatedAt,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func (db *DB) SearchProducts(ctx context.Context, query string) ([]appsumo.Product, error) {
	like := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	rows, err := db.db.QueryContext(ctx, `select raw_json from products
		where lower(name) like ?
		   or lower(slug) like ?
		   or lower(status) like ?
		   or lower(plan_name) like ?
		   or lower(support_email) like ?
		order by name`, like, like, like, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var products []appsumo.Product
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var product appsumo.Product
		if err := json.Unmarshal([]byte(raw), &product); err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

func (db *DB) QueryReadOnly(ctx context.Context, query string) ([]map[string]string, error) {
	if err := validateReadOnlyQuery(query); err != nil {
		return nil, err
	}
	conn, err := db.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "pragma query_only = on"); err != nil {
		return nil, fmt.Errorf("enable read-only sqlite mode: %w", err)
	}
	out, queryErr := queryRows(ctx, conn, query)
	_, resetErr := conn.ExecContext(context.Background(), "pragma query_only = off")
	if queryErr != nil {
		return nil, queryErr
	}
	if resetErr != nil {
		return nil, fmt.Errorf("reset read-only sqlite mode: %w", resetErr)
	}
	return out, nil
}

func queryRows(ctx context.Context, conn *sql.Conn, query string) ([]map[string]string, error) {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	columns, err := rows.Columns()
	if err != nil {
		_ = rows.Close()
		return nil, err
	}
	var out []map[string]string
	for rows.Next() {
		values := make([]sql.NullString, len(columns))
		scan := make([]any, len(columns))
		for i := range values {
			scan[i] = &values[i]
		}
		if err := rows.Scan(scan...); err != nil {
			return nil, err
		}
		row := make(map[string]string, len(columns))
		for i, column := range columns {
			if values[i].Valid {
				row[column] = values[i].String
			} else {
				row[column] = ""
			}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return out, nil
}

func (db *DB) init(ctx context.Context) error {
	_, err := db.db.ExecContext(ctx, `create table if not exists products (
		id integer,
		uuid text primary key,
		invoice_uuid text,
		name text not null,
		slug text,
		status text,
		plan_name text,
		support_email text,
		purchase_date text,
		redeem_date text,
		raw_json text not null,
		updated_at text not null
	)`)
	return err
}

func validateReadOnlyQuery(query string) error {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return fmt.Errorf("query is empty")
	}
	lower := strings.ToLower(trimmed)
	for _, blocked := range []string{"insert ", "update ", "delete ", "drop ", "alter ", "create ", "replace ", "pragma ", "attach ", "detach ", "vacuum "} {
		if strings.Contains(lower, blocked) || strings.HasPrefix(lower, strings.TrimSpace(blocked)) {
			return fmt.Errorf("only read-only select queries are allowed")
		}
	}
	if !strings.HasPrefix(lower, "select ") && lower != "select" {
		return fmt.Errorf("only select queries are allowed")
	}
	withoutFinalSemicolon := strings.TrimSuffix(lower, ";")
	if strings.Contains(withoutFinalSemicolon, ";") {
		return fmt.Errorf("multiple SQL statements are not allowed")
	}
	return nil
}
