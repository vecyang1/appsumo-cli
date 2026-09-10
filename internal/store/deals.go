package store

// Catalog snapshot persistence.
//
// Two tables, because they answer different questions:
//
//   - deals holds the latest known state of every slug, for querying.
//   - deal_snapshots holds one immutable row per slug per run, so `deals diff`
//     can compare two points in time.
//
// codes_remaining and percent_claimed are nullable on purpose. The catalog sends
// 0 for "this deal does not sell codes" and -1 for "no percentage available";
// appsumo.Deal already turns those into nil, and storing nil as 0 here would put
// the confident-wrong-number back after one round trip. LoadSnapshot is asserted
// against exactly that in TestDealSnapshotRoundTripKeepsUnknownsUnknown.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"
)

const dealsSchema = `
create table if not exists deals (
	slug text primary key,
	id integer,
	name text,
	url text,
	price real,
	original_price real,
	plus_price real,
	is_free integer,
	listing_type text,
	card_description text,
	value_prop text,
	best_for text,
	alternative_to text,
	integrations text,
	subcategory text,
	codes_remaining integer,
	percent_claimed integer,
	start_date text,
	end_date text,
	timer_reason text,
	review_count integer,
	average_rating real,
	category text,
	deal_group text,
	raw_json text not null,
	snapshot_at text not null
);

create table if not exists deal_snapshots (
	snapshot_at text not null,
	slug text not null,
	name text,
	url text,
	price real,
	original_price real,
	codes_remaining integer,
	end_date text,
	timer_reason text,
	primary key (snapshot_at, slug)
);

create index if not exists deal_snapshots_at on deal_snapshots (snapshot_at);
`

// snapshotStampFormat is fixed-width with padded nanoseconds so that snapshot
// ids sort lexicographically, which is what SnapshotIDs orders by.
//
// Second precision was not enough: two syncs inside the same second produced the
// same id, and because (snapshot_at, slug) upserts, the second walk silently
// overwrote the first instead of being recorded. `deals diff` then reported
// "need two catalog snapshots, have 1". time.RFC3339Nano is also unusable here
// because it trims trailing zeros, which breaks string ordering — ".1Z" sorts
// after ".15Z".
const snapshotStampFormat = "2006-01-02T15:04:05.000000000Z"

