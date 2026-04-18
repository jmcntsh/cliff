package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jmcntsh/cliff/internal/catalog"
)

const pinnedSHA = "0e791621ea617b95cecb3dd43ce261ca38bffce9"

type entry struct {
	Category    string
	Name        string
	Owner       string
	Repo        string
	URL         string
	Description string
}

var (
	categoryRE = regexp.MustCompile(`<summary><h2>([^<]+)</h2></summary>`)
	entryRE    = regexp.MustCompile(`^\s*-\s*\[([^\]]+)\]\(https://github\.com/([^/)]+)/([^/)]+?)/?\)\s*(.*)$`)
)

func parseReadme(md []byte) []entry {
	var out []entry
	var cur string
	for _, line := range strings.Split(string(md), "\n") {
		if m := categoryRE.FindStringSubmatch(line); m != nil {
			cur = strings.TrimSpace(m[1])
			continue
		}
		if cur == "" {
			continue
		}
		if m := entryRE.FindStringSubmatch(line); m != nil {
			out = append(out, entry{
				Category:    cur,
				Name:        m[1],
				Owner:       m[2],
				Repo:        m[3],
				URL:         fmt.Sprintf("https://github.com/%s/%s", m[2], m[3]),
				Description: strings.TrimSpace(m[4]),
			})
		}
	}
	return out
}

type ghRepo struct {
	StargazersCount int    `json:"stargazers_count"`
	Language        string `json:"language"`
	License         struct {
		SPDXID string `json:"spdx_id"`
	} `json:"license"`
	Description string `json:"description"`
}

func enrich(e entry, client *http.Client, token string) (catalog.App, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", e.Owner, e.Repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return catalog.App{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
		switch resp.StatusCode {
		case 200:
			var data ghRepo
			dec := json.NewDecoder(resp.Body)
			if err := dec.Decode(&data); err != nil {
				resp.Body.Close()
				return catalog.App{}, err
			}
			resp.Body.Close()
			return catalog.App{
				Name:        e.Name,
				Repo:        e.Owner + "/" + e.Repo,
				Description: e.Description,
				Category:    e.Category,
				Language:    data.Language,
				License:     data.License.SPDXID,
				Stars:       data.StargazersCount,
				Homepage:    e.URL,
			}, nil
		case 403, 429:
			reset := resp.Header.Get("X-RateLimit-Reset")
			resp.Body.Close()
			if reset != "" {
				var ts int64
				fmt.Sscanf(reset, "%d", &ts)
				wait := time.Until(time.Unix(ts, 0)) + 5*time.Second
				if wait > 0 {
					log.Printf("  rate limited; sleeping %v", wait.Truncate(time.Second))
					time.Sleep(wait)
					continue
				}
			}
			return catalog.App{}, fmt.Errorf("rate limited without reset header")
		case 404:
			resp.Body.Close()
			return catalog.App{}, fmt.Errorf("404 (repo moved or deleted)")
		default:
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return catalog.App{}, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
		}
	}
	return catalog.App{}, fmt.Errorf("max retries exceeded")
}

func main() {
	src := flag.String("src", "scripts/scrape/testdata/awesome-tuis.md", "path to awesome-tuis README snapshot")
	out := flag.String("out", "internal/catalog/data/catalog.json", "output path for catalog.json")
	flag.Parse()

	md, err := os.ReadFile(*src)
	if err != nil {
		log.Fatalf("read %s: %v", *src, err)
	}

	entries := parseReadme(md)
	log.Printf("parsed %d entries from %s", len(entries), *src)

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Println("warning: GITHUB_TOKEN not set; anonymous rate limit is 60/hr (will hit a wall)")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var apps []catalog.App
	catCounts := map[string]int{}
	skipped := 0
	for i, e := range entries {
		log.Printf("[%d/%d] %s/%s", i+1, len(entries), e.Owner, e.Repo)
		app, err := enrich(e, client, token)
		if err != nil {
			log.Printf("  skip: %v", err)
			skipped++
			continue
		}
		apps = append(apps, app)
		catCounts[app.Category]++
	}

	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Category != apps[j].Category {
			return apps[i].Category < apps[j].Category
		}
		if apps[i].Stars != apps[j].Stars {
			return apps[i].Stars > apps[j].Stars
		}
		return apps[i].Name < apps[j].Name
	})

	var cats []catalog.Category
	for name, count := range catCounts {
		cats = append(cats, catalog.Category{Name: name, Count: count})
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i].Name < cats[j].Name })

	c := catalog.Catalog{
		SchemaVersion: 1,
		GeneratedAt:   time.Now().UTC(),
		SourceCommit:  "awesome-tuis@" + pinnedSHA,
		Apps:          apps,
		Categories:    cats,
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(*out, append(data, '\n'), 0644); err != nil {
		log.Fatalf("write %s: %v", *out, err)
	}
	log.Printf("wrote %d apps across %d categories to %s (skipped %d)", len(apps), len(cats), *out, skipped)
}
