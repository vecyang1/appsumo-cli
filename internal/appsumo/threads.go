package appsumo

// Threaded public comment endpoints.
//
// AppSumo serves product reviews and product Q&A from two paths that share one
// envelope and one pagination contract:
//
//	GET /api/v2/deals/{deal_id}/reviews/?from=0&items_per_page=100&sort=date&order=asc
//	GET /api/v2/deals/{deal_id}/questions/?from=0&items_per_page=100&sort=date&order=asc
//
// Both answer {"comments": [...], "meta": {"total": N, "count": M}}, and in both
// `from` is the only real offset — the `page` parameter appsumo.com puts in its
// own URLs is accepted and ignored (see docs/03_reviews_api_discovery.md). The
// crawl lives here once so a guard added for one surface protects the other.
//
// Everything here is public and must never carry the buyer's session cookie;
// every request routes through Client.public().

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const (
	// DefaultThreadPageSize is the window size for one comment request.
	DefaultThreadPageSize = 100

	// DefaultThreadSort and DefaultThreadOrder page oldest-first on purpose.
	// Offset pagination over a live list is only stable when new rows land at
	// the end; with the newest-first order the site uses, a comment posted
	// mid-crawl shifts every later offset by one and one row is skipped.
	DefaultThreadSort  = "date"
	DefaultThreadOrder = "asc"

	maxThreadRequests = 500
)

// ThreadMeta carries the server's own count of a comment thread.
//
// Total and Count are pointers so an absent field stays unknown instead of
// collapsing to a confident zero.
type ThreadMeta struct {
	Total *int `json:"total"`
	Count *int `json:"count"`
}

// ThreadQuery addresses one offset window of a deal's public comments.
type ThreadQuery struct {
	DealID   int64
	From     int
	PageSize int
	Sort     string
	Order    string
}

// threadEnvelope is the shared response shape of both comment endpoints.
type threadEnvelope[T any] struct {
	Comments []T        `json:"comments"`
	Meta     ThreadMeta `json:"meta"`
}

// threadCrawl is the accumulated result of walking every offset window.
//
// Effective carries the parameters the crawl actually sent, which are not always
// the ones the caller passed: a zero page size and a blank sort are substituted.
// Reporting the request instead of the substitution labels a healthy crawl with
// settings it did not run under.
type threadCrawl[T any] struct {
	Items         []T
	ExpectedTotal *int
	Requests      int
	Truncated     bool
	Warnings      []string
	Effective     ThreadQuery
}

// resolve fills in the defaults this endpoint is actually queried with, so that
// one function decides them and callers can report what was sent.
func (q ThreadQuery) resolve() ThreadQuery {
	if q.PageSize <= 0 {
		q.PageSize = DefaultThreadPageSize
	}
	if q.From < 0 {
		q.From = 0
	}
	q.Sort = firstNonBlank(q.Sort, DefaultThreadSort)
	q.Order = firstNonBlank(q.Order, DefaultThreadOrder)
	return q
}

func (q ThreadQuery) params() map[string]string {
	resolved := q.resolve()
	return map[string]string{
		"from":           strconv.Itoa(resolved.From),
		"items_per_page": strconv.Itoa(resolved.PageSize),
		"sort":           resolved.Sort,
		"order":          resolved.Order,
	}
}

// fetchThreadPage reads one offset window of a deal's public comments.
func fetchThreadPage[T any](ctx context.Context, client *Client, path string, query ThreadQuery) (*threadEnvelope[T], error) {
	if query.DealID <= 0 {
		return nil, fmt.Errorf("deal id is required")
	}
	var envelope threadEnvelope[T]
	if err := client.public().getJSON(ctx, path, query.params(), &envelope); err != nil {
		return nil, err
	}
	return &envelope, nil
}

// fetchThread walks every offset window until the endpoint stops yielding new
// rows, then reconciles the crawl against the server's own total.
//
// noun names the rows for the caller's warnings ("reviews", "questions"); it is
// the one thing this shared code cannot know about its callers. limit caps the
// collected rows, 0 meaning no cap. idOf supplies the identity used to detect an
// endpoint that ignores `from`.
func fetchThread[T any](
	ctx context.Context,
	client *Client,
	path string,
	noun string,
	query ThreadQuery,
	limit int,
	idOf func(T) int64,
) (*threadCrawl[T], error) {
	query = query.resolve()
	crawl := &threadCrawl[T]{Effective: query, Warnings: []string{}}
	seen := make(map[int64]struct{})
	offset := query.From

	// Two different reasons to stop early, and they want opposite reporting.
	// A caller-requested cap is expected, so comparing its short count against
	// meta.total would warn on every --limit run. A detected anomaly is not
	// expected, and there the shortfall is the most useful number available.
	// Both mark the result Truncated, so neither can certify itself complete.
	cappedOnPurpose := false

	for crawl.Requests < maxThreadRequests {
		window := query
		window.From = offset
		page, err := fetchThreadPage[T](ctx, client, path, window)
		if err != nil {
			return nil, err
		}
		crawl.Requests++
		if crawl.ExpectedTotal == nil && page.Meta.Total != nil {
			total := *page.Meta.Total
			crawl.ExpectedTotal = &total
		}
		if len(page.Comments) == 0 {
			break
		}

		added := 0
		for _, item := range page.Comments {
			id := idOf(item)
			if _, duplicate := seen[id]; duplicate {
				continue
			}
			seen[id] = struct{}{}
			crawl.Items = append(crawl.Items, item)
			added++
			if limit > 0 && len(crawl.Items) >= limit {
				break
			}
		}

		if limit > 0 && len(crawl.Items) >= limit {
			crawl.Truncated = true
			cappedOnPurpose = true
			crawl.Warnings = append(crawl.Warnings, fmt.Sprintf(
				"stopped at --limit %d; more %s exist", limit, noun))
			break
		}

		// A full window of ids we already hold means the server ignored `from`.
		// Advancing further would loop on page one forever.
		//
		// Truncated is set for the same reason as in FetchAllDeals: the crawl
		// stopped on a detected anomaly, so its count must not be allowed to
		// certify itself against a total from the same suspect stream.
		if added == 0 {
			crawl.Truncated = true
			crawl.Warnings = append(crawl.Warnings, fmt.Sprintf(
				"offset %d returned %d %s but no new ids; the API appears to ignore `from` — crawl stopped early",
				offset, len(page.Comments), noun))
			break
		}
		offset += len(page.Comments)
	}

	if crawl.Requests >= maxThreadRequests {
		crawl.Truncated = true
		cappedOnPurpose = true
		crawl.Warnings = append(crawl.Warnings, fmt.Sprintf(
			"stopped after the %d request safety cap", maxThreadRequests))
	}
	switch {
	case crawl.ExpectedTotal == nil:
		crawl.Warnings = append(crawl.Warnings,
			"response carried no meta.total; completeness could not be verified")
	case !cappedOnPurpose && len(crawl.Items) != *crawl.ExpectedTotal:
		crawl.Warnings = append(crawl.Warnings, fmt.Sprintf(
			"collected %d %s but meta.total reported %d", len(crawl.Items), noun, *crawl.ExpectedTotal))
	}
	return crawl, nil
}

// public returns a copy of the client with no session cookie attached. Review,
// question, and product pages are public, and a crawl must not carry buyer
// credentials.
func (c *Client) public() *Client {
	if c.cookie == "" {
		return c
	}
	clone := *c
	clone.cookie = ""
	return &clone
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
