// Package gh talks to the GitHub GraphQL API through the user's gh CLI, so the
// tool never handles a token of its own.
package gh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Client runs GraphQL queries via `gh api graphql`.
type Client struct{ bin string }

// New returns a Client, verifying the gh CLI is present and authenticated.
func New() (*Client, error) {
	bin, err := exec.LookPath("gh")
	if err != nil {
		return nil, fmt.Errorf("the GitHub CLI (gh) is required: https://cli.github.com")
	}
	c := &Client{bin: bin}
	if _, err := c.Viewer(); err != nil {
		return nil, fmt.Errorf("gh is installed but not authenticated -- run `gh auth login`: %w", err)
	}
	return c, nil
}

type graphQLError struct {
	Message string `json:"message"`
}

// Query runs one GraphQL query and unmarshals data into out.
func (c *Client) Query(query string, vars map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return err
	}
	cmd := exec.Command(c.bin, "api", "graphql", "--input", "-")
	cmd.Stdin = bytes.NewReader(body)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh api graphql: %s", strings.TrimSpace(stderr.String()))
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []graphQLError  `json:"errors"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		return fmt.Errorf("decoding GitHub response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		msgs := make([]string, len(envelope.Errors))
		for i, e := range envelope.Errors {
			msgs[i] = e.Message
		}
		return fmt.Errorf("GitHub: %s", strings.Join(msgs, "; "))
	}
	return json.Unmarshal(envelope.Data, out)
}

// Viewer returns the authenticated user's login.
func (c *Client) Viewer() (string, error) {
	var out struct {
		Viewer struct {
			Login string `json:"login"`
		} `json:"viewer"`
	}
	if err := c.Query(`{ viewer { login } }`, nil, &out); err != nil {
		return "", err
	}
	return out.Viewer.Login, nil
}
