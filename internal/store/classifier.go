package store

import "strings"

// Classifier decides which changed files count as authored work. It is built
// from user configuration, so no project's layout is baked into the binary.
type Classifier struct {
	paths    []string
	suffixes []string
	bigFile  int
}

// NewClassifier builds a Classifier. A bigFile of zero disables the size rule.
func NewClassifier(paths, suffixes []string, bigFile int) *Classifier {
	return &Classifier{paths: paths, suffixes: suffixes, bigFile: bigFile}
}

// IsGenerated reports whether a file's diff measures a generator rather than a
// person. The size rule catches data blobs no path list thinks to name: a
// single 100k-line JSON fixture can otherwise outweigh a month of real work.
func (c *Classifier) IsGenerated(path string, additions, deletions int) bool {
	if c.bigFile > 0 && additions+deletions > c.bigFile {
		return true
	}
	for _, s := range c.suffixes {
		if strings.HasSuffix(path, s) {
			return true
		}
	}
	rooted := "/" + path
	for _, p := range c.paths {
		if strings.Contains(rooted, "/"+p) {
			return true
		}
	}
	return false
}
