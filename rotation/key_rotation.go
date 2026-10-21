package rotation

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type CreatedKey struct {
	ID  string `json:"key_id"`
	Key string `json:"key_secret"`
}

type RotatedKey struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

type LogEntry struct {
	Deployment string `json:"deployment"`
}

type LogSearch struct {
	Entries []LogEntry `json:"entries"`
}

type RotationInput struct {
	TenantID      string
	Name          string
	Scopes        []string
	GraceHours    int
	IdempotencyID string
}

type RotationResult struct {
	KeyID              string   `json:"key_id"`
	OldKey             string   `json:"old_key"`
	NewKey             string   `json:"new_key"`
	OldKeyDeployments  []string `json:"old_key_deployments"`
	SafeToRevokeOldKey bool     `json:"safe_to_revoke_old_key"`
}

func (c *Client) RotateTenantKey(ctx context.Context, in RotationInput) (RotationResult, error) {
	var created CreatedKey
	createBody := map[string]any{
		"project_id":      in.TenantID,
		"name":            in.Name,
		"scopes":          in.Scopes,
		"idempotency_key": in.IdempotencyID + "-create",
	}
	if err := c.do(ctx, http.MethodPost, "/v1/account/keys/create", nil, createBody, &created); err != nil {
		return RotationResult{}, fmt.Errorf("create temporary tenant key: %w", err)
	}
	if created.ID == "" || created.Key == "" {
		return RotationResult{}, fmt.Errorf("create temporary tenant key: response omitted credentials")
	}

	var rotated RotatedKey
	rotateBody := map[string]any{
		"grace_hours":     in.GraceHours,
		"idempotency_key": in.IdempotencyID + "-rotate",
	}
	rotatePath := "/v1/account/keys/rotate/" + url.PathEscape(created.ID)
	if err := c.do(ctx, http.MethodPost, rotatePath, nil, rotateBody, &rotated); err != nil {
		return RotationResult{}, fmt.Errorf("rotate temporary tenant key: %w", err)
	}

	query := url.Values{"q": {"key_id:" + created.ID}, "limit": {"100"}}
	var logs LogSearch
	if err := c.do(ctx, http.MethodGet, "/v1/logs/search", query, nil, &logs); err != nil {
		return RotationResult{}, fmt.Errorf("search old-key deployment logs: %w", err)
	}

	deployments := uniqueDeployments(logs.Entries)
	return RotationResult{
		KeyID:              created.ID,
		OldKey:             created.Key,
		NewKey:             rotated.Key,
		OldKeyDeployments:  deployments,
		SafeToRevokeOldKey: len(deployments) == 0,
	}, nil
}

func (c *Client) RevokeTemporaryKey(ctx context.Context, id string) error {
	path := "/v1/account/keys/revoke/" + url.PathEscape(id)
	return c.do(ctx, http.MethodDelete, path, nil, nil, nil)
}

func uniqueDeployments(entries []LogEntry) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Deployment)
		if name != "" && !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	return result
}
