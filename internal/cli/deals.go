package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"

	"github.com/spf13/cobra"
)

type dealsFetchInfo struct {
	UniqueDeals   int    `json:"unique_deals"`
	DeclaredTotal *int   `json:"declared_total"`
	Complete      *bool  `json:"complete"`
	Requests      int    `json:"requests"`
	Truncated     bool   `json:"truncated"`
	Sort          string `json:"sort"`
	PageSize      int    `json:"page_size"`
	SnapshotAt    string `json:"snapshot_at,omitempty"`
}

type dealsReport struct {
	Fetch    dealsFetchInfo `json:"fetch"`
	Warnings []string       `json:"warnings"`
	Deals    []appsumo.Deal `json:"deals"`
}

func (rt *runtime) dealsCmd() *cobra.Command {
	deals := &cobra.Command{
		Use:   "deals",
		Short: "Read the public AppSumo deal catalog",
		Long: "Read the public deal catalog.\n\n" +
			"The catalog is public: these commands never read or send the AppSumo session\n" +
			"cookie. Every walk sends a `sort` parameter, which is not cosmetic — without\n" +
			"one the backing search returns overlapping pages and silently drops rows\n" +
			"(measured 305 of 363 deals on 2026-08-14). Each walk is reconciled against\n" +
			"the catalog's own declared total and warns when it comes up short.",
	}
	deals.AddCommand(rt.dealsListCmd())
	deals.AddCommand(rt.dealsSearchCmd())
	deals.AddCommand(rt.dealsIdealCmd())
	deals.AddCommand(rt.dealsSyncNotionCmd())
	deals.AddCommand(rt.dealsSyncCmd())
	deals.AddCommand(rt.dealsDiffCmd())
	return deals
}

func (rt *runtime) dealsListCmd() *cobra.Command {
	var (
		limit      int
		pageSize   int
		sort       string
		query      string
		minRating  float64
		minReviews int
		maxPrice   float64
		category   string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List every live deal in the public catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := rt.publicClient().FetchAllDealsQuery(cmd.Context(), appsumo.DealsQuery{
				PerPage:    pageSize,
				Sort:       sort,
				Limit:      limit,
				Query:      query,
				MinRating:  minRating,
				MinReviews: minReviews,
				MaxPrice:   maxPrice,
				Category:   category,
			})
			if err != nil {
				return err
			}
			report := dealsReport{
				Fetch: dealsFetchInfo{
					UniqueDeals:   len(result.Deals),
					DeclaredTotal: result.DeclaredTotal,
					Complete:      result.Complete(),
					Requests:      result.Requests,
					Truncated:     result.Truncated,
					// Echo what the walk sent, not what was typed: an empty
					// --sort is substituted, and reporting the empty string
					// would describe a complete walk as the broken kind.
					Sort:     result.Sort,
					PageSize: result.PageSize,
				},
				Warnings: result.Warnings,
				Deals:    result.Deals,
			}
			return rt.emitDeals(cmd, report)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Stop after N deals (0 fetches all)")
	cmd.Flags().IntVar(&pageSize, "page-size", appsumo.DefaultDealsPageSize, "Deals per request")
	cmd.Flags().StringVar(&sort, "sort", appsumo.DefaultDealsSort, "Server-side sort; its presence is what makes the walk complete")
	cmd.Flags().StringVar(&query, "query", "", "Filter catalog deals by search keyword")
	cmd.Flags().Float64Var(&minRating, "min-rating", 0, "Filter by minimum average rating")
	cmd.Flags().IntVar(&minReviews, "min-reviews", 0, "Filter by minimum review count")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Filter by maximum price")
	cmd.Flags().StringVar(&category, "category", "", "Filter by category slug")
	return cmd
}

func (rt *runtime) dealsSearchCmd() *cobra.Command {
	var (
		limit      int
		pageSize   int
		sort       string
		local      bool
		minRating  float64
		minReviews int
		maxPrice   float64
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the AppSumo deal catalog",
		Long: "Search public AppSumo deals either live (default) or against local SQLite snapshots (--local).\n\n" +
			"The live search uses AppSumo's Elasticsearch public deal catalog without credentials.\n" +
			"Local search searches deal name, slug, description, value prop, alternatives, and category.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			if local {
				db, err := rt.openStore(cmd.Context())
				if err != nil {
					return err
				}
				defer db.Close()
				deals, err := db.SearchDeals(cmd.Context(), query)
				if err != nil {
					return err
				}
				filtered := make([]appsumo.Deal, 0, len(deals))
				for _, d := range deals {
					if minRating > 0 && (d.AverageRating == nil || *d.AverageRating < minRating) {
						continue
					}
					if minReviews > 0 && (d.ReviewCount == nil || *d.ReviewCount < minReviews) {
						continue
					}
					if maxPrice > 0 && d.Price > maxPrice {
						continue
					}
					filtered = append(filtered, d)
					if limit > 0 && len(filtered) >= limit {
						break
					}
				}
				report := dealsReport{
					Fetch: dealsFetchInfo{
						UniqueDeals: len(filtered),
						Sort:        "local-db",
					},
					Warnings: []string{},
					Deals:    filtered,
				}
				return rt.emitDeals(cmd, report)
			}

			result, err := rt.publicClient().FetchAllDealsQuery(cmd.Context(), appsumo.DealsQuery{
				Query:      query,
				Sort:       sort,
				PerPage:    pageSize,
				Limit:      limit,
				MinRating:  minRating,
				MinReviews: minReviews,
				MaxPrice:   maxPrice,
			})
			if err != nil {
				return err
			}
			report := dealsReport{
				Fetch: dealsFetchInfo{
					UniqueDeals:   len(result.Deals),
					DeclaredTotal: result.DeclaredTotal,
					Complete:      result.Complete(),
					Requests:      result.Requests,
					Truncated:     result.Truncated,
					Sort:          result.Sort,
					PageSize:      result.PageSize,
				},
				Warnings: result.Warnings,
				Deals:    result.Deals,
			}
			return rt.emitDeals(cmd, report)
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "Search synced deals in local SQLite database")
	cmd.Flags().IntVar(&limit, "limit", 20, "Stop after N deals (0 fetches all)")
	cmd.Flags().IntVar(&pageSize, "page-size", appsumo.DefaultDealsPageSize, "Deals per request")
	cmd.Flags().StringVar(&sort, "sort", appsumo.DealsSortRating, "Sort: rating, newest, etc.")
	cmd.Flags().Float64Var(&minRating, "min-rating", 0, "Filter by minimum average rating")
	cmd.Flags().IntVar(&minReviews, "min-reviews", 0, "Filter by minimum review count")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Filter by maximum price")
	return cmd
}

