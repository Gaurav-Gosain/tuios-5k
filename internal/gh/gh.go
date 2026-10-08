// Package gh holds the star and contributor data of the tuios repo: the
// snapshot shape that ships in data/, and the GitHub REST calls that bring it
// up to date.
//
// The live path works without a token. GitHub allows 60 unauthenticated
// requests an hour, so it never pages through all the stargazers: it asks for
// the repo, the contributors, and only the stargazer pages past the end of the
// snapshot.
package gh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Repo is the repository this program celebrates.
const Repo = "Gaurav-Gosain/tuios"

const dayLayout = "2006-01-02"

// Contributor is one human who has commits in the repo. Logins are public
// credit, so they are shown. Stargazer logins are never stored.
type Contributor struct {
	Login   string `json:"login"`
	Commits int    `json:"commits"`
	// First is the day of the first commit, YYYY-MM-DD. Empty when unknown.
	First string `json:"first,omitempty"`
}

// Data is everything the app draws.
type Data struct {
	Repo    string    `json:"repo"`
	Taken   time.Time `json:"taken"`
	Created time.Time `json:"created"`
	Stars   int       `json:"stars"`
	Forks   int       `json:"forks"`
	// Start is the first day of Daily, YYYY-MM-DD.
	Start string `json:"start"`
	// Daily holds the number of stars given on each day from Start on.
	Daily        []int         `json:"daily"`
	Contributors []Contributor `json:"contributors"`
}

// Parse reads a snapshot.
func Parse(b []byte) (*Data, error) {
	var d Data
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("parse snapshot: %w", err)
	}
	if _, err := time.Parse(dayLayout, d.Start); err != nil {
		return nil, fmt.Errorf("parse snapshot start: %w", err)
	}
	return &d, nil
}

// Clone returns a deep copy.
func (d *Data) Clone() *Data {
	c := *d
	c.Daily = slices.Clone(d.Daily)
	c.Contributors = slices.Clone(d.Contributors)
	return &c
}

// StartDay is Start as a time at midnight UTC.
func (d *Data) StartDay() time.Time {
	t, _ := time.Parse(dayLayout, d.Start)
	return t
}

// Listed is the number of stargazers the daily history accounts for.
func (d *Data) Listed() int {
	n := 0
	for _, v := range d.Daily {
		n += v
	}
	return n
}

// Cumulative returns the running star total at the end of each day.
func (d *Data) Cumulative() []int {
	out := make([]int, len(d.Daily))
	run := 0
	for i, v := range d.Daily {
		run += v
		out[i] = run
	}
	return out
}

// AddStar counts one star on the day of t.
func (d *Data) AddStar(t time.Time) {
	start := d.StartDay()
	day := int(t.UTC().Sub(start).Hours() / 24)
	if day < 0 {
		day = 0
	}
	for len(d.Daily) <= day {
		d.Daily = append(d.Daily, 0)
	}
	d.Daily[day]++
}

// IsBot reports whether a contributor login belongs to an automation account.
func IsBot(login, kind string) bool {
	return kind == "Bot" || strings.HasSuffix(login, "[bot]")
}

// Client talks to the GitHub REST API.
type Client struct {
	HTTP *http.Client
	// Token is optional. The app sets it only from TUIOS_5K_TOKEN, for
	// people who hit the hourly limit of 60 requests.
	Token string
}

// ErrRateLimited reports that GitHub refused the request for the hour.
var ErrRateLimited = errors.New("github rate limit reached")

// ErrNeedsAuth reports that GitHub wants a token for this call. The
// stargazer list with dates is one: the count and the contributors are
// still live without it, and the history comes from the snapshot.
var ErrNeedsAuth = errors.New("github needs a token for this call")

func (c *Client) get(ctx context.Context, url, accept string, out any) (http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if accept == "" {
		accept = "application/vnd.github+json"
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "tuios-5k")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		if resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.StatusCode == http.StatusTooManyRequests {
			return resp.Header, ErrRateLimited
		}
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return resp.Header, ErrNeedsAuth
	}
	if resp.StatusCode != http.StatusOK {
		return resp.Header, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return resp.Header, json.NewDecoder(resp.Body).Decode(out)
}

const api = "https://api.github.com/repos/" + Repo

type repoInfo struct {
	Stars   int       `json:"stargazers_count"`
	Forks   int       `json:"forks_count"`
	Created time.Time `json:"created_at"`
}

type rawContributor struct {
	Login         string `json:"login"`
	Type          string `json:"type"`
	Contributions int    `json:"contributions"`
}

type rawStar struct {
	StarredAt time.Time `json:"starred_at"`
}

const starAccept = "application/vnd.github.star+json"

