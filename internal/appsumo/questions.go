package appsumo

// Public product questions and answers.
//
//	GET /api/v2/deals/{deal_id}/questions/?from=0&items_per_page=100&sort=date&order=asc
//
// Same envelope, same offset contract, and the same ignored `page` parameter as
// the reviews surface; the walk itself lives in threads.go.
//
// Two things differ from reviews and both are visible in live data:
//
//   - Questions carry no rating. A Q&A row has `resolved`, `pinned`, and
//     `answer_type` where a review has `rating` and `would_recommend`.
//   - A row's deal_id is not always the deal you asked for. Growify's current
//     deal is 257731, and its oldest questions report deal_id 213645 — Q&A
//     carries across a product's earlier deal runs. Do not assert equality.

import (
	"context"
	"fmt"
)

type QuestionUser struct {
	ID             flexInt64 `json:"id"`
	Username       string    `json:"username"`
	DateJoined     string    `json:"date_joined"`
	DealsPurchased *int      `json:"deals_purchased"`
	HasPlus        *bool     `json:"has_plus"`
}

type Question struct {
	ID          flexInt64    `json:"id"`
	DealID      flexInt64    `json:"deal_id"`
	ParentID    *flexInt64   `json:"parent_id"`
	Level       int          `json:"level"`
	Title       string       `json:"title"`
	Comment     string       `json:"comment"`
	Created     string       `json:"created"`
	Modified    string       `json:"modified"`
	UpVotes     *int         `json:"up_votes"`
	DownVotes   *int         `json:"down_votes"`
	Pinned      *bool        `json:"pinned"`
	Resolved    *bool        `json:"resolved"`
	Approved    *bool        `json:"approved"`
	Edited      *bool        `json:"edited"`
	Followup    *bool        `json:"followup"`
	Purchased   *bool        `json:"purchased"`
	AnswerType  *string      `json:"answer_type"`
	Status      string       `json:"status"`
	DisplayPath string       `json:"display_path"`
	User        QuestionUser `json:"user"`
	Children    []Question   `json:"children"`
}

type QuestionsEnvelope struct {
	Comments []Question `json:"comments"`
	Meta     ThreadMeta `json:"meta"`
}

type QuestionsResult struct {
	Questions     []Question
	ExpectedTotal *int
	Requests      int
	Truncated     bool
	Warnings      []string

	// Effective is what the crawl actually sent after defaults were applied.
	Effective ThreadQuery
}

// Answered reports whether a question thread has at least one reply. It is a
// property of the thread, not a field: `resolved` is set on the answering reply
// rather than on the question, and is null on plenty of answered threads.
func (q Question) Answered() bool {
	return len(q.Children) > 0
}

func questionsPath(dealID int64) string {
	return fmt.Sprintf("/api/v2/deals/%d/questions/", dealID)
}

// FetchQuestionsPage reads one offset window of a deal's public Q&A.
func (c *Client) FetchQuestionsPage(ctx context.Context, query ThreadQuery) (*QuestionsEnvelope, error) {
	page, err := fetchThreadPage[Question](ctx, c, questionsPath(query.DealID), query)
	if err != nil {
		return nil, err
	}
	return &QuestionsEnvelope{Comments: page.Comments, Meta: page.Meta}, nil
}

// FetchAllQuestions walks every offset window until the API stops returning rows.
//
// limit caps the number of question threads collected; 0 means no cap. Replies
// arrive nested under their question and are not counted against the limit.
func (c *Client) FetchAllQuestions(ctx context.Context, query ThreadQuery, limit int) (*QuestionsResult, error) {
	crawl, err := fetchThread(ctx, c, questionsPath(query.DealID), "questions", query, limit,
		func(question Question) int64 { return int64(question.ID) })
	if err != nil {
		return nil, err
	}
	return &QuestionsResult{
		Questions:     crawl.Items,
		ExpectedTotal: crawl.ExpectedTotal,
		Requests:      crawl.Requests,
		Truncated:     crawl.Truncated,
		Warnings:      crawl.Warnings,
		Effective:     crawl.Effective,
	}, nil
}
