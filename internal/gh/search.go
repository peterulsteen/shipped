package gh

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// searchCap is GitHub's hard limit on results per search query. A window that
// would exceed it is split rather than silently truncated.
const searchCap = 1000

// PullRequest is one PR as this tool needs it.
type PullRequest struct {
	Number    int        `json:"number"`
	Repo      string     `json:"repo"`
	Author    string     `json:"author"`
	CreatedAt time.Time  `json:"created_at"`
	MergedAt  *time.Time `json:"merged_at,omitempty"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	Additions int        `json:"additions"`
	Deletions int        `json:"deletions"`
	Files     int        `json:"changed_files"`
	SrcAdd    int        `json:"src_additions"`
	SrcDel    int        `json:"src_deletions"`
	FilesCap  bool       `json:"files_capped"`
	// FilesSeenLines is the diff GitHub listed file by file; whatever the PR
	// changed beyond that sits in files past the 100-file cap.
	FilesSeenLines int  `json:"files_seen_lines"`
	ReviewsCap     bool `json:"reviews_capped"`
	// Reviews are the reviews submitted on this PR, by anyone, oldest first.
	// Both halves of the report read from these: on a PR the user authored they
	// are the reviews RECEIVED; on a PR they reviewed, theirs is the one GIVEN.
	Reviews []Review `json:"reviews,omitempty"`
}

// Review is one submitted review.
type Review struct {
	At time.Time `json:"at"`
	By string    `json:"by"`
}

// FirstReviewBy returns the earliest review not authored by login -- the first
// time somebody else engaged with the change.
func (p PullRequest) FirstReviewBy(login string) (Review, bool) {
	for _, r := range p.Reviews {
		if r.By != login {
			return r, true
		}
	}
	return Review{}, false
}

// MyReview returns the measured user's earliest review on this PR.
func (p PullRequest) MyReview(login string) (Review, bool) {
	for _, r := range p.Reviews {
		if r.By == login {
			return r, true
		}
	}
	return Review{}, false
}

// ID is the stable key for deduplication across windows and runs.
func (p PullRequest) ID() string { return fmt.Sprintf("%s#%d", p.Repo, p.Number) }

type prNode struct {
	Number    int        `json:"number"`
	CreatedAt time.Time  `json:"createdAt"`
	MergedAt  *time.Time `json:"mergedAt"`
	ClosedAt  *time.Time `json:"closedAt"`
	Additions int        `json:"additions"`
	Deletions int        `json:"deletions"`
	Files     int        `json:"changedFiles"`
	Author    *struct {
		Login string `json:"login"`
	} `json:"author"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	FileList struct {
		TotalCount int `json:"totalCount"`
		Nodes      []struct {
			Path      string `json:"path"`
			Additions int    `json:"additions"`
			Deletions int    `json:"deletions"`
		} `json:"nodes"`
	} `json:"files"`
	Reviews struct {
		TotalCount int `json:"totalCount"`
		Nodes      []struct {
			SubmittedAt *time.Time `json:"submittedAt"`
			Author      *struct {
				Login string `json:"login"`
			} `json:"author"`
		} `json:"nodes"`
	} `json:"reviews"`
}

type searchResult struct {
	Search struct {
		IssueCount int `json:"issueCount"`
		PageInfo   struct {
			HasNextPage bool   `json:"hasNextPage"`
			EndCursor   string `json:"endCursor"`
		} `json:"pageInfo"`
		Nodes []prNode `json:"nodes"`
	} `json:"search"`
}

const searchQuery = `
query($q: String!, $after: String) {
  search(query: $q, type: ISSUE, first: 100, after: $after) {
    issueCount
    pageInfo { hasNextPage endCursor }
    nodes { ... on PullRequest {
      number createdAt mergedAt closedAt additions deletions changedFiles
      author { login }
      repository { nameWithOwner }
      files(first: 100) { totalCount nodes { path additions deletions } }
      reviews(first: 100) { totalCount nodes { submittedAt author { login } } }
    } }
  }
}`

// Classifier decides whether a changed file counts as authored work.
type Classifier interface {
	IsGenerated(path string, additions, deletions int) bool
}

// Scope narrows a search to orgs or repos. Both empty means all of GitHub.
type Scope struct {
	Orgs  []string
	Repos []string
}

