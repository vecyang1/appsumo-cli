package cli

import (
	"fmt"
	"io"
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
	deals.AddCommand(rt.dealsSyncCmd())
	deals.AddCommand(rt.dealsDiffCmd())
	return deals
}

func (rt *runtime) dealsListCmd() *cobra.Command {
	var (
		limit    int
		pageSize int
		sort     string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List every live deal in the public catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := rt.publicClient().FetchAllDeals(cmd.Context(), pageSize, sort, limit)
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
	declared := "unknown"
	if report.Fetch.DeclaredTotal != nil {
		declared = fmt.Sprintf("%d", *report.Fetch.DeclaredTotal)
	}
	if _, err := fmt.Fprintf(out, "%d of %s live deals in %d requests (sort=%s)\n\n",
		report.Fetch.UniqueDeals, declared, report.Fetch.Requests, report.Fetch.Sort); err != nil {
		return err
	}
	for _, deal := range report.Deals {
		rating := "-"
		if deal.AverageRating != nil {
			rating = fmt.Sprintf("%.2f", *deal.AverageRating)
		}
		if _, err := fmt.Fprintf(out, "%-38s\t$%-8.2f\t%-4s\t%-12s\t%s\n",
			truncate(deal.Slug, 38), deal.Price, rating, truncate(deal.ListingType, 12), stockLabel(deal)); err != nil {
			return err
		}
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