// Repo fetches the star and fork counts.
func (c *Client) repo(ctx context.Context) (repoInfo, error) {
	var r repoInfo
	_, err := c.get(ctx, api, "", &r)
	return r, err
}

// Contributors fetches the human contributors, most commits first.
func (c *Client) Contributors(ctx context.Context) ([]Contributor, error) {
	var out []Contributor
	for page := 1; page <= 5; page++ {
		var raw []rawContributor
		url := fmt.Sprintf("%s/contributors?per_page=100&page=%d", api, page)
		if _, err := c.get(ctx, url, "", &raw); err != nil {
			return nil, err
		}
		for _, r := range raw {
			if IsBot(r.Login, r.Type) {
				continue
			}
			out = append(out, Contributor{Login: r.Login, Commits: r.Contributions})
		}
		if len(raw) < 100 {
			break
		}
	}
	return out, nil
}

// StarPage fetches one page of 100 stargazers, oldest first. Only the times
// are kept.
func (c *Client) StarPage(ctx context.Context, page int) ([]time.Time, error) {
	var raw []rawStar
	url := fmt.Sprintf("%s/stargazers?per_page=100&page=%d", api, page)
	if _, err := c.get(ctx, url, starAccept, &raw); err != nil {
		return nil, err
	}
	out := make([]time.Time, len(raw))
	for i, r := range raw {
		out[i] = r.StarredAt
	}
	return out, nil
}

var lastPage = regexp.MustCompile(`[?&]page=(\d+)>; rel="last"`)