// SaveDealSnapshot records a full catalog walk and returns the snapshot id.
//
// The caller passes the timestamp so a snapshot is stamped once, by the command
// that owns the run, rather than drifting across the rows of one walk.
func (db *DB) SaveDealSnapshot(ctx context.Context, snapshotAt time.Time, deals []appsumo.Deal) (string, error) {
	stamp := snapshotAt.UTC().Format(snapshotStampFormat)
	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	current, err := tx.PrepareContext(ctx, `insert into deals (
		slug, id, name, url, price, original_price, plus_price, is_free, listing_type,
		card_description, value_prop, best_for, alternative_to, integrations, subcategory,
		codes_remaining, percent_claimed, start_date, end_date, timer_reason,
		review_count, average_rating, category, deal_group, raw_json, snapshot_at
	) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	on conflict(slug) do update set
		id=excluded.id, name=excluded.name, url=excluded.url,
		price=excluded.price, original_price=excluded.original_price,
		plus_price=excluded.plus_price, is_free=excluded.is_free,
		listing_type=excluded.listing_type,
		card_description=excluded.card_description,
		value_prop=excluded.value_prop,
		best_for=excluded.best_for,
		alternative_to=excluded.alternative_to,
		integrations=excluded.integrations,
		subcategory=excluded.subcategory,
		codes_remaining=excluded.codes_remaining, percent_claimed=excluded.percent_claimed,
		start_date=excluded.start_date, end_date=excluded.end_date,
		timer_reason=excluded.timer_reason, review_count=excluded.review_count,
		average_rating=excluded.average_rating, category=excluded.category,
		deal_group=excluded.deal_group, raw_json=excluded.raw_json,
		snapshot_at=excluded.snapshot_at`)
	if err != nil {
		return "", err
	}
	defer current.Close()

	history, err := tx.PrepareContext(ctx, `insert into deal_snapshots (
		snapshot_at, slug, name, url, price, original_price, codes_remaining, end_date, timer_reason
	) values (?, ?, ?, ?, ?, ?, ?, ?, ?)
	on conflict(snapshot_at, slug) do update set
		name=excluded.name, url=excluded.url, price=excluded.price,
		original_price=excluded.original_price, codes_remaining=excluded.codes_remaining,
		end_date=excluded.end_date, timer_reason=excluded.timer_reason`)
	if err != nil {
		return "", err
	}
	defer history.Close()

	for _, deal := range deals {
		raw := string(deal.Raw)
		if raw == "" {
			encoded, marshalErr := json.Marshal(deal)
			if marshalErr != nil {
				return "", marshalErr
			}
			raw = string(encoded)
		}
		if _, err := current.ExecContext(ctx,
			deal.Slug, deal.ID, deal.Name, deal.URL,
			deal.Price, deal.OriginalPrice, deal.PlusPrice, deal.IsFree, deal.ListingType,
			deal.CardDescription, deal.ValueProp, stringSliceToJSON(deal.BestFor),
			stringSliceToJSON(deal.AlternativeTo), stringSliceToJSON(deal.Integrations), deal.Subcategory,
			nullableInt(deal.CodesRemaining), nullableInt(deal.PercentClaimed),
			deal.StartDate, deal.EndDate, deal.TimerReason,
			nullableInt(deal.ReviewCount), nullableFloat(deal.AverageRating),
			deal.Category, deal.Group, raw, stamp,
		); err != nil {
			return "", err
		}
		if _, err := history.ExecContext(ctx,
			stamp, deal.Slug, deal.Name, deal.URL,
			deal.Price, deal.OriginalPrice, nullableInt(deal.CodesRemaining),
			deal.EndDate, deal.TimerReason,
		); err != nil {
			return "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	committed = true
	return stamp, nil
}

// SnapshotIDs lists snapshot timestamps, newest first.
func (db *DB) SnapshotIDs(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 2
	}
	rows, err := db.db.QueryContext(ctx,
		`select snapshot_at from deal_snapshots group by snapshot_at order by snapshot_at desc limit ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stamps []string
	for rows.Next() {
		var stamp string
		if err := rows.Scan(&stamp); err != nil {
			return nil, err
		}
		stamps = append(stamps, stamp)
	}
	return stamps, rows.Err()
}

// LoadSnapshot reads one recorded catalog walk back as comparable deals.
func (db *DB) LoadSnapshot(ctx context.Context, snapshotAt string) ([]appsumo.Deal, error) {
	rows, err := db.db.QueryContext(ctx, `select slug, name, url, price, original_price,
		codes_remaining, end_date, timer_reason
		from deal_snapshots where snapshot_at = ? order by slug`, snapshotAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deals []appsumo.Deal
	for rows.Next() {
		var (
			deal      appsumo.Deal
			name      sql.NullString
			url       sql.NullString
			price     sql.NullFloat64
			original  sql.NullFloat64
			codes     sql.NullInt64
			endDate   sql.NullString
			timerText sql.NullString
		)
		if err := rows.Scan(&deal.Slug, &name, &url, &price, &original, &codes, &endDate, &timerText); err != nil {
			return nil, err
		}
		deal.Name = name.String
		deal.URL = url.String
		deal.Price = price.Float64
		deal.OriginalPrice = original.Float64
		deal.EndDate = endDate.String
		deal.TimerReason = timerText.String
		// A NULL here means the catalog never stated a stock count. Leaving the
		// pointer nil is what keeps it distinguishable from a real zero.
		if codes.Valid {
			remaining := int(codes.Int64)
			deal.CodesRemaining = &remaining
		}
		deals = append(deals, deal)
	}
	return deals, rows.Err()
}

// LatestDeals reads the current state of every known slug.
func (db *DB) LatestDeals(ctx context.Context) ([]appsumo.Deal, error) {
	stamps, err := db.SnapshotIDs(ctx, 1)
	if err != nil {
		return nil, err
	}
	if len(stamps) == 0 {
		return nil, nil
	}
	return db.LoadSnapshot(ctx, stamps[0])
}

// PruneSnapshots keeps the newest keep snapshots and deletes the rest.
func (db *DB) PruneSnapshots(ctx context.Context, keep int) (int, error) {
	if keep < 1 {
		return 0, fmt.Errorf("keep must be at least 1")
	}
	stamps, err := db.SnapshotIDs(ctx, keep+1)
	if err != nil {
		return 0, err
	}
	if len(stamps) <= keep {
		return 0, nil
	}
	cutoff := stamps[keep]
	result, err := db.db.ExecContext(ctx, `delete from deal_snapshots where snapshot_at <= ?`, cutoff)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	return int(affected), err
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func stringSliceToJSON(slice []string) string {
	if len(slice) == 0 {
		return ""
	}
	bytes, _ := json.Marshal(slice)
	return string(bytes)
}

func jsonToStringSlice(raw string) []string {
	if raw == "" {
		return nil
	}
	var res []string
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return nil
	}
	return res
}

// SearchDeals searches the latest deals snapshot in SQLite by query matching
// name, slug, card_description, value_prop, best_for, alternative_to, or category.
func (db *DB) SearchDeals(ctx context.Context, query string) ([]appsumo.Deal, error) {
	query = strings.TrimSpace(query)
	pattern := "%" + strings.ToLower(query) + "%"
	rows, err := db.db.QueryContext(ctx, `select
		slug, id, name, url, price, original_price, plus_price, is_free, listing_type,
		card_description, value_prop, best_for, alternative_to, integrations, subcategory,
		codes_remaining, percent_claimed, start_date, end_date, timer_reason,
		review_count, average_rating, category, deal_group, raw_json
		from deals
		where lower(name) like ? or lower(slug) like ? or lower(ifnull(card_description, '')) like ?
		   or lower(ifnull(value_prop, '')) like ? or lower(ifnull(best_for, '')) like ?
		   or lower(ifnull(alternative_to, '')) like ? or lower(ifnull(category, '')) like ?
		order by average_rating desc, review_count desc, name asc`,
		pattern, pattern, pattern, pattern, pattern, pattern, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDealsRows(rows)
}

// IdealDeals queries top deals from SQLite meeting minRating and minReviews,
// ordered by rating desc, review count desc.
func (db *DB) IdealDeals(ctx context.Context, minRating float64, minReviews int, limit int) ([]appsumo.Deal, error) {
	return db.IdealDealsQuery(ctx, appsumo.DealsQuery{
		MinRating:  minRating,
		MinReviews: minReviews,
		Limit:      limit,
	})
}

// IdealDealsQuery queries deals meeting DealsQuery criteria from SQLite,
// with all filters (minRating, minReviews, maxPrice, query, category) applied in SQL
// prior to ordering and limit.
func (db *DB) IdealDealsQuery(ctx context.Context, q appsumo.DealsQuery) ([]appsumo.Deal, error) {
	minRating := q.MinRating
	if minRating <= 0 {
		minRating = 4.5
	}
	minReviews := q.MinReviews
	if minReviews <= 0 {
		minReviews = 10
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}

	query := strings.TrimSpace(q.Query)
	pattern := "%" + strings.ToLower(query) + "%"

	sqlQuery := `select
		slug, id, name, url, price, original_price, plus_price, is_free, listing_type,
		card_description, value_prop, best_for, alternative_to, integrations, subcategory,
		codes_remaining, percent_claimed, start_date, end_date, timer_reason,
		review_count, average_rating, category, deal_group, raw_json
		from deals
		where average_rating >= ? and review_count >= ?`
	args := []any{minRating, minReviews}

	if query != "" {
		sqlQuery += ` and (lower(name) like ? or lower(slug) like ? or lower(ifnull(card_description, '')) like ?
		               or lower(ifnull(value_prop, '')) like ? or lower(ifnull(best_for, '')) like ?
		               or lower(ifnull(alternative_to, '')) like ? or lower(ifnull(category, '')) like ?)`
		args = append(args, pattern, pattern, pattern, pattern, pattern, pattern, pattern)
	}
	if q.Category != "" {
		sqlQuery += ` and (lower(category) = lower(?) or lower(category) like ?)`
		catPattern := "%" + strings.ToLower(strings.TrimSpace(q.Category)) + "%"
		args = append(args, q.Category, catPattern)
	}
	if q.MaxPrice > 0 {
		sqlQuery += ` and price <= ?`
		args = append(args, q.MaxPrice)
	}

	sqlQuery += ` order by average_rating desc, review_count desc, price asc limit ?`
	args = append(args, limit)

	rows, err := db.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDealsRows(rows)
}

func scanDealsRows(rows *sql.Rows) ([]appsumo.Deal, error) {
	var deals []appsumo.Deal
	for rows.Next() {
		var (
			deal        appsumo.Deal
			id          sql.NullInt64
			name        sql.NullString
			url         sql.NullString
			price       sql.NullFloat64
			original    sql.NullFloat64
			plusPrice   sql.NullFloat64
			isFree      sql.NullInt64
			listingType sql.NullString
			cardDesc    sql.NullString
			valueProp   sql.NullString
			bestFor     sql.NullString
			altTo       sql.NullString
			integ       sql.NullString
			subcat      sql.NullString
			codes       sql.NullInt64
			percent     sql.NullInt64
			startDate   sql.NullString
			endDate     sql.NullString
			timerText   sql.NullString
			reviewCount sql.NullInt64
			avgRating   sql.NullFloat64
			category    sql.NullString
			dealGroup   sql.NullString
			rawJSON     sql.NullString
		)
		if err := rows.Scan(
			&deal.Slug, &id, &name, &url, &price, &original, &plusPrice, &isFree, &listingType,
			&cardDesc, &valueProp, &bestFor, &altTo, &integ, &subcat,
			&codes, &percent, &startDate, &endDate, &timerText,
			&reviewCount, &avgRating, &category, &dealGroup, &rawJSON,
		); err != nil {
			return nil, err
		}
		deal.ID = id.Int64
		deal.Name = name.String
		deal.URL = url.String
		deal.Price = price.Float64
		deal.OriginalPrice = original.Float64
		deal.PlusPrice = plusPrice.Float64
		deal.IsFree = isFree.Int64 != 0
		deal.ListingType = listingType.String
		deal.CardDescription = cardDesc.String
		deal.ValueProp = valueProp.String
		deal.BestFor = jsonToStringSlice(bestFor.String)
		deal.AlternativeTo = jsonToStringSlice(altTo.String)
		deal.Integrations = jsonToStringSlice(integ.String)
		deal.Subcategory = subcat.String
		if codes.Valid {
			c := int(codes.Int64)
			deal.CodesRemaining = &c
		}
		if percent.Valid {
			p := int(percent.Int64)
			deal.PercentClaimed = &p
		}
		deal.StartDate = startDate.String
		deal.EndDate = endDate.String
		deal.TimerReason = timerText.String
		if reviewCount.Valid {
			rc := int(reviewCount.Int64)
			deal.ReviewCount = &rc
		}
		if avgRating.Valid {
			ar := avgRating.Float64
			deal.AverageRating = &ar
		}
		deal.Category = category.String
		deal.Group = dealGroup.String
		if rawJSON.Valid && rawJSON.String != "" {
			deal.Raw = json.RawMessage(rawJSON.String)
		}
		deals = append(deals, deal)
	}
	return deals, rows.Err()
}