func (rt *runtime) dealsIdealCmd() *cobra.Command {
	var (
		limit      int
		pageSize   int
		sort       string
		local      bool
		minRating  float64
		minReviews int
		maxPrice   float64
		query      string
		category   string
		chinese    bool
		format     string
	)
	cmd := &cobra.Command{
		Use:   "ideal",
		Short: "Discover ideal, top-rated products from the AppSumo catalog",
		Long: "Discover ideal products with verified high customer satisfaction and review volume.\n\n" +
			"By default returns lifetime deals with rating >= 4.5 and at least 10 reviews,\n" +
			"sorted by rating. Works live or against local SQLite snapshots (--local).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if minRating <= 0 {
				minRating = 4.5
			}
			if minReviews <= 0 {
				minReviews = 10
			}
			if limit <= 0 {
				limit = 10
			}
			if sort == "" {
				sort = appsumo.DealsSortRating
			}

			if local {
				db, err := rt.openStore(cmd.Context())
				if err != nil {
					return err
				}
				defer db.Close()
				deals, err := db.IdealDealsQuery(cmd.Context(), appsumo.DealsQuery{
					Query:      query,
					Category:   category,
					MaxPrice:   maxPrice,
					MinRating:  minRating,
					MinReviews: minReviews,
					Limit:      limit,
				})
				if err != nil {
					return err
				}
				report := dealsReport{
					Fetch: dealsFetchInfo{
						UniqueDeals: len(deals),
						Sort:        "local-db-ideal",
					},
					Warnings: []string{},
					Deals:    deals,
				}
				if rt.asJSON {
					return writeRedactedJSON(cmd.OutOrStdout(), report)
				}
				if chinese {
					return writeIdealDealsChinese(cmd.OutOrStdout(), report, strings.EqualFold(format, "markdown"))
				}
				if strings.EqualFold(format, "markdown") {
					return writeIdealDealsChineseMarkdown(cmd.OutOrStdout(), report)
				}
				if strings.EqualFold(format, "card") {
					return writeIdealDealsChinese(cmd.OutOrStdout(), report, false)
				}
				return rt.emitDeals(cmd, report)
			}

			result, err := rt.publicClient().FetchAllDealsQuery(cmd.Context(), appsumo.DealsQuery{
				Query:      query,
				Sort:       sort,
				PerPage:    pageSize,
				Limit:      limit,
				MinRating:  minRating,
				MinReviews: minReviews,
				MaxPrice:   maxPrice,
				Category:   category,
			})
			if err != nil {
				return err
			}
			report := dealsReport{
				Fetch: dealsFetchInfo{
					UniqueDeals:   len(result.Deals),
					DeclaredTotal: result.DeclaredTotal,
					Complete:      result.Complete(),
					Requests:      result.Requests,
					Truncated:     result.Truncated,
					Sort:          result.Sort,
					PageSize:      result.PageSize,
				},
				Warnings: result.Warnings,
				Deals:    result.Deals,
			}
			if rt.asJSON {
				return writeRedactedJSON(cmd.OutOrStdout(), report)
			}
			if chinese {
				return writeIdealDealsChinese(cmd.OutOrStdout(), report, strings.EqualFold(format, "markdown"))
			}
			if strings.EqualFold(format, "markdown") {
				return writeIdealDealsChineseMarkdown(cmd.OutOrStdout(), report)
			}
			if strings.EqualFold(format, "card") {
				return writeIdealDealsChinese(cmd.OutOrStdout(), report, false)
			}
			return rt.emitDeals(cmd, report)
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "Query from local SQLite database instead of live API")
	cmd.Flags().Float64Var(&minRating, "min-rating", 4.5, "Minimum average rating (e.g. 4.5)")
	cmd.Flags().IntVar(&minReviews, "min-reviews", 10, "Minimum review count (e.g. 10)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Number of ideal deals to return (default 10)")
	cmd.Flags().IntVar(&pageSize, "page-size", appsumo.DefaultDealsPageSize, "Deals per request")
	cmd.Flags().StringVar(&sort, "sort", appsumo.DealsSortRating, "Server-side sort order (default 'rating')")
	cmd.Flags().StringVar(&query, "query", "", "Filter ideal deals by keyword")
	cmd.Flags().StringVar(&category, "category", "", "Filter ideal deals by category")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum price")
	cmd.Flags().BoolVarP(&chinese, "chinese", "c", false, "Output deal recommendations in Chinese")
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, card, markdown")
	return cmd
}