// FirstCommit finds the day of a login's oldest commit. It costs two requests.
func (c *Client) FirstCommit(ctx context.Context, login string) (string, error) {
	type commit struct {
		Commit struct {
			Author struct {
				Date time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}
	var list []commit
	url := fmt.Sprintf("%s/commits?author=%s&per_page=1", api, login)
	h, err := c.get(ctx, url, "", &list)
	if err != nil {
		return "", err
	}
	if m := lastPage.FindStringSubmatch(h.Get("Link")); m != nil {
		n, _ := strconv.Atoi(m[1])
		list = nil
		if _, err := c.get(ctx, fmt.Sprintf("%s&page=%d", url, n), "", &list); err != nil {
			return "", err
		}
	}
	if len(list) == 0 {
		return "", fmt.Errorf("no commits for %s", login)
	}
	return list[0].Commit.Author.Date.UTC().Format(dayLayout), nil
}

// Snapshot pages through every stargazer and every contributor. It needs a
// token in practice, because it makes about 50 + 2 per contributor requests.
func (c *Client) Snapshot(ctx context.Context) (*Data, error) {
	r, err := c.repo(ctx)
	if err != nil {
		return nil, err
	}
	d := &Data{
		Repo:    Repo,
		Taken:   time.Now().UTC().Truncate(time.Second),
		Created: r.Created.UTC(),
		Stars:   r.Stars,
		Forks:   r.Forks,
		Start:   r.Created.UTC().Format(dayLayout),
	}
	for page := 1; ; page++ {
		ts, err := c.StarPage(ctx, page)
		if err != nil {
			return nil, err
		}
		for _, t := range ts {
			d.AddStar(t)
		}
		fmt.Fprintf(os.Stderr, "stargazers page %d: %d\n", page, len(ts))
		if len(ts) < 100 {
			break
		}
	}
	people, err := c.Contributors(ctx)
	if err != nil {
		return nil, err
	}
	for i := range people {
		first, err := c.FirstCommit(ctx, people[i].Login)
		if err != nil {
			fmt.Fprintf(os.Stderr, "first commit of %s: %v\n", people[i].Login, err)
		}
		people[i].First = first
	}
	d.Contributors = people
	return d, nil
}

// Refresh brings a snapshot up to date with at most a handful of requests:
// the repo, the contributors, and the stargazer pages past the end of the
// snapshot. It returns the best data it has together with the first error, so
// a partial update is still used.
func (c *Client) Refresh(ctx context.Context, snap *Data) (*Data, error) {
	d := snap.Clone()
	r, err := c.repo(ctx)
	if err != nil {
		return d, err
	}
	d.Stars, d.Forks = r.Stars, r.Forks
	d.Taken = time.Now().UTC()

	if people, err := c.Contributors(ctx); err == nil {
		d.Contributors = mergeContributors(snap.Contributors, people)
	} else {
		return d, err
	}

	listed := d.Listed()
	if r.Stars <= listed {
		return d, nil
	}
	first := listed/100 + 1
	last := (r.Stars + 99) / 100
	if last-first > 4 {
		// Far behind the snapshot: take the newest pages only, and leave
		// the gap out of the history rather than spend the hourly budget.
		first = last - 4
		listed = (first - 1) * 100
	}
	for page := first; page <= last; page++ {
		ts, err := c.StarPage(ctx, page)
		if err != nil {
			return d, err
		}
		skip := 0
		if page == first {
			skip = listed - (page-1)*100
		}
		for i, t := range ts {
			if i >= skip {
				d.AddStar(t)
			}
		}
	}
	return d, nil
}

// mergeContributors keeps the order and first-commit days of the snapshot,
// takes the live commit counts, and appends people who are new since.
func mergeContributors(old, live []Contributor) []Contributor {
	byLogin := make(map[string]Contributor, len(live))
	for _, c := range live {
		byLogin[strings.ToLower(c.Login)] = c
	}
	var out []Contributor
	seen := map[string]bool{}
	for _, o := range old {
		k := strings.ToLower(o.Login)
		if l, ok := byLogin[k]; ok {
			o.Commits = l.Commits
		}
		seen[k] = true
		out = append(out, o)
	}
	today := time.Now().UTC().Format(dayLayout)
	for _, l := range live {
		if !seen[strings.ToLower(l.Login)] {
			l.First = today
			out = append(out, l)
		}
	}
	return out
}

// ErrNoUser reports that GitHub has no account with that login.
var ErrNoUser = errors.New("no such github user")

// FindStar looks for the day login starred the repo. It reads the user's
// stars newest first and gives up after five pages of 100, or at the day the
// repo was created. The first page comes alone, because most people find
// their star there; the other four come together. It reports false when the
// star is not there.
func (c *Client) FindStar(ctx context.Context, login string, since time.Time) (time.Time, bool, error) {
	type starred struct {
		StarredAt time.Time `json:"starred_at"`
		Repo      struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	}
	fetch := func(page int) ([]starred, http.Header, error) {
		var list []starred
		url := fmt.Sprintf("https://api.github.com/users/%s/starred?sort=created&direction=desc&per_page=100&page=%d", login, page)
		h, err := c.get(ctx, url, starAccept, &list)
		if err != nil {
			if strings.Contains(err.Error(), " 404 ") {
				return nil, nil, ErrNoUser
			}
			return nil, nil, err
		}
		return list, h, nil
	}
	// scan reports the star, whether to stop, in page order.
	scan := func(list []starred) (time.Time, bool, bool) {
		for _, s := range list {
			if strings.EqualFold(s.Repo.FullName, Repo) {
				return s.StarredAt, true, true
			}
			if s.StarredAt.Before(since) {
				return time.Time{}, false, true
			}
		}
		return time.Time{}, false, len(list) < 100
	}
	first, h, err := fetch(1)
	if err != nil {
		return time.Time{}, false, err
	}
	if at, ok, stop := scan(first); stop {
		return at, ok, nil
	}
	// The list is newest first, so every page down to the one that reaches
	// back past the repo's creation can hold the star. Someone who stars a
	// lot can be many pages deep: read them in small batches, in order, and
	// stop at the first page that answers.
	last := linkLast(h)
	limit := maxStarPages
	if c.Token != "" {
		limit = maxStarPagesToken
	}
	if last == 0 || last > limit {
		last = limit
	}
	const batch = 6
	for from := 2; from <= last; from += batch {
		n := min(batch, last-from+1)
		pages := make([][]starred, n)
		errs := make([]error, n)
		var wg sync.WaitGroup
		for k := range n {
			wg.Go(func() { pages[k], _, errs[k] = fetch(from + k) })
		}
		wg.Wait()
		for k := range n {
			if errs[k] != nil {
				return time.Time{}, false, errs[k]
			}
			if at, ok, stop := scan(pages[k]); stop {
				return at, ok, nil
			}
		}
	}
	if lp := linkLast(h); lp > limit {
		return time.Time{}, false, ErrTooManyStars
	}
	return time.Time{}, false, nil
}

// maxStarPages bounds how deep FindStar reads without a token, in pages of
// 100. GitHub allows 60 requests an hour without one.
const (
	maxStarPages      = 40
	maxStarPagesToken = 300
)

// ErrTooManyStars reports that the user has starred more repos since tuios
// began than FindStar reads without a token.
var ErrTooManyStars = errors.New("too many starred repos to read without a token")

// linkLast reads the page number of rel="last" from a Link header, or 0.
func linkLast(h http.Header) int {
	if m := lastPage.FindStringSubmatch(h.Get("Link")); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	return 0
}

// ValidLogin reports whether s can be a GitHub login: letters, digits and
// single hyphens, at most 39 characters.
func ValidLogin(s string) bool {
	return loginRe.MatchString(s) && len(s) <= 39
}

var loginRe = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9]|-[A-Za-z0-9])*$`)
