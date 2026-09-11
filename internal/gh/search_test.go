package gh

import (
	"encoding/json"
	"testing"
)

type noneGenerated struct{}

func (noneGenerated) IsGenerated(string, int, int) bool { return false }

// convert flags a PR whose files or reviews GitHub truncated, and records how
// much diff it did list, so the page can bound what is missing.
func TestConvertRecordsTruncation(t *testing.T) {
	raw := `[
	 {"number": 1, "createdAt": "2026-09-01T10:00:00Z", "additions": 900, "deletions": 100,
	  "repository": {"nameWithOwner": "o/r"},
	  "files": {"totalCount": 140, "nodes": [{"path": "a.go", "additions": 5, "deletions": 1}]},
	  "reviews": {"totalCount": 130, "nodes": [{"submittedAt": "2026-09-01T11:00:00Z", "author": {"login": "alice"}}]}},
	 {"number": 2, "createdAt": "2026-09-01T10:00:00Z", "additions": 5, "deletions": 1,
	  "repository": {"nameWithOwner": "o/r"},
	  "files": {"totalCount": 1, "nodes": [{"path": "a.go", "additions": 5, "deletions": 1}]},
	  "reviews": {"totalCount": 1, "nodes": [{"submittedAt": "2026-09-01T11:00:00Z", "author": {"login": "alice"}}]}}
	]`
	var nodes []prNode
	if err := json.Unmarshal([]byte(raw), &nodes); err != nil {
		t.Fatal(err)
	}
	prs := convert(nodes, "me", noneGenerated{})
	if !prs[0].FilesCap || !prs[0].ReviewsCap || prs[0].FilesSeenLines != 6 {
		t.Errorf("truncated PR: FilesCap=%v ReviewsCap=%v FilesSeenLines=%d, want true true 6",
			prs[0].FilesCap, prs[0].ReviewsCap, prs[0].FilesSeenLines)
	}
	if prs[1].FilesCap || prs[1].ReviewsCap {
		t.Errorf("complete PR flagged as truncated: FilesCap=%v ReviewsCap=%v", prs[1].FilesCap, prs[1].ReviewsCap)
	}
}
