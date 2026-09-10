package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vecyang1/appsumo-cli/internal/appsumo"
)

const defaultNotionPageID = "3d3e1b43-2393-81f1-a87d-c75cb340e5e1"

func (rt *runtime) dealsSyncNotionCmd() *cobra.Command {
	var (
		pageID     string
		token      string
		minRating  float64
		minReviews int
		limit      int
		local      bool
		query      string
		category   string
	)
	cmd := &cobra.Command{
		Use:   "sync-notion",
		Short: "Sync ideal deal recommendations to Notion",
		Long: "Discover top-rated AppSumo deals and append rich recommendation cards to a Notion page.\n\n" +
			"Credentials are read from --token, NOTION_TOKEN / NOTION_API_KEY environment variables,\n" +
			"or the local notion-mcp-connector configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if minRating <= 0 {
				minRating = 4.8
			}
			if minReviews <= 0 {
				minReviews = 50
			}
			if limit <= 0 {
				limit = 10
			}

			notionToken := resolveNotionToken(token)
			if notionToken == "" {
				return fmt.Errorf("Notion token not found in --token, NOTION_TOKEN / NOTION_API_KEY, or local config")
			}

			targetPageID := resolveNotionPageID(pageID)
			if targetPageID == "" {
				return fmt.Errorf("Notion page ID not specified")
			}

			var deals []appsumo.Deal
			if local {
				db, err := rt.openStore(cmd.Context())
				if err != nil {
					return err
				}
				defer db.Close()
				deals, err = db.IdealDealsQuery(cmd.Context(), appsumo.DealsQuery{
					Query:      query,
					Category:   category,
					MinRating:  minRating,
					MinReviews: minReviews,
					Limit:      limit,
				})
				if err != nil {
					return err
				}
			} else {
				res, err := rt.publicClient().FetchAllDealsQuery(cmd.Context(), appsumo.DealsQuery{
					Query:      query,
					Category:   category,
					Sort:       appsumo.DealsSortRating,
					MinRating:  minRating,
					MinReviews: minReviews,
					Limit:      limit,
				})
				if err != nil {
					return err
				}
				deals = res.Deals
			}

			if len(deals) == 0 {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "No ideal deals found to sync.\n"); err != nil {
					return err
				}
				return nil
			}

			blocks := buildNotionRecommendationBlocks(deals, minRating, minReviews)
			if err := pushNotionBlocks(cmd.Context(), notionToken, targetPageID, blocks); err != nil {
				return fmt.Errorf("push to notion: %w", err)
			}

			if rt.asJSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"synced_deals": len(deals),
					"page_id":      targetPageID,
					"blocks":       len(blocks),
				})
			}

			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Successfully synced %d ideal deals (%d Notion blocks) to page %s\n",
				len(deals), len(blocks), targetPageID)
			return err
		},
	}
	cmd.Flags().StringVar(&pageID, "page-id", "", "Notion target page ID (default from env NOTION_PAGE_ID or default page)")
	cmd.Flags().StringVar(&token, "token", "", "Notion API secret token")
	cmd.Flags().Float64Var(&minRating, "min-rating", 4.8, "Minimum average rating (e.g. 4.8)")
	cmd.Flags().IntVar(&minReviews, "min-reviews", 50, "Minimum review count (e.g. 50)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Number of deals to recommend (default 10)")
	cmd.Flags().BoolVar(&local, "local", false, "Use local SQLite snapshot instead of live catalog API")
	cmd.Flags().StringVar(&query, "query", "", "Filter deals by search keyword")
	cmd.Flags().StringVar(&category, "category", "", "Filter deals by category")
	return cmd
}

func resolveNotionToken(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if tok := os.Getenv("NOTION_TOKEN"); tok != "" {
		return tok
	}
	if tok := os.Getenv("NOTION_API_KEY"); tok != "" {
		return tok
	}
	home, err := os.UserHomeDir()
	if err == nil {
		envPath := filepath.Join(home, ".gemini", "antigravity", "skills", "notion-mcp-connector", ".env")
		if data, err := os.ReadFile(envPath); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "NOTION_TOKEN=") || strings.HasPrefix(line, "NOTION_API_KEY=") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
						if val != "" {
							return val
						}
					}
				}
			}
		}
	}
	return ""
}

func resolveNotionPageID(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if pid := os.Getenv("NOTION_PAGE_ID"); pid != "" {
		return pid
	}
	return defaultNotionPageID
}