func (rt *runtime) dealsSyncCmd() *cobra.Command {
	var (
		pageSize int
		sort     string
		keep     int
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Store a catalog snapshot in local SQLite",
		Long: "Walk the public catalog and record it as a timestamped snapshot.\n\n" +
			"Run this on a schedule; `appsumo deals diff` then reports what moved between\n" +
			"the two most recent snapshots.",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := rt.publicClient().FetchAllDeals(cmd.Context(), pageSize, sort, 0)
			if err != nil {
				return err
			}
			// An incomplete walk must not be recorded as a snapshot: the next
			// diff would report every unserved deal as "gone".
			if complete := result.Complete(); complete != nil && !*complete {
				for _, warning := range result.Warnings {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
				}
				return fmt.Errorf("refusing to snapshot an incomplete catalog walk: collected %d of %d deals", len(result.Deals), *result.DeclaredTotal)
			}

			db, err := rt.openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			stamp, err := db.SaveDealSnapshot(cmd.Context(), time.Now(), result.Deals)
			if err != nil {
				return err
			}
			if keep > 0 {
				if _, err := db.PruneSnapshots(cmd.Context(), keep); err != nil {
					return err
				}
			}

			for _, warning := range result.Warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
			}
			if rt.asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"snapshot_at": stamp, "deals": len(result.Deals),
					"declared_total": result.DeclaredTotal, "complete": result.Complete(),
				})
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "snapshot %s recorded %d deals\n", stamp, len(result.Deals))
			return err
		},
	}
	cmd.Flags().IntVar(&pageSize, "page-size", appsumo.DefaultDealsPageSize, "Deals per request")
	cmd.Flags().StringVar(&sort, "sort", appsumo.DefaultDealsSort, "Server-side sort; its presence is what makes the walk complete")
	cmd.Flags().IntVar(&keep, "keep", 30, "Keep only the newest N snapshots (0 keeps all)")
	return cmd
}

