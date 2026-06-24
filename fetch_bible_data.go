package bibletext

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	maxChapterProbe        = 200
	maxRetries             = 6
	maxRateLimitRecoveries = 8
	minRequestDelay        = 900 * time.Millisecond
	bookDelay              = 1500 * time.Millisecond
	maxRetryDelay          = 30 * time.Second
)

var errChapterNotFound = errors.New("chapter not found")
var retrySleep = time.Sleep

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type apiHTTPError struct {
	StatusCode int
	RetryAfter time.Duration
	Details    string
}

func (e *apiHTTPError) Error() string {
	if e.Details == "" {
		return fmt.Sprintf("API status %d", e.StatusCode)
	}
	return fmt.Sprintf("API status %d: %s", e.StatusCode, e.Details)
}

// FetchBibleFromAPI fetches the complete World English Bible from a public API
// This uses the free bible-api service which has the complete WEB translation
// Source: https://bible-api.com/
func FetchBibleFromAPI() (*BibleData, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	return fetchBibleFromAPIWithClient(NewBibleData().Books, client, time.Sleep)
}

func fetchBibleFromAPIWithClient(books []string, client httpClient, sleepFn func(time.Duration)) (*BibleData, error) {
	fmt.Println("Fetching complete World English Bible from bible-api.com...")
	fmt.Println("(This happens once on first run; it fetches chapter-by-chapter and can take several minutes. The result is cached.)")

	bd := &BibleData{
		Verses: make(map[string]map[int][]Verse),
		Books:  append([]string(nil), books...),
	}

	// Create a map to hold chapter data temporarily
	// Structure: book -> (chapter -> verses)
	bibleVerses := make(map[string]map[int][]Verse)

	// Initialize maps for each book
	for _, book := range bd.Books {
		bibleVerses[book] = make(map[int][]Verse)
	}

	// Loop through each book and fetch all chapters
	totalBooksLoaded := 0
	failedBooks := []string{}
	failedBookSet := make(map[string]bool)
	addFailedBook := func(book string) {
		if failedBookSet[book] {
			return
		}
		failedBookSet[book] = true
		failedBooks = append(failedBooks, book)
	}

	for bookIdx, book := range bd.Books {
		fmt.Printf("\n[%d/%d] 📖 Loading %s", bookIdx+1, len(bd.Books), book)
		if loadProgressFn != nil {
			loadProgressFn(book, bookIdx+1, len(bd.Books), 0) // book starting
		}

		// Try to fetch each chapter (most books have 1-150 chapters)
		chaptersLoaded := 0
		versesInBook := 0
		sampleVerse := ""
		consecutiveChapterFailures := 0
		abortedBook := false
		rateLimitRecoveries := 0

		for chapter := 1; chapter <= maxChapterProbe; {
			// Build the API URL with proper URL encoding
			// Format: https://bible-api.com/Genesis+1 or https://bible-api.com/1+Corinthians+1
			// We use %20 for spaces to ensure proper URL encoding
			encodedBook := strings.ReplaceAll(book, " ", "%20")
			url := fmt.Sprintf("https://bible-api.com/%s+%d", encodedBook, chapter)

			// Fetch the chapter with retry
			response, err := fetchWithRetry(client, url, maxRetries)
			if err != nil {
				if errors.Is(err, errChapterNotFound) {
					// Natural end-of-book condition.
					if chapter == 1 {
						fmt.Printf("\n      ⚠️  Chapter 1 returned not found. Skipping %s.\n", book)
					}
					break
				}

				var statusErr *apiHTTPError
				if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusTooManyRequests {
					rateLimitRecoveries++
					if rateLimitRecoveries <= maxRateLimitRecoveries {
						recoveryDelay := statusErr.RetryAfter
						if recoveryDelay <= 0 {
							recoveryDelay = 5 * time.Second
						}
						if recoveryDelay > maxRetryDelay {
							recoveryDelay = maxRetryDelay
						}
						fmt.Printf("\n      ⏳ Rate limited on %s %d. Recovery %d/%d, waiting %s.\n",
							book, chapter, rateLimitRecoveries, maxRateLimitRecoveries, recoveryDelay)
						sleepFn(recoveryDelay)
						continue
					}
				}

				consecutiveChapterFailures++
				fmt.Printf("\n      ⚠️  Failed to fetch %s %d (%d/%d): %v\n",
					book, chapter, consecutiveChapterFailures, maxRetries, err)

				if consecutiveChapterFailures >= maxRetries {
					fmt.Printf("      ❌ Stopping %s after %d consecutive chapter failures.\n", book, consecutiveChapterFailures)
					abortedBook = true
					break
				}

				// Retry same chapter in outer loop after a short cooldown.
				sleepFn(time.Duration(consecutiveChapterFailures) * time.Second)
				continue
			}

			// Parse the response
			chapterVerses, err := decodeChapterResponse(book, chapter, response)
			if err != nil {
				consecutiveChapterFailures++
				fmt.Printf("\n      ⚠️  Failed to parse %s %d (%d/%d): %v\n",
					book, chapter, consecutiveChapterFailures, maxRetries, err)

				if consecutiveChapterFailures >= maxRetries {
					fmt.Printf("      ❌ Stopping %s after repeated parse failures.\n", book)
					abortedBook = true
					break
				}

				sleepFn(time.Duration(consecutiveChapterFailures) * time.Second)
				continue
			}

			// Only add verses if we actually got some back
			if len(chapterVerses) == 0 {
				fmt.Printf("\n      ⚠️  Empty response for %s %d. Treating this as end of book.\n", book, chapter)
				break
			}

			consecutiveChapterFailures = 0
			rateLimitRecoveries = 0
			chaptersLoaded++
			if loadProgressFn != nil {
				loadProgressFn(book, bookIdx+1, len(bd.Books), chapter) // a chapter landed
			}

			// Add verses to our data structure
			for _, verse := range chapterVerses {
				bibleVerses[book][chapter] = append(bibleVerses[book][chapter], verse)
				versesInBook++

				// Keep first verse as sample to display
				if chapter == 1 && verse.Verse == 1 && sampleVerse == "" {
					sampleVerse = verse.Text
				}
			}

			// Show progress every 5 chapters
			if chapter%5 == 0 {
				fmt.Printf(".")
			}

			// Be nice to the API server - increased delay to respect rate limits
			sleepFn(minRequestDelay)
			chapter++
		}

		// Show completion message for this book with sample text
		if chaptersLoaded > 0 {
			totalBooksLoaded++
			fmt.Printf(" ✓\n")
			fmt.Printf("   → %d chapters, %d verses\n", chaptersLoaded, versesInBook)
			if sampleVerse != "" {
				// Show first ~100 chars of first verse
				if len(sampleVerse) > 100 {
					fmt.Printf("   → Sample: \"%s...\"\n", sampleVerse[:100])
				} else {
					fmt.Printf("   → Sample: \"%s\"\n", sampleVerse)
				}
			}
			if abortedBook {
				fmt.Printf("   → ⚠️  Book stopped early due repeated API failures.\n")
			}
		} else {
			if abortedBook {
				fmt.Printf(" (failed to load chapter data)\n")
			} else {
				fmt.Printf(" (no chapters found)\n")
			}
			addFailedBook(book)
		}

		if abortedBook {
			addFailedBook(book)
		}

		// Add delay between books to further respect rate limiting
		sleepFn(bookDelay)
	}

	fmt.Printf("\n\n✅ Successfully loaded %d books!\n", totalBooksLoaded)
	if len(failedBooks) > 0 {
		fmt.Printf("⚠️  Books with incomplete or failed loads: %s\n", strings.Join(failedBooks, ", "))
	}

	// Transfer the fetched data into the BibleData structure
	bd.Verses = bibleVerses

	if totalBooksLoaded == 0 {
		return nil, fmt.Errorf("could not load any books from API")
	}
	if len(failedBooks) > 0 {
		return bd, fmt.Errorf("loaded %d/%d books; failed or incomplete books: %s",
			totalBooksLoaded, len(bd.Books), strings.Join(failedBooks, ", "))
	}

	return bd, nil
}

