package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"
	"github.com/vecyang1/appsumo-cli/internal/redact"
	"github.com/vecyang1/appsumo-cli/internal/store"

	"github.com/spf13/cobra"
)

type Options struct {
	BaseURL    string
	Cookie     string
	CookieFile string
	DBPath     string
	Out        io.Writer
	Err        io.Writer
	HTTPClient *http.Client
}

type runtime struct {
	options Options
	asJSON  bool
}

func NewRoot(options Options) *cobra.Command {
	if options.Out == nil {
		options.Out = os.Stdout
	}
	if options.Err == nil {
		options.Err = os.Stderr
	}
	rt := &runtime{options: options}

	cmd := &cobra.Command{
		Use:           "appsumo",
		Short:         "Read-only AppSumo buyer-account CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(options.Out)
	cmd.SetErr(options.Err)
	cmd.PersistentFlags().BoolVar(&rt.asJSON, "json", false, "Output JSON")
	cmd.PersistentFlags().StringVar(&rt.options.BaseURL, "base-url", options.BaseURL, "AppSumo base URL")
	cmd.PersistentFlags().StringVar(&rt.options.CookieFile, "cookie-file", options.CookieFile, "Ignored local file containing a Cookie header")
	cmd.PersistentFlags().StringVar(&rt.options.DBPath, "db", options.DBPath, "SQLite database path")

	cmd.AddCommand(rt.authCmd())
	cmd.AddCommand(rt.productsCmd())
	cmd.AddCommand(rt.syncCmd())
	cmd.AddCommand(rt.searchCmd())
	cmd.AddCommand(rt.sqlCmd())
	return cmd
}

func (rt *runtime) authCmd() *cobra.Command {
	auth := &cobra.Command{Use: "auth", Short: "Authentication helpers"}
	status := &cobra.Command{
		Use:   "status",
		Short: "Check AppSumo session status without printing credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.client()
			if err != nil {
				return err
			}
			status, err := client.AuthStatus(cmd.Context())
			if err != nil {
				return err
			}
			if rt.asJSON {
				return writeJSON(cmd.OutOrStdout(), status)
			}
			if status.Authenticated {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "authenticated")
			} else {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "not authenticated")
			}
			return err
		},
	}
	auth.AddCommand(status)
	return auth
}

func (rt *runtime) productsCmd() *cobra.Command {
	products := &cobra.Command{Use: "products", Short: "Read AppSumo buyer products"}
	products.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List live products",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, _, err := rt.fetchProducts(cmd.Context())
			if err != nil {
				return err
			}
			return rt.writeProducts(cmd.OutOrStdout(), items)
		},
	})
	products.AddCommand(&cobra.Command{
		Use:   "search <query>",
		Short: "Search live products",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			items, _, err := rt.fetchProducts(cmd.Context())
			if err != nil {
				return err
			}
			return rt.writeProducts(cmd.OutOrStdout(), filterProducts(items, args[0]))
		},
	})
	var format string
	export := &cobra.Command{
		Use:   "export",
		Short: "Export live products with default secret redaction",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := rt.client()
			if err != nil {
				return err
			}
			switch strings.ToLower(format) {
			case "csv":
				data, err := client.FetchProductsCSV(cmd.Context())
				if err != nil {
					return err
				}
				data, err = redact.CSV(data)
				if err != nil {
					return err
				}
				_, err = cmd.OutOrStdout().Write(data)
				return err
			case "json":
				items, _, err := client.FetchAllProducts(cmd.Context())
				if err != nil {
					return err
				}
				return writeRedactedJSON(cmd.OutOrStdout(), items)
			default:
				return fmt.Errorf("unsupported export format %q", format)
			}
		},
	}
	export.Flags().StringVar(&format, "format", "json", "Export format: json or csv")
	products.AddCommand(export)
	return products
}

func (rt *runtime) syncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync live products into local SQLite",
		RunE: func(cmd *cobra.Command, args []string) error {
			items, total, err := rt.fetchProducts(cmd.Context())
			if err != nil {
				return err
			}
			db, err := rt.openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.UpsertProducts(cmd.Context(), items); err != nil {
				return err
			}
			if rt.asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"synced": len(items), "total": total})
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "synced %d products (api total %d)\n", len(items), total)
			return err
		},
	}
}

func (rt *runtime) searchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search synced products",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := rt.openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			items, err := db.SearchProducts(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return rt.writeProducts(cmd.OutOrStdout(), items)
		},
	}
}

func (rt *runtime) sqlCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sql <select-query>",
		Short: "Run read-only SQL against synced products",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := rt.openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.QueryReadOnly(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if rt.asJSON {
				return writeJSON(cmd.OutOrStdout(), rows)
			}
			for _, row := range rows {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), row); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func (rt *runtime) fetchProducts(ctx context.Context) ([]appsumo.Product, int, error) {
	client, err := rt.client()
	if err != nil {
		return nil, 0, err
	}
	return client.FetchAllProducts(ctx)
}

func (rt *runtime) client() (*appsumo.Client, error) {
	cookie, err := rt.cookie()
	if err != nil {
		return nil, err
	}
	baseURL := firstNonEmpty(rt.options.BaseURL, os.Getenv("APPSUMO_BASE_URL"), appsumo.DefaultBaseURL)
	return appsumo.NewClient(appsumo.ClientOptions{
		BaseURL:    baseURL,
		Cookie:     cookie,
		HTTPClient: rt.options.HTTPClient,
	}), nil
}

func (rt *runtime) cookie() (string, error) {
	if rt.options.Cookie != "" {
		return strings.TrimSpace(rt.options.Cookie), nil
	}
	if env := os.Getenv("APPSUMO_COOKIE"); env != "" {
		return strings.TrimSpace(env), nil
	}
	if rt.options.CookieFile != "" {
		data, err := os.ReadFile(rt.options.CookieFile)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	}
	return "", nil
}

func (rt *runtime) openStore(ctx context.Context) (*store.DB, error) {
	dbPath, err := rt.dbPath()
	if err != nil {
		return nil, err
	}
	return store.Open(ctx, dbPath)
}

func (rt *runtime) dbPath() (string, error) {
	if rt.options.DBPath != "" {
		return rt.options.DBPath, nil
	}
	if env := os.Getenv("APPSUMO_DB_PATH"); env != "" {
		return env, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	path := filepath.Join(configDir, "appsumo-cli")
	if err := os.MkdirAll(path, 0o700); err != nil {
		return "", fmt.Errorf("create default database directory %s: %w", path, err)
	}
	return filepath.Join(path, "appsumo.db"), nil
}

func (rt *runtime) writeProducts(out io.Writer, products []appsumo.Product) error {
	if rt.asJSON {
		return writeJSON(out, products)
	}
	for _, product := range products {
		if _, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", product.Name, product.Status, product.PlanName, product.PurchaseDate); err != nil {
			return err
		}
	}
	return nil
}

func filterProducts(products []appsumo.Product, query string) []appsumo.Product {
	query = strings.ToLower(strings.TrimSpace(query))
	var filtered []appsumo.Product
	for _, product := range products {
		haystack := strings.ToLower(strings.Join([]string{
			product.Name,
			product.Slug,
			product.Status,
			product.PlanName,
			product.SupportEmail,
		}, "\n"))
		if strings.Contains(haystack, query) {
			filtered = append(filtered, product)
		}
	}
	return filtered
}

func writeJSON(out io.Writer, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(encoded))
	return err
}

func writeRedactedJSON(out io.Writer, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var generic any
	if err := json.Unmarshal(encoded, &generic); err != nil {
		return err
	}
	return writeJSON(out, redact.JSONValue(generic))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