func buildNotionRecommendationBlocks(deals []appsumo.Deal, minRating float64, minReviews int) []map[string]any {
	var blocks []map[string]any

	nowStr := time.Now().Format("2006-01-02 15:04:05 MST")
	calloutText := fmt.Sprintf("AppSumo 优质 Deal 实时推荐榜单（更新时间：%s）\n通过 AppSumo CLI 原生工具链自动甄选，共推荐 %d 款经过真实社群验证的高分终身特惠产品。", nowStr, len(deals))
	blocks = append(blocks, notionCallout(calloutText, "🔥"))

	sectionTitle := fmt.Sprintf("精选 AppSumo 优质产品推荐榜单 (Ideal Deals Top %d)", len(deals))
	blocks = append(blocks, notionHeading2(sectionTitle))

	filterDesc := []map[string]any{
		notionText("筛选标准：", true, false, false, ""),
		notionText(fmt.Sprintf("买家平均评分 ≥ %.2f 星，真实评价数 ≥ %d 条，具备终身授权 (LTD) 与高性价比。", minRating, minReviews), false, false, false, ""),
	}
	blocks = append(blocks, notionParagraph(filterDesc))
	blocks = append(blocks, notionDivider())

	for idx, d := range deals {
		name := d.Name
		if name == "" {
			name = d.Slug
		}
		rating := 0.0
		if d.AverageRating != nil {
			rating = *d.AverageRating
		}
		reviews := 0
		if d.ReviewCount != nil {
			reviews = *d.ReviewCount
		}
		link := d.DealURL()
		discountPct := 0
		if d.OriginalPrice > d.Price && d.OriginalPrice > 0 {
			discountPct = int(math.Round(((d.OriginalPrice - d.Price) / d.OriginalPrice) * 100))
		}

		// Heading for deal
		headingText := fmt.Sprintf("%d. %s — ★ %.2f (%d 条真实评价)", idx+1, name, rating, reviews)
		blocks = append(blocks, notionHeading3(headingText))

		// Price and meta
		metaParts := []map[string]any{
			notionText("💰 终身特惠: ", true, false, false, ""),
			notionText(fmt.Sprintf("$%.2f", d.Price), true, false, false, ""),
		}
		if discountPct > 0 {
			metaParts = append(metaParts, notionText(fmt.Sprintf(" (官方原价 $%.2f，立省 %d%%) | ", d.OriginalPrice, discountPct), false, false, false, ""))
		} else {
			metaParts = append(metaParts, notionText(" | ", false, false, false, ""))
		}
		if d.Category != "" {
			metaParts = append(metaParts, notionText("📂 分类: ", true, false, false, ""))
			metaParts = append(metaParts, notionText(d.Category+" | ", false, false, false, ""))
		}
		if link != "" {
			metaParts = append(metaParts, notionText("🔗 直达链接: ", true, false, false, ""))
			metaParts = append(metaParts, notionText("点击前往 AppSumo", false, false, false, link))
		}
		blocks = append(blocks, notionParagraph(metaParts))

		desc := d.ValueProp
		if desc == "" {
			desc = d.CardDescription
		}
		if desc != "" {
			blocks = append(blocks, notionBullet([]map[string]any{
				notionText("核心亮点: ", true, false, false, ""),
				notionText(desc, false, false, false, ""),
			}))
		}
		if len(d.BestFor) > 0 {
			blocks = append(blocks, notionBullet([]map[string]any{
				notionText("适合人群: ", true, false, false, ""),
				notionText(strings.Join(d.BestFor, ", "), false, false, false, ""),
			}))
		}
		if len(d.AlternativeTo) > 0 {
			blocks = append(blocks, notionBullet([]map[string]any{
				notionText("对标知名竞品: ", true, false, false, ""),
				notionText(strings.Join(d.AlternativeTo, ", "), false, true, false, ""),
			}))
		}
		if d.CodesRemaining != nil {
			blocks = append(blocks, notionBullet([]map[string]any{
				notionText("授权库存: ", true, false, false, ""),
				notionText(fmt.Sprintf("剩余 %d codes", *d.CodesRemaining), false, false, false, ""),
			}))
		}
		blocks = append(blocks, notionDivider())
	}

	return blocks
}

func notionText(content string, bold, italic, code bool, link string) map[string]any {
	var linkObj map[string]any
	if link != "" {
		linkObj = map[string]any{"url": link}
	}
	return map[string]any{
		"type": "text",
		"text": map[string]any{
			"content": content,
			"link":    linkObj,
		},
		"annotations": map[string]any{
			"bold":          bold,
			"italic":        italic,
			"strikethrough": false,
			"underline":     false,
			"code":          code,
			"color":         "default",
		},
	}
}

func notionCallout(text, icon string) map[string]any {
	return map[string]any{
		"object": "block",
		"type":   "callout",
		"callout": map[string]any{
			"icon": map[string]any{
				"type":  "emoji",
				"emoji": icon,
			},
			"rich_text": []map[string]any{notionText(text, false, false, false, "")},
		},
	}
}

func notionHeading2(text string) map[string]any {
	return map[string]any{
		"object": "block",
		"type":   "heading_2",
		"heading_2": map[string]any{
			"rich_text": []map[string]any{notionText(text, true, false, false, "")},
		},
	}
}

func notionHeading3(text string) map[string]any {
	return map[string]any{
		"object": "block",
		"type":   "heading_3",
		"heading_3": map[string]any{
			"rich_text": []map[string]any{notionText(text, true, false, false, "")},
		},
	}
}

func notionParagraph(parts []map[string]any) map[string]any {
	return map[string]any{
		"object":    "block",
		"type":      "paragraph",
		"paragraph": map[string]any{"rich_text": parts},
	}
}

func notionBullet(parts []map[string]any) map[string]any {
	return map[string]any{
		"object":             "block",
		"type":               "bulleted_list_item",
		"bulleted_list_item": map[string]any{"rich_text": parts},
	}
}

func notionDivider() map[string]any {
	return map[string]any{
		"object":  "block",
		"type":    "divider",
		"divider": map[string]any{},
	}
}

func pushNotionBlocks(ctx context.Context, token, pageID string, blocks []map[string]any) error {
	client := &http.Client{Timeout: 30 * time.Second}
	url := fmt.Sprintf("https://api.notion.com/v1/blocks/%s/children", pageID)
	chunkSize := 40

	for i := 0; i < len(blocks); i += chunkSize {
		end := i + chunkSize
		if end > len(blocks) {
			end = len(blocks)
		}
		chunk := blocks[i:end]

		payload, err := json.Marshal(map[string]any{"children": chunk})
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Notion-Version", "2022-06-28")
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("Notion API status %d: %s", resp.StatusCode, string(body))
		}
	}
	return nil
}
