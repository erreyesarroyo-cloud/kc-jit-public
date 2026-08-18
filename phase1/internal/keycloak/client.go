package keycloak

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jadeuc/keycloak-pim/internal/config"
)

type Client struct {
	cfg        config.Config
	httpClient *http.Client

	mu          sync.Mutex
	cachedToken string
	tokenExpiry time.Time

	groupIDs map[string]string
}

func New(cfg config.Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		groupIDs:   map[string]string{},
	}
}

func (c *Client) InGroup(username, groupName string) (bool, error) {
	uid, err := c.userID(username)
	if err != nil {
		return false, err
	}
	groups, err := c.userGroups(uid)
	if err != nil {
		return false, err
	}
	for _, g := range groups {
		if g == groupName {
			return true, nil
		}
	}
	return false, nil
}

func (c *Client) AddToGroup(username, groupName string) error {
	uid, err := c.userID(username)
	if err != nil {
		return err
	}
	gid, err := c.groupID(groupName)
	if err != nil {
		return err
	}
	return c.do(http.MethodPut, fmt.Sprintf("/admin/realms/%s/users/%s/groups/%s", c.cfg.Realm, uid, gid), nil, nil)
}

func (c *Client) RemoveFromGroup(username, groupName string) error {
	uid, err := c.userID(username)
	if err != nil {
		return err
	}
	gid, err := c.groupID(groupName)
	if err != nil {
		return err
	}
	return c.do(http.MethodDelete, fmt.Sprintf("/admin/realms/%s/users/%s/groups/%s", c.cfg.Realm, uid, gid), nil, nil)
}

func (c *Client) userID(username string) (string, error) {
	var users []struct {
		ID string `json:"id"`
	}
	q := url.Values{"username": {username}, "exact": {"true"}}
	if err := c.do(http.MethodGet, fmt.Sprintf("/admin/realms/%s/users?%s", c.cfg.Realm, q.Encode()), nil, &users); err != nil {
		return "", err
	}
	if len(users) == 0 || users[0].ID == "" {
		return "", fmt.Errorf("user %q not found", username)
	}
	return users[0].ID, nil
}

func (c *Client) userGroups(uid string) ([]string, error) {
	var groups []struct {
		Name string `json:"name"`
	}
	if err := c.do(http.MethodGet, fmt.Sprintf("/admin/realms/%s/users/%s/groups", c.cfg.Realm, uid), nil, &groups); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, g.Name)
	}
	return out, nil
}

func (c *Client) groupID(name string) (string, error) {
	c.mu.Lock()
	if id, ok := c.groupIDs[name]; ok {
		c.mu.Unlock()
		return id, nil
	}
	c.mu.Unlock()

	var groups []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	q := url.Values{"search": {name}, "exact": {"true"}}
	if err := c.do(http.MethodGet, fmt.Sprintf("/admin/realms/%s/groups?%s", c.cfg.Realm, q.Encode()), nil, &groups); err != nil {
		return "", err
	}
	for _, g := range groups {
		if g.Name == name && g.ID != "" {
			c.mu.Lock()
			c.groupIDs[name] = g.ID
			c.mu.Unlock()
			return g.ID, nil
		}
	}
	return "", fmt.Errorf("group %q not found", name)
}

func (c *Client) accessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cachedToken != "" && time.Now().Before(c.tokenExpiry.Add(-30*time.Second)) {
		return c.cachedToken, nil
	}
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
	}
	resp, err := c.httpClient.PostForm(
		fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", strings.TrimRight(c.cfg.BaseURL, "/"), c.cfg.Realm),
		form,
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("token: %s: %s", resp.Status, string(body))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", err
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("empty access_token")
	}
	c.cachedToken = tok.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	return c.cachedToken, nil
}

func (c *Client) do(method, path string, body io.Reader, out any) error {
	tok, err := c.accessToken()
	if err != nil {
		return err
	}
	req, err := http.NewRequest(method, strings.TrimRight(c.cfg.BaseURL, "/")+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, string(raw))
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}
