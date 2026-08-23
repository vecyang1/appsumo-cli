package store

// Review and Q&A persistence.
//
// Both surfaces land in one table discriminated by `kind`, because they share a
// shape and the useful queries cross them ("everything people said about the
// products I own"). Reply threads are flattened: a reply is a row with
// parent_id set, so SQL can group a thread without parsing nested JSON.
//
// rating is nullable and stays NULL for questions and for replies — neither
// carries one. Writing 0 there would put a one-star review into the average.

import (
	"context"
	"encoding/json"
	"time"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"
)

const threadsSchema = `
create table if not exists product_comments (
	kind text not null,
	id integer not null,
	product_slug text not null,
	deal_id integer,
	parent_id integer,
	level integer,
	title text,
	body text,
	rating integer,
	created text,
	modified text,
	up_votes integer,
	down_votes integer,
	status text,
	display_path text,
	username text,
	answered integer,
	raw_json text not null,
	synced_at text not null,
	primary key (kind, id)
);

create index if not exists product_comments_slug on product_comments (product_slug, kind);
`

// CommentKind names the surface a stored comment came from.
type CommentKind string

const (
	KindReview   CommentKind = "review"
	KindQuestion CommentKind = "question"
)

type commentRow struct {
	kind        CommentKind
	id          int64
	productSlug string
	dealID      int64
	parentID    *int64
	level       int
	title       string
	body        string
	rating      *int
	created     string
	modified    string
	upVotes     *int
	downVotes   *int
	status      string
	displayPath string
	username    string
	answered    *bool
	raw         any
}

// SaveReviews flattens and stores a review crawl for one product.
func (db *DB) SaveReviews(ctx context.Context, productSlug string, reviews []appsumo.Review) (int, error) {
	var rows []commentRow
	var walk func(items []appsumo.Review)
	walk = func(items []appsumo.Review) {
		for _, review := range items {
			rows = append(rows, commentRow{
				kind: KindReview, id: int64(review.ID), productSlug: productSlug,
				dealID: int64(review.DealID), parentID: reviewParent(review),
				level: review.Level, title: review.Title, body: review.Comment,
				rating: review.Rating, created: review.Created, modified: review.Modified,
				upVotes: review.UpVotes, downVotes: review.DownVotes,
				status: review.Status, displayPath: review.DisplayPath,
				username: review.User.Username, raw: review,
			})
			walk(review.Children)
		}
	}
	walk(reviews)
	return db.saveComments(ctx, rows)
}

// SaveQuestions flattens and stores a Q&A crawl for one product.
func (db *DB) SaveQuestions(ctx context.Context, productSlug string, questions []appsumo.Question) (int, error) {
	var rows []commentRow
	var walk func(items []appsumo.Question)
	walk = func(items []appsumo.Question) {
		for _, question := range items {
			answered := question.Answered()
			rows = append(rows, commentRow{
				kind: KindQuestion, id: int64(question.ID), productSlug: productSlug,
				dealID: int64(question.DealID), parentID: questionParent(question),
				level: question.Level, title: question.Title, body: question.Comment,
				created: question.Created, modified: question.Modified,
				upVotes: question.UpVotes, downVotes: question.DownVotes,
				status: question.Status, displayPath: question.DisplayPath,
				username: question.User.Username, answered: &answered, raw: question,
			})
			walk(question.Children)
		}
	}
	walk(questions)
	return db.saveComments(ctx, rows)
}

func (db *DB) saveComments(ctx context.Context, rows []commentRow) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `insert into product_comments (
		kind, id, product_slug, deal_id, parent_id, level, title, body, rating,
		created, modified, up_votes, down_votes, status, display_path, username,
		answered, raw_json, synced_at
	) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	on conflict(kind, id) do update set
		product_slug=excluded.product_slug, deal_id=excluded.deal_id,
		parent_id=excluded.parent_id, level=excluded.level, title=excluded.title,
		body=excluded.body, rating=excluded.rating, created=excluded.created,
		modified=excluded.modified, up_votes=excluded.up_votes,
		down_votes=excluded.down_votes, status=excluded.status,
		display_path=excluded.display_path, username=excluded.username,
		answered=excluded.answered, raw_json=excluded.raw_json,
		synced_at=excluded.synced_at`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	syncedAt := time.Now().UTC().Format(time.RFC3339)
	for _, row := range rows {
		raw, marshalErr := json.Marshal(row.raw)
		if marshalErr != nil {
			return 0, marshalErr
		}
		if _, err := stmt.ExecContext(ctx,
			string(row.kind), row.id, row.productSlug, row.dealID,
			nullableInt64(row.parentID), row.level, row.title, row.body,
			nullableInt(row.rating), row.created, row.modified,
			nullableInt(row.upVotes), nullableInt(row.downVotes),
			row.status, row.displayPath, row.username,
			nullableBool(row.answered), string(raw), syncedAt,
		); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true
	return len(rows), nil
}

// CountComments reports how many rows of one kind are stored for a product.
func (db *DB) CountComments(ctx context.Context, productSlug string, kind CommentKind) (int, error) {
	var count int
	err := db.db.QueryRowContext(ctx,
		`select count(*) from product_comments where product_slug = ? and kind = ?`,
		productSlug, string(kind)).Scan(&count)
	return count, err
}

// reviewParent and questionParent exist because ParentID is a pointer to an
// unexported flexible-id type; a caller outside the appsumo package can
// dereference and convert it but cannot name it.
func reviewParent(review appsumo.Review) *int64 {
	if review.ParentID == nil {
		return nil
	}
	id := int64(*review.ParentID)
	return &id
}

func questionParent(question appsumo.Question) *int64 {
	if question.ParentID == nil {
		return nil
	}
	id := int64(*question.ParentID)
	return &id
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableBool(value *bool) any {
	if value == nil {
		return nil
	}
	if *value {
		return 1
	}
	return 0
}
