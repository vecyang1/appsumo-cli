package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"
	"github.com/vecyang1/appsumo-cli/internal/store"

	"github.com/spf13/cobra"
)

type reviewsFetchInfo struct {
	UniqueReviews int    `json:"unique_reviews"`
	ExpectedTotal *int   `json:"expected_total"`
	Complete      *bool  `json:"complete"`
	Requests      int    `json:"requests"`
	Truncated     bool   `json:"truncated"`
	Saved         *int   `json:"saved_rows"`
	Sort          string `json:"sort"`
	Order         string `json:"order"`
	PageSize      int    `json:"page_size"`
}

type reviewsReport struct {
	Product  *appsumo.ProductRef `json:"product"`
	Fetch    reviewsFetchInfo    `json:"fetch"`
	Warnings []string            `json:"warnings"`
	Reviews  []appsumo.Review    `json:"reviews"`
}

func (rt *runtime) reviewsCmd() *cobra.Command {
	var (
		limit    int
		pageSize int
		sort     string
		order    string
		save     bool
	)
	cmd := &cobra.Command{
		Use:   "reviews <product-slug>",
		Short: "Fetch every public review for a product",
		Long: "Fetch every public review for a product, walking the reviews API by offset.\n\n" +
			"Reviews are public: this command never reads or sends the AppSumo session cookie.\n" +
			"The `page` parameter appsumo.com puts in its own review URLs is ignored by the\n" +
			"server, so pagination here walks `from` instead and reconciles the crawl against\n" +
			"the reported review total.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := rt.publicClient()
			product, err := client.ResolveProduct(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			result, err := client.FetchAllReviews(cmd.Context(), appsumo.ReviewsQuery{
				DealID:   product.DealID,
				PageSize: pageSize,
				Sort:     sort,
				Order:    order,
			}, limit)
			if err != nil {
				return err
			}

			report := reviewsReport{
				Product: product,
				Fetch: reviewsFetchInfo{
					UniqueReviews: len(result.Reviews),
					ExpectedTotal: result.ExpectedTotal,
					Complete:      threadCompleteness(result.ExpectedTotal, len(result.Reviews), result.Truncated),
					Requests:      result.Requests,
					Truncated:     result.Truncated,
					Sort:          result.Effective.Sort,
					Order:         result.Effective.Order,
					PageSize:      result.Effective.PageSize,
				},
				Warnings: result.Warnings,
				Reviews:  result.Reviews,
			}

			if save {
				saved, err := rt.saveThread(cmd, func(db *store.DB) (int, error) {
					return db.SaveReviews(cmd.Context(), product.Slug, result.Reviews)
				})
				if err != nil {
					return err
				}
				report.Fetch.Saved = &saved
			}

			// Warnings go to stderr so --json output stays a clean pipe while a
			// human still sees that the crawl was short.
			for _, warning := range report.Warnings {
				if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning); err != nil {
					return err
				}
			}
			if rt.asJSON {
				return writeRedactedJSON(cmd.OutOrStdout(), report)
			}
			return writeReviewsText(cmd.OutOrStdout(), report)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Stop after N reviews (0 fetches all)")
	cmd.Flags().IntVar(&pageSize, "page-size", appsumo.DefaultReviewsPageSize, "Reviews per request")
	cmd.Flags().StringVar(&sort, "sort", appsumo.DefaultReviewsSort, "Server-side sort field")
	cmd.Flags().StringVar(&order, "order", appsumo.DefaultReviewsOrder, "Server-side sort order")
	cmd.Flags().BoolVar(&save, "save", false, "Also store the crawl in the local SQLite database")
	return cmd
}

func (rt *runtime) publicClient() *appsumo.Client {
	baseURL := firstNonEmpty(rt.options.BaseURL, os.Getenv("APPSUMO_BASE_URL"), appsumo.DefaultBaseURL)
	return appsumo.NewClient(appsumo.ClientOptions{
		BaseURL:    baseURL,
		HTTPClient: rt.options.HTTPClient,
	})
}

func writeReviewsText(out io.Writer, report reviewsReport) error {
	product := report.Product
	if _, err := fmt.Fprintf(out, "%s (%s) deal %d\n", firstNonEmpty(product.Name, product.Slug), product.Slug, product.DealID); err != nil {
		return err
	}
	if ratings := product.Ratings; ratings != nil {
		count := "unknown"
		if ratings.ReviewCount != nil {
			count = fmt.Sprintf("%d", *ratings.ReviewCount)
		}
		if _, err := fmt.Fprintf(out, "listed reviews %s, average %s\n", count, firstNonEmpty(ratings.AverageRating, "unknown")); err != nil {
			return err
		}
	}
	expected := "unknown"
	if report.Fetch.ExpectedTotal != nil {
		expected = fmt.Sprintf("%d", *report.Fetch.ExpectedTotal)
	}
	if _, err := fmt.Fprintf(out, "fetched %d of %s in %d requests (sort=%s order=%s)\n\n",
		report.Fetch.UniqueReviews, expected, report.Fetch.Requests, report.Fetch.Sort, report.Fetch.Order); err != nil {
		return err
	}
	for _, review := range report.Reviews {
		rating := "-"
		if review.Rating != nil {
			rating = fmt.Sprintf("%d", *review.Rating)
		}
		if _, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\n",
			truncate(review.Created, 10), rating, review.User.Username, truncate(oneLine(review.Title), 60)); err != nil {
			return err
		}
	}
	return nil
}
