package main

import (
	"fmt"
	"golang.org/x/net/html"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"unicode"
)

type scrapResult struct {
	url   string
	words map[string]int
}

var allowedTags = map[string]bool{
	"h1":         true,
	"h2":         true,
	"h3":         true,
	"h4":         true,
	"h5":         true,
	"h6":         true,
	"p":          true,
	"li":         true,
	"dt":         true,
	"dd":         true,
	"a":          true,
	"strong":     true,
	"em":         true,
	"b":          true,
	"i":          true,
	"blockquote": true,
	"figcaption": true,
	"td":         true,
	"th":         true,
	"dfn":        true,
	"address":    true,
	"time":       true,
	"cite":       true,
	"abbr":       true,
	"details":    true,
	"summary":    true,
	"figure":     true,
	"span":       true,
}

var parallelLimit = runtime.NumCPU()

func main() {
	// input could be a list from incoming http request or from args as binary would be run as a CLI
	// we should validate for valid urls and remove duplicates
	// (not covered as im writing this code in a speedrun mode :D)
	input := []string{"https://www.github.com", "https://www.gitlab.com"}
	// assume it is a real cache like Redis or memcached and could be populated before.
	// now we would check if there are already results for given urls
	cache := NewInMemoryCache()
	wg := sync.WaitGroup{}
	wg.Add(len(input))

	sem := make(chan struct{}, parallelLimit-1)
	results := make(chan scrapResult, len(input))

	for i := range input {
		go func(j int) {
			defer wg.Done()

			sem <- struct{}{}
			body, err := fetch(input[j])
			if err != nil {
				slog.Default().Error(err.Error())
				<-sem
				return
			}
			defer body.Close()

			// we could go for more granular semaphores here,
			// instead of acquiring one at the beginning of this code block
			// so with (parallelLimit(4 cpu)  - 1) there could be 3 goroutines running at the same time on
			// CPU bound tasks, and 1 goroutine running on IO bound tasks
			// I am not considering cases about runtime parking goroutines on cpu bound task and so on,
			// so take this idea with grain of salt as I am wondering, but have no time for benchmarking and investigating.

			// sem <- struct{}{}
			res := countWords(body)
			// we are sure it will not block due to buffered channel for len(input) elements
			// so in the optimistic scenario, we could have all the results in the channel without blocking
			results <- scrapResult{url: input[j], words: res}
			<-sem
		}(i)
	}
	wg.Wait()
	// producers are done, so we can close the channel, normally the producer should do that,
	// so we don't risk panic on send but in current implementation we are sure that after wg.Done there are no more
	// sends to the channel, so we can close it here, which will automatically drain channel and exit for range loop later.
	close(results)
	// with the assumption we don't fetch the same url twice and we check for given url before fetching,
	// we can safely set the results to the cache
	for r := range results {
		cache.Set(r.url, r.words)
	}
	// channel handles the synchronization of the goroutines results, so map itself does not need to be synchronized.
	// we could go as well for a sync.Map and rely on it to handle the synchronization instead of using channels.
	fmt.Println(cache)
}

func fetch(url string) (io.ReadCloser, error) {
	r, err := http.Get(url)
	return r.Body, err
}

func countWords(r io.Reader) map[string]int {
	tokenizer := html.NewTokenizer(r)
	wordCounts := make(map[string]int)

	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			return wordCounts
		case html.StartTagToken, html.EndTagToken:
			tagName, _ := tokenizer.TagName()
			if allowedTags[string(tagName)] {
				tokenType = tokenizer.Next()
				if tokenType == html.TextToken {
					text := string(tokenizer.Text())
					words := strings.Fields(text)
					for _, word := range words {
						word = strings.ToLower(word)
						word = trimNonAlphanumeric(word)
						if word != "" && isAlphabetical(word) {
							wordCounts[word]++
						}
					}
				}
			}
		}
	}
}

func trimNonAlphanumeric(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

func isAlphabetical(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}
