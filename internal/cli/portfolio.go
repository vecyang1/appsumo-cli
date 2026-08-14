package cli

import (
	"fmt"
	"io"
	"sort"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"

	"github.com/spf13/cobra"
)

func (rt *runtime) portfolioCmd() *cobra.Command {
	var live bool
	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Summarise the account: status, redemption, licensing, refund window",
		Long: "Roll up the buyer account into counts worth acting on.\n\n" +
			"Redemption is derived from redeem_date, not from the API's is_redeemed flag.\n" +
			"That flag reads false on every product of a live 70-product account, including\n" +
			"all 36 the buyer has activated, so a rollup keyed on it reports an action list\n" +
			"of 70 items that do not exist. When the flag disagrees with redeem_date this\n" +
			"command says so on stderr rather than quietly picking one.\n\n" +
			"Reads the local database by default; --live re-reads the account API.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			products, source, err := rt.portfolioProducts(cmd, live)
			if err != nil {
				return err
			}
			summary := appsumo.Summarise(products)

			for _, warning := range summary.Warnings {
				if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning); err != nil {
					return err
				}
			}
			if rt.asJSON {
				return writeRedactedJSON(cmd.OutOrStdout(), map[string]any{
					"source": source, "summary": summary,
				})
			}
			return writePortfolioText(cmd.OutOrStdout(), source, summary)
		},
	}
	cmd.Flags().BoolVar(&live, "live", false, "Read the account API instead of the local database")
	return cmd
}

func (rt *runtime) portfolioProducts(cmd *cobra.Command, live bool) ([]appsumo.Product, string, error) {
	if live {
		products, _, err := rt.fetchProducts(cmd.Context())
		return products, "live", err
	}
	db, err := rt.openStore(cmd.Context())
	if err != nil {
		return nil, "", err
	}
	defer db.Close()
	// An empty query matches every row; SearchProducts is the existing read path.
	products, err := db.SearchProducts(cmd.Context(), "")
	return products, "local", err
}

func writePortfolioText(out io.Writer, source string, summary appsumo.PortfolioSummary) error {
	if _, err := fmt.Fprintf(out, "%d products (%s)\n\n", summary.Products, source); err != nil {
		return err
	}
	if err := writeCounts(out, "status", summary.ByStatus); err != nil {
		return err
	}
	if err := writeCounts(out, "redemption", summary.ByRedemption); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "\n%-14s %d active, %d transferable\n", "licensing", summary.ActiveLicenses, summary.Transferable); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "%-14s %d of %d still refundable\n", "refund window", summary.Refundable, summary.Products); err != nil {
		return err
	}
	if summary.OldestPurchase != "" {
		if _, err := fmt.Fprintf(out, "%-14s %s to %s\n", "purchases", truncate(summary.OldestPurchase, 10), truncate(summary.NewestPurchase, 10)); err != nil {
			return err
		}
	}

	if len(summary.AwaitingRedemption) == 0 {
		_, err := fmt.Fprintln(out, "\nnothing awaiting redemption")
		return err
	}
	if _, err := fmt.Fprintf(out, "\nawaiting redemption (%d)\n", len(summary.AwaitingRedemption)); err != nil {
		return err
	}
	for _, item := range summary.AwaitingRedemption {
		if _, err := fmt.Fprintf(out, "  %-40s\t%s\t%s\n",
			truncate(item.Name, 40), item.Status, truncate(item.Purchase, 10)); err != nil {
			return err
		}
	}
	return nil
}

func writeCounts(out io.Writer, label string, counts map[string]int) error {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if _, err := fmt.Fprintf(out, "%-14s", label); err != nil {
		return err
	}
	for _, key := range keys {
		if _, err := fmt.Fprintf(out, " %s %d ", key, counts[key]); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out)
	return err
}