func (rt *runtime) dealsDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Report what changed between the two most recent catalog snapshots",
		Long: "Compare the two most recent `deals sync` snapshots.\n\n" +
			"A deal that left the catalog is reported as `gone`, not `ended`: the browse\n" +
			"endpoint marks every row it serves as current, so disappearing is the only\n" +
			"available signal and it cannot tell sold out from expired from delisted.",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := rt.openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			stamps, err := db.SnapshotIDs(cmd.Context(), 2)
			if err != nil {
				return err
			}
			if len(stamps) < 2 {
				return fmt.Errorf("need two catalog snapshots to diff, have %d; run `appsumo deals sync` again later", len(stamps))
			}
			newer, older := stamps[0], stamps[1]
			after, err := db.LoadSnapshot(cmd.Context(), newer)
			if err != nil {
				return err
			}
			before, err := db.LoadSnapshot(cmd.Context(), older)
			if err != nil {
				return err
			}

			changes := appsumo.DiffDeals(before, after)
			if rt.asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"from": older, "to": newer,
					"before_count": len(before), "after_count": len(after),
					"changes": changes,
				})
			}
			return writeDealChangesText(cmd.OutOrStdout(), older, newer, len(before), len(after), changes)
		},
	}
	return cmd
}

func (rt *runtime) emitDeals(cmd *cobra.Command, report dealsReport) error {
	for _, warning := range report.Warnings {
		if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning); err != nil {
			return err
		}
	}
	if rt.asJSON {
		return writeRedactedJSON(cmd.OutOrStdout(), report)
	}
	return writeDealsText(cmd.OutOrStdout(), report)
}

func writeDealsText(out io.Writer, report dealsReport) error {
	if strings.HasPrefix(report.Fetch.Sort, "local") {
		if _, err := fmt.Fprintf(out, "%d deals in local database (sort=%s)\n\n",
			report.Fetch.UniqueDeals, report.Fetch.Sort); err != nil {
			return err
		}
	} else {
		declared := "unknown"
		if report.Fetch.DeclaredTotal != nil {
			declared = fmt.Sprintf("%d", *report.Fetch.DeclaredTotal)
		}
		if _, err := fmt.Fprintf(out, "%d of %s live deals in %d requests (sort=%s)\n\n",
			report.Fetch.UniqueDeals, declared, report.Fetch.Requests, report.Fetch.Sort); err != nil {
			return err
		}
	}
	for _, deal := range report.Deals {
		rating := "-"
		if deal.AverageRating != nil {
			if deal.ReviewCount != nil && *deal.ReviewCount > 0 {
				rating = fmt.Sprintf("%.2f★(%d)", *deal.AverageRating, *deal.ReviewCount)
			} else {
				rating = fmt.Sprintf("%.2f★", *deal.AverageRating)
			}
		}
		if _, err := fmt.Fprintf(out, "%-38s\t$%-8.2f\t%-14s\t%-12s\t%s\n",
			truncate(deal.Slug, 38), deal.Price, rating, truncate(deal.ListingType, 12), stockLabel(deal)); err != nil {
			return err
		}
	}
	return nil
}

func writeIdealDealsChinese(out io.Writer, report dealsReport, markdown bool) error {
	if markdown {
		return writeIdealDealsChineseMarkdown(out, report)
	}
	if len(report.Deals) == 0 {
		_, err := fmt.Fprintln(out, "未找到符合条件的优质 Deal。")
		return err
	}
	header := fmt.Sprintf("=== 精选 AppSumo 优质 Deal 推荐榜单（共 %d 款）===\n\n", len(report.Deals))
	if _, err := fmt.Fprint(out, header); err != nil {
		return err
	}
	for i, d := range report.Deals {
		name := d.Name
		if name == "" {
			name = d.Slug
		}
		rating := "-"
		reviews := 0
		if d.AverageRating != nil {
			rating = fmt.Sprintf("%.2f", *d.AverageRating)
		}
		if d.ReviewCount != nil {
			reviews = *d.ReviewCount
		}
		discountStr := ""
		if pct := d.DiscountPercent(); pct > 0 {
			discountStr = fmt.Sprintf(" (官方原价 $%.2f，立省 %.0f%%)", d.OriginalPrice, pct)
		}
		link := d.DealURL()

		fmt.Fprintf(out, "[%d] %s (%s)\n", i+1, name, d.Slug)
		fmt.Fprintf(out, "    ★ %s 星 (%d 条买家真实评价) | 终身价格: $%.2f%s\n", rating, reviews, d.Price, discountStr)
		if d.Category != "" {
			fmt.Fprintf(out, "    📂 分类: %s | 授权模式: %s\n", d.Category, d.ListingType)
		}
		if link != "" {
			fmt.Fprintf(out, "    🔗 直达链接: %s\n", link)
		}
		desc := d.ValueProp
		if desc == "" {
			desc = d.CardDescription
		}
		if desc != "" {
			fmt.Fprintf(out, "    💡 核心亮点: %s\n", desc)
		}
		if len(d.BestFor) > 0 {
			fmt.Fprintf(out, "    🎯 适合人群: %s\n", strings.Join(d.BestFor, ", "))
		}
		if len(d.AlternativeTo) > 0 {
			fmt.Fprintf(out, "    🔄 对标知名竞品: %s\n", strings.Join(d.AlternativeTo, ", "))
		}
		if d.CodesRemaining != nil {
			fmt.Fprintf(out, "    📦 授权库存: 剩余 %d codes\n", *d.CodesRemaining)
		}
		fmt.Fprintln(out)
	}
	return nil
}