func (s Scope) qualifier() string {
	var parts []string
	for _, o := range s.Orgs {
		parts = append(parts, "org:"+o)
	}
	for _, r := range s.Repos {
		parts = append(parts, "repo:"+r)
	}
	return strings.Join(parts, " ")
}

// Role selects which side of the work to fetch.
type Role string

const (
	// Authored finds PRs the user opened.
	Authored Role = "author"
	// Reviewed finds PRs the user reviewed -- the half most tools ignore.
	Reviewed Role = "reviewed-by"
)

// Search fetches every PR matching role and scope updated within [start, end].
// Windows that would exceed GitHub's 1000-result cap are split, never truncated.
func (c *Client) Search(role Role, login string, scope Scope, start, end time.Time, cls Classifier, progress func(string)) ([]PullRequest, error) {
	q := strings.TrimSpace(fmt.Sprintf("%s:%s is:pr %s updated:%s..%s",
		role, login, scope.qualifier(),
		start.Format(time.DateOnly), end.Format(time.DateOnly)))

	var first searchResult
	if err := c.Query(searchQuery, map[string]any{"q": q, "after": nil}, &first); err != nil {
		return nil, err
	}

	if first.Search.IssueCount > searchCap {
		if !end.After(start) {
			return nil, fmt.Errorf("%s: %d results in a single day exceeds GitHub's %d-result cap",
				start.Format(time.DateOnly), first.Search.IssueCount, searchCap)
		}
		mid := start.Add(end.Sub(start) / 2)
		if progress != nil {
			progress(fmt.Sprintf("  %s..%s: %d results, splitting",
				start.Format(time.DateOnly), end.Format(time.DateOnly), first.Search.IssueCount))
		}
		left, err := c.Search(role, login, scope, start, mid, cls, progress)
		if err != nil {
			return nil, err
		}
		right, err := c.Search(role, login, scope, mid.AddDate(0, 0, 1), end, cls, progress)
		if err != nil {
			return nil, err
		}
		return append(left, right...), nil
	}

	out := convert(first.Search.Nodes, login, cls)
	page := first
	for page.Search.PageInfo.HasNextPage {
		var next searchResult
		if err := c.Query(searchQuery, map[string]any{"q": q, "after": page.Search.PageInfo.EndCursor}, &next); err != nil {
			return nil, err
		}
		out = append(out, convert(next.Search.Nodes, login, cls)...)
		page = next
	}
	if progress != nil && len(out) > 0 {
		progress(fmt.Sprintf("  %s..%s: %d",
			start.Format(time.DateOnly), end.Format(time.DateOnly), len(out)))
	}
	return out, nil
}

func convert(nodes []prNode, login string, cls Classifier) []PullRequest {
	out := make([]PullRequest, 0, len(nodes))
	for _, n := range nodes {
		if n.Repository.NameWithOwner == "" {
			continue // a non-PR node in the search result
		}
		pr := PullRequest{
			Number:     n.Number,
			Repo:       n.Repository.NameWithOwner,
			CreatedAt:  n.CreatedAt,
			MergedAt:   n.MergedAt,
			ClosedAt:   n.ClosedAt,
			Additions:  n.Additions,
			Deletions:  n.Deletions,
			Files:      n.Files,
			FilesCap:   n.FileList.TotalCount > len(n.FileList.Nodes),
			ReviewsCap: n.Reviews.TotalCount > len(n.Reviews.Nodes),
		}
		if n.Author != nil {
			pr.Author = n.Author.Login
		}
		for _, f := range n.FileList.Nodes {
			pr.FilesSeenLines += f.Additions + f.Deletions
			if cls.IsGenerated(f.Path, f.Additions, f.Deletions) {
				continue
			}
			pr.SrcAdd += f.Additions
			pr.SrcDel += f.Deletions
		}
		for _, r := range n.Reviews.Nodes {
			if r.Author == nil || r.SubmittedAt == nil {
				continue
			}
			pr.Reviews = append(pr.Reviews, Review{At: *r.SubmittedAt, By: r.Author.Login})
		}
		sort.Slice(pr.Reviews, func(i, j int) bool {
			return pr.Reviews[i].At.Before(pr.Reviews[j].At)
		})
		out = append(out, pr)
	}
	return out
}
