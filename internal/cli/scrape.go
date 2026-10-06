package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vecyang1/appsumo-cli/internal/appsumo"
	"github.com/vecyang1/appsumo-cli/internal/store"
)

type scrapeSummary struct {
	Slug             string   `json:"slug"`
	Name             string   `json:"name"`
	DealID           int64    `json:"deal_id"`
	Status           string   `json:"status"`
	Rating           string   `json:"rating"`
	TotalReviewCount int      `json:"total_review_count"`
	TiersCount       int      `json:"tiers_count"`
	FAQsCount        int      `json:"faqs_count"`
	FounderPosts     int      `json:"founder_posts_count"`
	ReviewsScraped   int      `json:"reviews_scraped"`
	QuestionsScraped int      `json:"questions_scraped"`
	OutputDir        string   `json:"output_dir"`
	SavedFiles       []string `json:"saved_files"`
}

func (rt *runtime) scrapeCmd() *cobra.Command {
	var (
		outDir      string
		noReviews   bool
		noQuestions bool
		saveDB      bool
	)

	cmd := &cobra.Command{
		Use:   "scrape <product-slug-or-url>",
		Short: "Scrape, aggregate, save, and cache all product info, pricing tiers, FAQs, reviews, and questions",
		Long: "Scrape, aggregate, save, and cache complete AppSumo product information.\n\n" +
			"Accepts a product slug (e.g. 'poppy-ai') or full URL (e.g. 'https://appsumo.com/products/poppy-ai/').\n" +
			"Extracts full deal specifications, founder profiles & posts, pricing tiers, feature comparison matrix,\n" +
			"terms, FAQs, and walks all public reviews and questions into structured JSON files and a Markdown dossier.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := appsumo.CleanProductSlug(args[0])
			if slug == "" {
				return fmt.Errorf("invalid product slug or url: %s", args[0])
			}

			if outDir == "" {
				outDir = filepath.Join("data", slug)
			}

			client := rt.publicClient()

			if !rt.asJSON {
				fmt.Fprintf(cmd.OutOrStdout(), "📦 Fetching deal specifications for '%s'...\n", slug)
			}

			includeReviews := !noReviews
			includeQuestions := !noQuestions

			archive, err := client.ScrapeProductArchive(cmd.Context(), slug, includeReviews, includeQuestions)
			if err != nil {
				return err
			}

			if !rt.asJSON {
				fmt.Fprintf(cmd.OutOrStdout(), "💾 Saving artifacts to %s...\n", outDir)
			}

			if err := archive.SaveToDirectory(outDir); err != nil {
				return fmt.Errorf("save archive to %s: %w", outDir, err)
			}

			if saveDB {
				if len(archive.Reviews) > 0 {
					_, _ = rt.saveThread(cmd, func(db *store.DB) (int, error) {
						return db.SaveReviews(cmd.Context(), archive.Deal.Slug, archive.Reviews)
					})
				}
				if len(archive.Questions) > 0 {
					_, _ = rt.saveThread(cmd, func(db *store.DB) (int, error) {
						return db.SaveQuestions(cmd.Context(), archive.Deal.Slug, archive.Questions)
					})
				}
			}

			ratingStr := "N/A"
			revCount := 0
			if r := archive.Deal.Ratings; r != nil {
				ratingStr = r.AverageRating
				if r.ReviewCount != nil {
					revCount = *r.ReviewCount
				}
			}

			savedFiles := []string{
				"deal.json",
				"reviews.json",
				"questions.json",
				"faqs.json",
				"founders.json",
				"full_archive.json",
				"DOSSIER.md",
				"SUMMARY.md",
			}

			summary := scrapeSummary{
				Slug:             archive.Deal.Slug,
				Name:             archive.Deal.PublicName,
				DealID:           int64(archive.Deal.ID),
				Status:           archive.Deal.DealStatus,
				Rating:           ratingStr,
				TotalReviewCount: revCount,
				TiersCount:       len(archive.Deal.Plans),
				FAQsCount:        len(archive.Deal.FAQs),
				FounderPosts:     len(archive.Deal.Founders.Posts),
				ReviewsScraped:   len(archive.Reviews),
				QuestionsScraped: len(archive.Questions),
				OutputDir:        outDir,
				SavedFiles:       savedFiles,
			}

			if rt.asJSON {
				return writeRedactedJSON(cmd.OutOrStdout(), summary)
			}

			return writeScrapeText(cmd.OutOrStdout(), summary)
		},
	}

	cmd.Flags().StringVarP(&outDir, "out-dir", "o", "", "Directory to save scraped cache (default: data/<slug>)")
	cmd.Flags().BoolVar(&noReviews, "no-reviews", false, "Skip scraping reviews")
	cmd.Flags().BoolVar(&noQuestions, "no-questions", false, "Skip scraping questions")
	cmd.Flags().BoolVar(&saveDB, "save-db", false, "Also store reviews and questions into local SQLite DB")

	return cmd
}

func writeScrapeText(out io.Writer, s scrapeSummary) error {
	fmt.Fprintf(out, "\n=======================================================\n")
	fmt.Fprintf(out, "  APPSUMO PRODUCT SCRAPE COMPLETE: %s\n", s.Name)
	fmt.Fprintf(out, "=======================================================\n")
	fmt.Fprintf(out, "  • Slug:             %s\n", s.Slug)
	fmt.Fprintf(out, "  • Deal ID:          %d\n", s.DealID)
	fmt.Fprintf(out, "  • Deal Status:      %s\n", s.Status)
	fmt.Fprintf(out, "  • Overall Rating:   ⭐ %s / 5 Tacos (%d total declared)\n", s.Rating, s.TotalReviewCount)
	fmt.Fprintf(out, "  • Pricing Tiers:    %d plans\n", s.TiersCount)
	fmt.Fprintf(out, "  • FAQs:             %d questions & answers\n", s.FAQsCount)
	fmt.Fprintf(out, "  • Founder Posts:    %d announcements\n", s.FounderPosts)
	fmt.Fprintf(out, "  • Reviews Scraped:  %d reviews\n", s.ReviewsScraped)
	fmt.Fprintf(out, "  • Questions Scraped:%d questions\n", s.QuestionsScraped)
	fmt.Fprintf(out, "  • Output Directory: %s\n", s.OutputDir)
	fmt.Fprintf(out, "  • Saved Files:      %v\n", s.SavedFiles)
	fmt.Fprintf(out, "=======================================================\n")
	fmt.Fprintf(out, "✓ All product specifications, tiers, reviews, and questions cached successfully.\n\n")
	return nil
}
