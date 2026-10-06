package appsumo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CleanProductSlug normalises a product slug or URL into a clean AppSumo slug.
func CleanProductSlug(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err == nil {
			path := strings.Trim(u.Path, "/")
			parts := strings.Split(path, "/")
			for i, p := range parts {
				if p == "products" && i+1 < len(parts) {
					return parts[i+1]
				}
			}
			if len(parts) > 0 {
				return parts[len(parts)-1]
			}
		}
	}
	raw = strings.Trim(raw, "/")
	parts := strings.Split(raw, "/")
	return parts[len(parts)-1]
}

type PlanFeatureItem struct {
	Feature string `json:"feature"`
}

type DealPlan struct {
	ID            flexInt64         `json:"id"`
	Tier          int               `json:"tier"`
	PlanType      string            `json:"plan_type"`
	Price         string            `json:"price"`
	OriginalPrice string            `json:"original_price"`
	IsActive      bool              `json:"is_active"`
	PlanFeatures  []PlanFeatureItem `json:"plan_features"`
}

type DealFeatureEntry struct {
	Entry  any       `json:"entry"`
	PlanID flexInt64 `json:"plan_id"`
}

type DealFeatureMatrix struct {
	Feature string             `json:"feature"`
	Order   int                `json:"order"`
	Tooltip *string            `json:"tooltip"`
	Entries []DealFeatureEntry `json:"entries"`
}

type DealTermItem struct {
	ID   flexInt64 `json:"id"`
	Text string    `json:"text"`
}

type CommonFeatureItem struct {
	ID   flexInt64 `json:"id"`
	Text string    `json:"text"`
}

type DealFAQ struct {
	ID       flexInt64 `json:"id"`
	Question string    `json:"question"`
	Answer   string    `json:"answer"`
}

type FounderLink struct {
	ID    flexInt64 `json:"id"`
	Type  string    `json:"type"`
	Title string    `json:"title"`
	URL   string    `json:"url"`
}

