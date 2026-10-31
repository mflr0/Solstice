package devseed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
)

// client is a signed-in session on the in-process server.
type client struct {
	base string
	http *http.Client
}

func login(ctx context.Context, base, username, password string) (*client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	c := &client{base: base, http: &http.Client{Jar: jar}}
	if err := c.call("POST", "/api/login", map[string]string{"username": username, "password": password}, nil); err != nil {
		return nil, fmt.Errorf("signing in as %s: %w", username, err)
	}
	return c, nil
}

// call sends a JSON request and decodes the answer into out, when given.
func (c *client) call(method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.base+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Solstice", "1")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %d %s", method, path, res.StatusCode, bytes.TrimSpace(raw))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}