func writeIdealDealsChineseMarkdown(out io.Writer, report dealsReport) error {
	if len(report.Deals) == 0 {
		_, err := fmt.Fprintln(out, "_未找到符合条件的优质 Deal。_")
		return err
	}
	fmt.Fprintf(out, "## 精选 AppSumo 优质 Deal 推荐榜单（共 %d 款）\n\n", len(report.Deals))
	for i, d := range report.Deals {
		name := d.Name
		if name == "" {
			name = d.Slug
		}
		rating := "-"
		reviews := 0
		if d.AverageRating != nil {
			rating = fmt.Sprintf("%.2f", *d.AverageRating)
		}
		if d.ReviewCount != nil {
			reviews = *d.ReviewCount
		}
		discountStr := ""
		if pct := d.DiscountPercent(); pct > 0 {
			discountStr = fmt.Sprintf(" (原价 `$%.2f`，立省 **%.0f%%**)", d.OriginalPrice, pct)
		}
		link := d.DealURL()

		fmt.Fprintf(out, "### %d. [%s](%s) — ★ %s (%d 条评价)\n\n", i+1, name, link, rating, reviews)
		fmt.Fprintf(out, "- **终身价格**: `$%.2f`%s\n", d.Price, discountStr)
		if d.Category != "" {
			fmt.Fprintf(out, "- **所属分类**: %s (%s)\n", d.Category, d.ListingType)
		}
		desc := d.ValueProp
		if desc == "" {
			desc = d.CardDescription
		}
		if desc != "" {
			fmt.Fprintf(out, "- **核心亮点**: %s\n", desc)
		}
		if len(d.BestFor) > 0 {
			fmt.Fprintf(out, "- **适用人群**: %s\n", strings.Join(d.BestFor, ", "))
		}
		if len(d.AlternativeTo) > 0 {
			fmt.Fprintf(out, "- **对标知名竞品**: *%s*\n", strings.Join(d.AlternativeTo, ", "))
		}
		if d.CodesRemaining != nil {
			fmt.Fprintf(out, "- **授权库存**: 剩余 `%d` codes\n", *d.CodesRemaining)
		}
		fmt.Fprintln(out)
	}
	return nil
}

// stockLabel never renders an unknown stock count as a number. The catalog
// reports codes_remaining 0 for deals that do not sell codes at all, so "0 left"
// would be a sold-out claim about a fully available deal.
func stockLabel(deal appsumo.Deal) string {
	if deal.CodesRemaining == nil {
		return ""
	}
	return fmt.Sprintf("%d codes left", *deal.CodesRemaining)
}

func writeDealChangesText(out io.Writer, from, to string, beforeCount, afterCount int, changes []appsumo.DealChange) error {
	if _, err := fmt.Fprintf(out, "%s (%d deals) -> %s (%d deals)\n\n", from, beforeCount, to, afterCount); err != nil {
		return err
	}
	if len(changes) == 0 {
		_, err := fmt.Fprintln(out, "no changes")
		return err
	}
	for _, change := range changes {
		switch change.Kind {
		case "changed":
			if _, err := fmt.Fprintf(out, "changed\t%-38s\t%s %s -> %s\n",
				truncate(change.Slug, 38), change.Field, change.Before, change.After); err != nil {
				return err
			}
		default:
			if _, err := fmt.Fprintf(out, "%-7s\t%-38s\t%s\n",
				change.Kind, truncate(change.Slug, 38), change.Name); err != nil {
				return err
			}
		}
	}
	return nil
}