// fetchWithRetry fetches a URL with automatic retry on failure
// Retries up to maxRetries times with exponential backoff
func fetchWithRetry(client httpClient, url string, maxRetries int) ([]byte, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create HTTP request
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			lastErr = err
			continue
		}

		// Set user agent to be polite to the server
		req.Header.Set("User-Agent", "BibleText-Go/1.0 (Learning Project)")

		// Make the request
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d/%d request failed: %w", attempt, maxRetries, err)
			// Wait a bit before retrying
			retrySleep(time.Duration(attempt) * time.Second)
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("attempt %d/%d body read failed: %w", attempt, maxRetries, readErr)
			retrySleep(time.Duration(attempt) * time.Second)
			continue
		}

		// Check for successful status code
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("%w (%s)", errChapterNotFound, url)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			delay := backoffDelay(attempt)
			if retryAfterDelay, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
				delay = retryAfterDelay
			}
			if delay > maxRetryDelay {
				delay = maxRetryDelay
			}

			responsePreview := strings.TrimSpace(string(body))
			if len(responsePreview) > 160 {
				responsePreview = responsePreview[:160] + "..."
			}
			if responsePreview != "" {
				lastErr = fmt.Errorf("attempt %d/%d status %d (retry after %s): %s",
					attempt, maxRetries, resp.StatusCode, delay, responsePreview)
			} else {
				lastErr = fmt.Errorf("attempt %d/%d status %d (retry after %s)",
					attempt, maxRetries, resp.StatusCode, delay)
			}
			lastErr = fmt.Errorf("%w", &apiHTTPError{
				StatusCode: resp.StatusCode,
				RetryAfter: delay,
				Details:    lastErr.Error(),
			})
			retrySleep(delay)
			continue
		}

		return body, nil
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries, lastErr)
}

func decodeChapterResponse(book string, chapter int, response []byte) ([]Verse, error) {
	var result struct {
		Reference string `json:"reference"`
		Text      string `json:"text"`
		Verses    []struct {
			Verse int    `json:"verse"`
			Text  string `json:"text"`
		} `json:"verses"`
	}

	if err := json.Unmarshal(response, &result); err != nil {
		preview := strings.TrimSpace(string(response))
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		if preview == "" {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
		return nil, fmt.Errorf("invalid JSON: %w (response preview: %s)", err, preview)
	}

	verses := make([]Verse, 0, len(result.Verses))
	for _, v := range result.Verses {
		verses = append(verses, Verse{
			BookName: book,
			Book:     book,
			Chapter:  chapter,
			Verse:    v.Verse,
			Text:     v.Text,
		})
	}

	return verses, nil
}

func backoffDelay(attempt int) time.Duration {
	delay := time.Duration(attempt) * 2 * time.Second
	if delay < time.Second {
		return time.Second
	}
	return delay
}

func parseRetryAfter(value string) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	if seconds, err := time.ParseDuration(value + "s"); err == nil {
		if seconds < 0 {
			return 0, false
		}
		return seconds, true
	}

	retryAt, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	delay := time.Until(retryAt)
	if delay < 0 {
		return 0, true
	}
	return delay, true
}