type FounderPostUser struct {
	Username    string `json:"username"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Avatar      string `json:"avatar"`
	LinkedinURL string `json:"linkedin_url"`
}

type FounderPost struct {
	ID      flexInt64       `json:"id"`
	Title   string          `json:"title"`
	Message string          `json:"message"`
	Created string          `json:"created"`
	User    FounderPostUser `json:"user"`
}

type StageDetail struct {
	Label       string `json:"label"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

type FounderCompanyData struct {
	CompanySize    string      `json:"company_size"`
	Headquarters   string      `json:"headquarters"`
	FoundedAt      string      `json:"founded_at"`
	ProductStage   StageDetail `json:"product_stage"`
	FinancialStage StageDetail `json:"financial_stage"`
	IsVerified     bool        `json:"is_verified"`
}

type FoundersInfo struct {
	Data  FounderCompanyData `json:"data"`
	Links []FounderLink      `json:"links"`
	Posts []FounderPost      `json:"posts"`
}

type OverviewMediaItem struct {
	HTMLBody string `json:"html_body"`
}

// ProductDealDetail holds complete specifications, company details, pricing tiers,
// terms, FAQs, and founder posts for a single AppSumo product deal.
type ProductDealDetail struct {
	ID                 flexInt64           `json:"id"`
	Slug               string              `json:"slug"`
	PublicName         string              `json:"public_name"`
	ProductURL         string              `json:"product_url"`
	AppSumoURL         string              `json:"appsumo_url"`
	DealStatus         string              `json:"deal_status"`
	RefundableDays     int                 `json:"refundable_days"`
	MoneyBackType      string              `json:"money_back_callout_type"`
	MediaURL           string              `json:"media_url"`
	FeaturedImageURL   string              `json:"featured_image_url"`
	ProductLogo        string              `json:"product_logo"`
	Ratings            *DealRatings        `json:"ratings"`
	OverviewMedia      []OverviewMediaItem `json:"overview_media"`
	DefaultPlanTerms   []DealTermItem      `json:"default_plan_terms"`
	CommonFeatures     []CommonFeatureItem `json:"common_features"`
	Plans              []DealPlan          `json:"plans"`
	DealFeaturesMatrix []DealFeatureMatrix `json:"dealfeatures"`
	Founders           FoundersInfo        `json:"founders"`
	FAQs               []DealFAQ           `json:"faqs"`
}

// FullProductArchive aggregates product deal specifications, all public reviews,
// and all public questions for offline caching, analysis, and LLM consumption.
type FullProductArchive struct {
	ScrapedAt string            `json:"scraped_at"`
	Slug      string            `json:"slug"`
	Deal      ProductDealDetail `json:"deal"`
	Reviews   []Review          `json:"reviews"`
	Questions []Question        `json:"questions"`
}

// FetchProductDeal fetches and extracts the full deal specification from the
// public product landing page's __NEXT_DATA__ island.
func (c *Client) FetchProductDeal(ctx context.Context, slug string) (*ProductDealDetail, error) {
	cleanSlug := CleanProductSlug(slug)
	if cleanSlug == "" {
		return nil, fmt.Errorf("product slug or url is required")
	}
	path := fmt.Sprintf("/products/%s/", cleanSlug)
	body, err := c.public().getHTML(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("fetch product page %s: %w", path, err)
	}

	match := nextDataPattern.FindSubmatch(body)
	if match == nil {
		return nil, fmt.Errorf("no __NEXT_DATA__ island on %s; the product page layout changed", path)
	}

	var payload struct {
		Props struct {
			PageProps struct {
				Deal struct {
					ID               flexInt64           `json:"id"`
					Slug             string              `json:"slug"`
					PublicName       string              `json:"public_name"`
					ProductURL       string              `json:"product_url"`
					DealStatus       string              `json:"deal_status"`
					RefundableDays   int                 `json:"refundable_days"`
					MoneyBackType    string              `json:"money_back_callout_type"`
					MediaURL         string              `json:"media_url"`
					FeaturedImageURL string              `json:"featured_image_url"`
					ProductLogo      string              `json:"product_logo"`
					OverviewMedia    []OverviewMediaItem `json:"overview_media"`
					DefaultPlanTerms []DealTermItem      `json:"default_plan_terms"`
					CommonFeatures   []CommonFeatureItem `json:"common_features"`
					Plans            []DealPlan          `json:"plans"`
					DealFeatures     []DealFeatureMatrix `json:"dealfeatures"`
					Founders         FoundersInfo        `json:"founders"`
					FAQs             []DealFAQ           `json:"faqs"`
					DealReview       *struct {
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
		return nil, fmt.Errorf("no deal id for product %q; check the slug at %s%s", cleanSlug, c.baseURL, path)
	}

	detail := &ProductDealDetail{
		ID:                 deal.ID,
		Slug:               firstNonBlank(deal.Slug, cleanSlug),
		PublicName:         deal.PublicName,
		ProductURL:         deal.ProductURL,
		AppSumoURL:         c.baseURL + path,
		DealStatus:         deal.DealStatus,
		RefundableDays:     deal.RefundableDays,
		MoneyBackType:      deal.MoneyBackType,
		MediaURL:           deal.MediaURL,
		FeaturedImageURL:   deal.FeaturedImageURL,
		ProductLogo:        deal.ProductLogo,
		OverviewMedia:      deal.OverviewMedia,
		DefaultPlanTerms:   deal.DefaultPlanTerms,
		CommonFeatures:     deal.CommonFeatures,
		Plans:              deal.Plans,
		DealFeaturesMatrix: deal.DealFeatures,
		Founders:           deal.Founders,
		FAQs:               deal.FAQs,
	}

	if review := deal.DealReview; review != nil {
		dist := map[string]int{}
		for label, val := range map[string]*int{
			"1": review.OneTaco,
			"2": review.TwoTacos,
			"3": review.ThreeTacos,
			"4": review.FourTacos,
			"5": review.FiveTacos,
		} {
			if val != nil {
				dist[label] = *val
			}
		}
		detail.Ratings = &DealRatings{
			ReviewCount:   review.ReviewCount,
			AverageRating: review.AverageRating,
			Distribution:  dist,
		}
	}

	return detail, nil
}

// ScrapeProductArchive collects the complete product package: deal specifications,
// all reviews, and all questions, returning a structured FullProductArchive.
func (c *Client) ScrapeProductArchive(
	ctx context.Context,
	slug string,
	includeReviews bool,
	includeQuestions bool,
) (*FullProductArchive, error) {
	deal, err := c.FetchProductDeal(ctx, slug)
	if err != nil {
		return nil, err
	}

	archive := &FullProductArchive{
		ScrapedAt: time.Now().UTC().Format(time.RFC3339),
		Slug:      deal.Slug,
		Deal:      *deal,
		Reviews:   []Review{},
		Questions: []Question{},
	}

	if includeReviews {
		reviewsResult, err := c.FetchAllReviews(ctx, ReviewsQuery{
			DealID: int64(deal.ID),
		}, 0)
		if err != nil {
			return nil, fmt.Errorf("fetch all reviews for %s: %w", deal.Slug, err)
		}
		archive.Reviews = reviewsResult.Reviews
	}

	if includeQuestions {
		questionsResult, err := c.FetchAllQuestions(ctx, QuestionsQuery{
			DealID: int64(deal.ID),
		}, 0)
		if err != nil {
			return nil, fmt.Errorf("fetch all questions for %s: %w", deal.Slug, err)
		}
		archive.Questions = questionsResult.Questions
	}

	return archive, nil
}

// SaveToDirectory saves all components of the archive into the specified directory:
// deal.json, reviews.json, questions.json, faqs.json, founders.json, full_archive.json,
// and DOSSIER.md.
func (a *FullProductArchive) SaveToDirectory(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create output directory %s: %w", dir, err)
	}

	writeJSON := func(filename string, data any) error {
		path := filepath.Join(dir, filename)
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}

	if err := writeJSON("deal.json", a.Deal); err != nil {
		return fmt.Errorf("save deal.json: %w", err)
	}
	if err := writeJSON("reviews.json", a.Reviews); err != nil {
		return fmt.Errorf("save reviews.json: %w", err)
	}
	if err := writeJSON("questions.json", a.Questions); err != nil {
		return fmt.Errorf("save questions.json: %w", err)
	}
	if err := writeJSON("faqs.json", a.Deal.FAQs); err != nil {
		return fmt.Errorf("save faqs.json: %w", err)
	}
	if err := writeJSON("founders.json", a.Deal.Founders); err != nil {
		return fmt.Errorf("save founders.json: %w", err)
	}
	if err := writeJSON("full_archive.json", a); err != nil {
		return fmt.Errorf("save full_archive.json: %w", err)
	}

	// Write Markdown Dossier
	mdContent := a.GenerateMarkdownDossier()
	dossierPath := filepath.Join(dir, "DOSSIER.md")
	if err := os.WriteFile(dossierPath, []byte(mdContent), 0644); err != nil {
		return fmt.Errorf("save DOSSIER.md: %w", err)
	}

	summaryPath := filepath.Join(dir, "SUMMARY.md")
	_ = os.WriteFile(summaryPath, []byte(mdContent), 0644)

	return nil
}

// GenerateMarkdownDossier creates an agent-friendly, comprehensive Markdown dossier
// summarizing the product, tiers, terms, founders, FAQs, and review sentiment.
func (a *FullProductArchive) GenerateMarkdownDossier() string {
	var b strings.Builder
	deal := a.Deal

	b.WriteString(fmt.Sprintf("# AppSumo Product Dossier: %s\n\n", deal.PublicName))
	b.WriteString(fmt.Sprintf("> **Scraped At**: %s  \n", a.ScrapedAt))
	b.WriteString(fmt.Sprintf("> **Deal Slug**: `%s`  \n", deal.Slug))
	b.WriteString(fmt.Sprintf("> **AppSumo URL**: %s  \n", deal.AppSumoURL))
	b.WriteString(fmt.Sprintf("> **Official Website**: %s  \n", deal.ProductURL))
	b.WriteString(fmt.Sprintf("> **Deal Status**: **%s**  \n", firstNonBlank(deal.DealStatus, "Active/Ended")))
	b.WriteString(fmt.Sprintf("> **Money Back Guarantee**: %d Days\n\n", deal.RefundableDays))

	// Ratings
	b.WriteString("## 1. Ratings & Community Feedback Summary\n\n")
	if r := deal.Ratings; r != nil {
		count := 0
		if r.ReviewCount != nil {
			count = *r.ReviewCount
		}
		b.WriteString(fmt.Sprintf("- **Overall Rating**: ⭐ **%s / 5 Tacos** (%d reviews total)\n", r.AverageRating, count))
		b.WriteString("- **Rating Distribution**:\n")
		b.WriteString(fmt.Sprintf("  - 5 Tacos: %d\n", r.Distribution["5"]))
		b.WriteString(fmt.Sprintf("  - 4 Tacos: %d\n", r.Distribution["4"]))
		b.WriteString(fmt.Sprintf("  - 3 Tacos: %d\n", r.Distribution["3"]))
		b.WriteString(fmt.Sprintf("  - 2 Tacos: %d\n", r.Distribution["2"]))
		b.WriteString(fmt.Sprintf("  - 1 Taco:  %d\n", r.Distribution["1"]))
	}
	b.WriteString(fmt.Sprintf("- **Total Public Questions**: %d threads\n", len(a.Questions)))
	b.WriteString(fmt.Sprintf("- **Total Public Reviews Collected**: %d reviews\n\n", len(a.Reviews)))

	// Company & Founders
	b.WriteString("## 2. Company & Founder Background\n\n")
	fd := deal.Founders.Data
	b.WriteString(fmt.Sprintf("- **Founded Date**: %s\n", firstNonBlank(fd.FoundedAt, "N/A")))
	b.WriteString(fmt.Sprintf("- **Headquarters**: %s\n", firstNonBlank(fd.Headquarters, "N/A")))
	b.WriteString(fmt.Sprintf("- **Company Size**: %s\n", firstNonBlank(fd.CompanySize, "N/A")))
	b.WriteString(fmt.Sprintf("- **Product Stage**: %s (%s)\n", fd.ProductStage.Label, fd.ProductStage.Description))
	b.WriteString(fmt.Sprintf("- **Financial Stage**: %s (%s)\n", fd.FinancialStage.Label, fd.FinancialStage.Description))
	b.WriteString(fmt.Sprintf("- **Identity Verified**: %v (PandaDoc)\n", fd.IsVerified))

	if len(deal.Founders.Links) > 0 {
		b.WriteString("- **Official Links**:\n")
		for _, l := range deal.Founders.Links {
			b.WriteString(fmt.Sprintf("  - [%s](%s) (%s)\n", l.Title, l.URL, l.Type))
		}
	}
	b.WriteString("\n")

	// Pricing & License Tiers
	b.WriteString("## 3. Pricing & License Tiers\n\n")
	if len(deal.Plans) > 0 {
		b.WriteString("| Tier | Price | Original Price | Highlights / Key Limits |\n")
		b.WriteString("| :--- | :---: | :---: | :--- |\n")
		for _, p := range deal.Plans {
			featuresStr := []string{}
			for _, f := range p.PlanFeatures {
				cleanF := strings.ReplaceAll(f.Feature, "<b>", "**")
				cleanF = strings.ReplaceAll(cleanF, "</b>", "**")
				featuresStr = append(featuresStr, cleanF)
			}
			b.WriteString(fmt.Sprintf("| **Tier %d** | $%s | $%s | %s |\n",
				p.Tier, p.Price, p.OriginalPrice, strings.Join(featuresStr, "; ")))
		}
		b.WriteString("\n")
	}

	// Features Matrix
	if len(deal.DealFeaturesMatrix) > 0 {
		b.WriteString("### Feature Comparison Matrix\n\n")
		b.WriteString("| Feature | Tier 1 | Tier 2 | Tier 3 | Tier 4 | Tier 5 | Tier 6 |\n")
		b.WriteString("| :--- | :---: | :---: | :---: | :---: | :---: | :---: |\n")

		// Map plan IDs to Tiers
		planToTier := map[int64]int{}
		for _, p := range deal.Plans {
			planToTier[int64(p.ID)] = p.Tier
		}

		for _, m := range deal.DealFeaturesMatrix {
			tierValues := make([]string, 6)
			for i := range tierValues {
				tierValues[i] = "-"
			}
			for _, entry := range m.Entries {
				t := planToTier[int64(entry.PlanID)]
				if t >= 1 && t <= 6 {
					valStr := fmt.Sprintf("%v", entry.Entry)
					if valStr == "true" {
						valStr = "✅"
					} else if valStr == "false" {
						valStr = "❌"
					}
					tierValues[t-1] = valStr
				}
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s |\n",
				m.Feature, tierValues[0], tierValues[1], tierValues[2], tierValues[3], tierValues[4], tierValues[5]))
		}
		b.WriteString("\n")
	}

	// Deal Terms
	if len(deal.DefaultPlanTerms) > 0 {
		b.WriteString("## 4. Deal Terms & Conditions\n\n")
		for _, t := range deal.DefaultPlanTerms {
			b.WriteString(fmt.Sprintf("- %s\n", t.Text))
		}
		b.WriteString("\n")
	}

	// Common Features
	if len(deal.CommonFeatures) > 0 {
		b.WriteString("## 5. Common Features (Included Across All Plans)\n\n")
		for _, f := range deal.CommonFeatures {
			b.WriteString(fmt.Sprintf("- %s\n", f.Text))
		}
		b.WriteString("\n")
	}

	// From the Founders
	if len(deal.Founders.Posts) > 0 {
		b.WriteString("## 6. From the Founders (Announcements & Vision)\n\n")
		for i, post := range deal.Founders.Posts {
			b.WriteString(fmt.Sprintf("### Post %d: %s\n", i+1, post.Title))
			b.WriteString(fmt.Sprintf("- **Author**: %s (%s %s)\n", post.User.Username, post.User.FirstName, post.User.LastName))
			b.WriteString(fmt.Sprintf("- **Date**: %s\n", post.Created))
			cleanMsg := stripHTMLTags(post.Message)
			b.WriteString(fmt.Sprintf("\n> %s\n\n", strings.ReplaceAll(cleanMsg, "\n", "\n> ")))
		}
	}

	// FAQs
	if len(deal.FAQs) > 0 {
		b.WriteString("## 7. Frequently Asked Questions (FAQs)\n\n")
		for i, faq := range deal.FAQs {
			b.WriteString(fmt.Sprintf("#### Q%d: %s\n\n", i+1, faq.Question))
			b.WriteString(fmt.Sprintf("%s\n\n", faq.Answer))
		}
	}

	// Review Sentiment & Analysis
	if len(a.Reviews) > 0 {
		b.WriteString("## 8. Review Sentiment & Top Feedback Analysis\n\n")

		// Sort reviews by upvotes descending
		reviewsByVotes := make([]Review, len(a.Reviews))
		copy(reviewsByVotes, a.Reviews)
		sort.Slice(reviewsByVotes, func(i, j int) bool {
			vi, vj := 0, 0
			if reviewsByVotes[i].UpVotes != nil {
				vi = int(*reviewsByVotes[i].UpVotes)
			}
			if reviewsByVotes[j].UpVotes != nil {
				vj = int(*reviewsByVotes[j].UpVotes)
			}
			return vi > vj
		})

		b.WriteString("### Top Upvoted Community Reviews\n\n")
		count := 5
		if len(reviewsByVotes) < count {
			count = len(reviewsByVotes)
		}
		for i := 0; i < count; i++ {
			rev := reviewsByVotes[i]
			rating := 0
			if rev.Rating != nil {
				rating = *rev.Rating
			}
			up := 0
			if rev.UpVotes != nil {
				up = int(*rev.UpVotes)
			}
			b.WriteString(fmt.Sprintf("#### [%d Tacos] %s (👍 %d upvotes)\n", rating, rev.Title, up))
			b.WriteString(fmt.Sprintf("- **Reviewer**: `%s` (Deals purchased: %s, Joined: %s)\n",
				rev.User.Username, flexIntString(rev.User.DealsPurchased), truncateStr(rev.User.DateJoined, 10)))
			b.WriteString(fmt.Sprintf("- **Date**: %s\n\n", truncateStr(rev.Created, 10)))
			b.WriteString(fmt.Sprintf("%s\n\n", rev.Comment))
		}

		// Critical Reviews Breakdown (1-3 tacos)
		criticalReviews := []Review{}
		for _, r := range a.Reviews {
			if r.Rating != nil && *r.Rating <= 3 {
				criticalReviews = append(criticalReviews, r)
			}
		}
		if len(criticalReviews) > 0 {
			b.WriteString("### Critical Feedback & Friction Points (1-3 Tacos)\n\n")
			for _, rev := range criticalReviews {
				rating := 0
				if rev.Rating != nil {
					rating = *rev.Rating
				}
				up := 0
				if rev.UpVotes != nil {
					up = int(*rev.UpVotes)
				}
				b.WriteString(fmt.Sprintf("- **[%d Tacos] %s** (by `%s`, 👍 %d)\n", rating, rev.Title, rev.User.Username, up))
				b.WriteString(fmt.Sprintf("  - *Date*: %s\n", truncateStr(rev.Created, 10)))
				summary := oneLineStr(rev.Comment)
				if len(summary) > 200 {
					summary = summary[:200] + "..."
				}
				b.WriteString(fmt.Sprintf("  - *Feedback*: %s\n\n", summary))
			}
		}
	}

	return b.String()
}

func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	res := b.String()
	res = strings.ReplaceAll(res, "&nbsp;", " ")
	res = strings.ReplaceAll(res, "&amp;", "&")
	res = strings.ReplaceAll(res, "&lt;", "<")
	res = strings.ReplaceAll(res, "&gt;", ">")
	res = strings.ReplaceAll(res, "&quot;", "\"")
	return strings.TrimSpace(res)
}

func truncateStr(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func oneLineStr(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.Join(strings.Fields(s), " ")
}

func flexIntString(val *flexInt) string {
	if val == nil {
		return "N/A"
	}
	return fmt.Sprintf("%d", *val)
}
