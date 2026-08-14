package appsumo

// Public product reviews.
//
// These endpoints are public: they are not part of the buyer account surface and
// they must never carry the user's session cookie. Every request routes through
// Client.public(), so a cookie-bearing client cannot leak credentials into a
// review crawl.
//
// The offset walk, its duplicate guard, and its reconciliation against
// meta.total live in threads.go, shared with the Q&A surface.

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	// Retained as the documented names for the reviews surface; the shared
	// thread defaults are the single source of truth.
	DefaultReviewsPageSize = DefaultThreadPageSize
	DefaultReviewsSort     = DefaultThreadSort
	DefaultReviewsOrder    = DefaultThreadOrder
)

var nextDataPattern = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)

type ReviewUser struct {
	ID              flexInt64 `json:"id"`
	Username        string    `json:"username"`
	DateJoined      string    `json:"date_joined"`
	DealsPurchased  *int      `json:"deals_purchased"`
	CommentDisabled *bool     `json:"comment_blacklist"`
}

type Review struct {
	ID             flexInt64  `json:"id"`
	DealID         flexInt64  `json:"deal_id"`
	ParentID       *flexInt64 `json:"parent_id"`
	Level          int        `json:"level"`
	Title          string     `json:"title"`
	Comment        string     `json:"comment"`
	Rating         *int       `json:"rating"`
	Created        string     `json:"created"`
	Modified       string     `json:"modified"`
	UpVotes        *int       `json:"up_votes"`
	DownVotes      *int       `json:"down_votes"`
	WouldRecommend *bool      `json:"would_recommend"`
	Incentivized   *bool      `json:"incentivized"`
	Purchased      *bool      `json:"purchased"`
	Approved       *bool      `json:"approved"`
	Edited         *bool      `json:"edited"`
	Status         string     `json:"status"`
	DisplayPath    string     `json:"display_path"`
	User           ReviewUser `json:"user"`
	Children       []Review   `json:"children"`
}

// ReviewsMeta is the reviews-surface name for the shared thread envelope meta.
type ReviewsMeta = ThreadMeta

type ReviewsEnvelope struct {
	Comments []Review    `json:"comments"`
	Meta     ReviewsMeta `json:"meta"`
}

type DealRatings struct {
	ReviewCount   *int           `json:"review_count"`
	AverageRating string         `json:"average_rating"`
	Distribution  map[string]int `json:"distribution"`
}

type ProductRef struct {
	Slug    string       `json:"slug"`
	DealID  int64        `json:"deal_id"`
	Name    string       `json:"name"`
	URL     string       `json:"url"`
	Ratings *DealRatings `json:"ratings"`
}

// ReviewsQuery is the reviews-surface name for the shared thread query.
type ReviewsQuery = ThreadQuery

type ReviewsResult struct {
	Reviews       []Review
	ExpectedTotal *int
	Requests      int
	Truncated     bool
	Warnings      []string
}

// ResolveProduct maps a product slug to the deal id the reviews API is keyed on,
// by reading the __NEXT_DATA__ island the reviews page already ships.
func (c *Client) ResolveProduct(ctx context.Context, slug string) (*ProductRef, error) {
	slug = strings.Trim(strings.TrimSpace(slug), "/")
	if slug == "" {
		return nil, fmt.Errorf("product slug is required")
	}
	path := fmt.Sprintf("/products/%s/reviews/", slug)
	body, err := c.public().getHTML(ctx, path)
	if err != nil {
		return nil, err
	}
	match := nextDataPattern.FindSubmatch(body)
	if match == nil {
		return nil, fmt.Errorf("no __NEXT_DATA__ island on %s; the product page layout changed", path)
	}

	var payload struct {
		Props struct {
			PageProps struct {
				Deal struct {
					ID         flexInt64 `json:"id"`
					Slug       string    `json:"slug"`
					PublicName string    `json:"public_name"`
					DealReview *struct {
						ReviewCount   *int   `json:"review_count"`
						AverageRating string `json:"average_rating"`
						OneTaco       *int   `json:"review_count_1_tacos"`
						TwoTacos      *int   `json:"review_count_2_tacos"`
						ThreeTacos    *int   `json:"review_count_3_tacos"`
						FourTacos     *int   `json:"review_count_4_tacos"`
						FiveTacos     *int   `json:"review_count_5_tacos"`
					} `json:"deal_review"`
				} `json:"deal"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(match[1], &payload); err != nil {
		return nil, fmt.Errorf("decode __NEXT_DATA__ on %s: %w", path, err)
	}

	deal := payload.Props.PageProps.Deal
	if deal.ID == 0 {
		return nil, fmt.Errorf("no deal id for product %q; check the slug at %s%s", slug, c.baseURL, path)
	}

	ref := &ProductRef{
		Slug:   firstNonBlank(deal.Slug, slug),
		DealID: int64(deal.ID),
		Name:   deal.PublicName,
		URL:    c.baseURL + path,
	}
	if review := deal.DealReview; review != nil {
		distribution := map[string]int{}
		for label, value := range map[string]*int{
			"1": review.OneTaco, "2": review.TwoTacos, "3": review.ThreeTacos,
			"4": review.FourTacos, "5": review.FiveTacos,
		} {
			if value != nil {
				distribution[label] = *value
			}
		}
		ref.Ratings = &DealRatings{
			ReviewCount:   review.ReviewCount,
			AverageRating: review.AverageRating,
			Distribution:  distribution,
		}
	}
	return ref, nil
}

func reviewsPath(dealID int64) string {
	return fmt.Sprintf("/api/v2/deals/%d/reviews/", dealID)
}

// FetchReviewsPage reads one offset window of a deal's public reviews.
func (c *Client) FetchReviewsPage(ctx context.Context, query ReviewsQuery) (*ReviewsEnvelope, error) {
	page, err := fetchThreadPage[Review](ctx, c, reviewsPath(query.DealID), query)
	if err != nil {
		return nil, err
	}
	return &ReviewsEnvelope{Comments: page.Comments, Meta: page.Meta}, nil
}

// FetchAllReviews walks every offset window until the API stops returning rows.
//
// limit caps the number of reviews collected; 0 means no cap. The result carries
// warnings rather than silently reporting a short crawl as complete.
func (c *Client) FetchAllReviews(ctx context.Context, query ReviewsQuery, limit int) (*ReviewsResult, error) {
	crawl, err := fetchThread(ctx, c, reviewsPath(query.DealID), "reviews", query, limit,
		func(review Review) int64 { return int64(review.ID) })
	if err != nil {
		return nil, err
	}
	return &ReviewsResult{
		Reviews:       crawl.Items,
		ExpectedTotal: crawl.ExpectedTotal,
		Requests:      crawl.Requests,
		Truncated:     crawl.Truncated,
		Warnings:      crawl.Warnings,
	}, nil
}
