package handlers

import (
	"context"
	"encoding/xml"
	"log"
	"net/http"
	"strings"
	"time"

	"olv-parish/internal/models"

	"gorm.io/gorm"
)

// defaultCourierFeedURL is the RSS feed the home page shows headlines from.
// Override with the COURIER_FEED_URL env var if the Catholic Courier changes
// its address.
const defaultCourierFeedURL = "https://catholiccourier.com/feed/"

// CourierFetcher refreshes the CourierHeadline table from the Diocese of
// Rochester's Catholic Courier RSS feed. It runs as a background goroutine
// kicked off from main; the home page just reads the rows the fetcher wrote.
type CourierFetcher struct {
	db       *gorm.DB
	url      string
	interval time.Duration
	client   *http.Client
}

func NewCourierFetcher(db *gorm.DB, url string) *CourierFetcher {
	if url == "" {
		url = defaultCourierFeedURL
	}
	return &CourierFetcher{
		db:       db,
		url:      url,
		interval: time.Hour,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// Start runs an initial fetch and then refreshes on an interval ticker. It
// never returns; call it from a goroutine. Cancellation is via ctx.
func (f *CourierFetcher) Start(ctx context.Context) {
	if f.db == nil {
		log.Println("CourierFetcher: no database; feed will not be fetched")
		return
	}
	if err := f.fetchOnce(ctx); err != nil {
		log.Printf("CourierFetcher: initial fetch failed: %v", err)
	}
	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := f.fetchOnce(ctx); err != nil {
				log.Printf("CourierFetcher: fetch failed: %v", err)
			}
		}
	}
}

type rssFeed struct {
	XMLName xml.Name  `xml:"rss"`
	Channel rssChan   `xml:"channel"`
}

type rssChan struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title    string   `xml:"title"`
	Link     string   `xml:"link"`
	PubDate  string   `xml:"pubDate"`
	Creator  string   `xml:"creator"`
	Categories []string `xml:"category"`
}

func (f *CourierFetcher) fetchOnce(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "OLV-Parish-Site/1.0 (+https://www.olv14843.org/)")
	req.Header.Set("Accept", "application/rss+xml, application/xml;q=0.9, */*;q=0.8")

	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var feed rssFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return err
	}

	now := time.Now()
	for _, item := range feed.Channel.Items {
		if item.Link == "" || item.Title == "" {
			continue
		}
		pub := parseRSSDate(item.PubDate)
		headline := models.CourierHeadline{
			URL:         strings.TrimSpace(item.Link),
			Title:       strings.TrimSpace(item.Title),
			Byline:      strings.TrimSpace(item.Creator),
			Category:    joinCategories(item.Categories),
			PublishedAt: pub,
			FetchedAt:   now,
		}
		var existing models.CourierHeadline
		err := f.db.Where("url = ?", headline.URL).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if err := f.db.Create(&headline).Error; err != nil {
				log.Printf("CourierFetcher: insert %q: %v", headline.URL, err)
			}
			continue
		}
		if err != nil {
			log.Printf("CourierFetcher: lookup %q: %v", headline.URL, err)
			continue
		}
		// Row exists — update the mutable fields (title or byline can change
		// if the publisher edits a post).
		existing.Title = headline.Title
		existing.Byline = headline.Byline
		existing.Category = headline.Category
		existing.PublishedAt = pub
		existing.FetchedAt = now
		if err := f.db.Save(&existing).Error; err != nil {
			log.Printf("CourierFetcher: update %q: %v", headline.URL, err)
		}
	}
	return nil
}

// LoadCourierHeadlines returns the newest `limit` cached headlines.
func LoadCourierHeadlines(db *gorm.DB, limit int) []models.CourierHeadline {
	if db == nil {
		return nil
	}
	if limit <= 0 {
		limit = 5
	}
	var rows []models.CourierHeadline
	if err := db.Order("published_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil
	}
	return rows
}

func parseRSSDate(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	for _, layout := range []string{
		time.RFC1123,
		time.RFC1123Z,
		time.RFC822,
		time.RFC822Z,
		"Mon, 2 Jan 2006 15:04:05 MST",
		"Mon, 2 Jan 2006 15:04:05 -0700",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Now()
}

func joinCategories(cats []string) string {
	cleaned := make([]string, 0, len(cats))
	for _, c := range cats {
		c = strings.TrimSpace(c)
		if c != "" {
			cleaned = append(cleaned, c)
		}
	}
	return strings.Join(cleaned, ", ")
}
