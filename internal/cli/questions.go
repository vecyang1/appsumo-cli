package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"
	"github.com/vecyang1/appsumo-cli/internal/store"

	"github.com/spf13/cobra"
)

type questionsFetchInfo struct {
	UniqueQuestions int    `json:"unique_questions"`
	Answered        int    `json:"answered"`
	ExpectedTotal   *int   `json:"expected_total"`
	Complete        *bool  `json:"complete"`
	Requests        int    `json:"requests"`
	Truncated       bool   `json:"truncated"`
	Sort            string `json:"sort"`
	Order           string `json:"order"`
	PageSize        int    `json:"page_size"`
	Saved           *int   `json:"saved_rows"`
}

type questionsReport struct {
	Product   *appsumo.ProductRef `json:"product"`
	Fetch     questionsFetchInfo  `json:"fetch"`
	Warnings  []string            `json:"warnings"`
	Questions []appsumo.Question  `json:"questions"`
}

func (rt *runtime) questionsCmd() *cobra.Command {
	var (
		limit    int
		pageSize int
		sort     string
		order    string
		save     bool
	)
	cmd := &cobra.Command{
		Use:   "questions <product-slug>",
		Short: "Fetch every public question and answer for a product",
		Long: "Fetch every public question thread for a product, walking the Q&A API by offset.\n\n" +
			"Q&A is public: this command never reads or sends the AppSumo session cookie.\n" +
			"It shares the reviews pagination contract — `from` is the real offset and the\n" +
			"`page` parameter appsumo.com puts in its own URLs is ignored by the server — so\n" +
			"the crawl reconciles itself against the reported question total.\n\n" +
			"Answers arrive nested under their question and are not counted against --limit.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := rt.publicClient()
			product, err := client.ResolveProduct(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			result, err := client.FetchAllQuestions(cmd.Context(), appsumo.ThreadQuery{
				DealID:   product.DealID,
				PageSize: pageSize,
				Sort:     sort,
				Order:    order,
			}, limit)
			if err != nil {
				return err
			}

			answered := 0
			for _, question := range result.Questions {
				if question.Answered() {
					answered++
				}
			}

			report := questionsReport{
				Product: product,
				Fetch: questionsFetchInfo{
					UniqueQuestions: len(result.Questions),
					Answered:        answered,
					ExpectedTotal:   result.ExpectedTotal,
					Complete:        threadCompleteness(result.ExpectedTotal, len(result.Questions), result.Truncated),
					Requests:        result.Requests,
					Truncated:       result.Truncated,
					Sort:            result.Effective.Sort,
					Order:           result.Effective.Order,
					PageSize:        result.Effective.PageSize,
				},
				Warnings:  result.Warnings,
				Questions: result.Questions,
			}

			if save {
				saved, err := rt.saveThread(cmd, func(db *store.DB) (int, error) {
					return db.SaveQuestions(cmd.Context(), product.Slug, result.Questions)
				})
				if err != nil {
					return err
				}
				report.Fetch.Saved = &saved
			}

			for _, warning := range report.Warnings {
				if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning); err != nil {
					return err
				}
			}
			if rt.asJSON {
				return writeRedactedJSON(cmd.OutOrStdout(), report)
			}
			return writeQuestionsText(cmd.OutOrStdout(), report)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Stop after N question threads (0 fetches all)")
	cmd.Flags().IntVar(&pageSize, "page-size", appsumo.DefaultThreadPageSize, "Question threads per request")
	cmd.Flags().StringVar(&sort, "sort", appsumo.DefaultThreadSort, "Server-side sort field")
	cmd.Flags().StringVar(&order, "order", appsumo.DefaultThreadOrder, "Server-side sort order")
	cmd.Flags().BoolVar(&save, "save", false, "Also store the crawl in the local SQLite database")
	return cmd
}

// threadCompleteness stays nil when the API reported no total: an unverifiable
// crawl is unknown, not incomplete and not complete.
func threadCompleteness(expected *int, collected int, truncated bool) *bool {
	if expected == nil {
		return nil
	}
	complete := !truncated && collected == *expected
	return &complete
}

func (rt *runtime) saveThread(cmd *cobra.Command, save func(*store.DB) (int, error)) (int, error) {
	db, err := rt.openStore(cmd.Context())
	if err != nil {
		return 0, err
	}
	defer db.Close()
	return save(db)
}

func writeQuestionsText(out io.Writer, report questionsReport) error {
	product := report.Product
	if _, err := fmt.Fprintf(out, "%s (%s) deal %d\n", firstNonEmpty(product.Name, product.Slug), product.Slug, product.DealID); err != nil {
		return err
	}
	expected := "unknown"
	if report.Fetch.ExpectedTotal != nil {
		expected = fmt.Sprintf("%d", *report.Fetch.ExpectedTotal)
	}
	if _, err := fmt.Fprintf(out, "fetched %d of %s question threads in %d requests, %d answered\n\n",
		report.Fetch.UniqueQuestions, expected, report.Fetch.Requests, report.Fetch.Answered); err != nil {
		return err
	}
	for _, question := range report.Questions {
		mark := "open"
		if question.Answered() {
			mark = fmt.Sprintf("%d repl", len(question.Children))
			if len(question.Children) == 1 {
				mark += "y"
			} else {
				mark += "ies"
			}
		}
		body := firstNonEmpty(oneLine(question.Title), oneLine(question.Comment))
		if _, err := fmt.Fprintf(out, "%s\t%-9s\t%s\t%s\n",
			truncate(question.Created, 10), mark, question.User.Username, truncate(body, 70)); err != nil {
			return err
		}
	}
	return nil
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

// truncate cuts on rune boundaries; titles carry emoji and accents.
func truncate(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}
