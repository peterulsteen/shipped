package store

import "testing"

func TestIsGenerated(t *testing.T) {
	c := NewClassifier(
		[]string{"pnpm-lock.yaml", "dist/", "vendor/", "testdata/"},
		[]string{".min.js", ".snap"},
		2000,
	)

	cases := []struct {
		name          string
		path          string
		add, del      int
		wantGenerated bool
	}{
		{"root lockfile", "pnpm-lock.yaml", 400, 380, true},
		{"nested lockfile", "apps/web/pnpm-lock.yaml", 12, 3, true},
		{"build output", "apps/web/dist/main.js", 900, 0, true},
		{"suffix rule", "static/app.min.js", 50, 2, true},
		{"vendored", "vendor/github.com/x/y.go", 100, 0, true},
		{"ordinary source", "internal/store/classifier.go", 80, 4, false},
		{"small fixture stays authored", "internal/testdata_helper.go", 30, 1, false},

		// The near-miss: a substring check without boundaries would call this
		// generated because it starts with "dist".
		{"distribution is not dist/", "src/distribution/rates.ts", 120, 8, false},
		{"vendorable is not vendor/", "src/vendorable/x.go", 40, 0, false},

		// The size rule catches blobs no path list names.
		{"huge json blob", "apps/api/__t/replay-capture.json", 100041, 0, true},
		{"just under the cap", "internal/big_but_written.go", 1500, 499, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := c.IsGenerated(tc.path, tc.add, tc.del); got != tc.wantGenerated {
				t.Errorf("IsGenerated(%q, %d, %d) = %v, want %v",
					tc.path, tc.add, tc.del, got, tc.wantGenerated)
			}
		})
	}
}

func TestBigFileRuleDisabled(t *testing.T) {
	c := NewClassifier(nil, nil, 0)
	if c.IsGenerated("huge.json", 500000, 0) {
		t.Error("a big_file_lines of 0 must disable the size rule, not apply it at zero")
	}
}
