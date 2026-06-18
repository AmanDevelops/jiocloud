package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// UserInfo is the subset of GET /security/users we care about.
type UserInfo struct {
	UserID        string `json:"userId"`
	FirstName     string `json:"firstName"`
	RootFolderKey string `json:"rootFolderKey"`
	Quota         struct {
		AllocatedSpace int64 `json:"allocatedSpace"`
		UsedSpace      int64 `json:"usedSpace"`
	} `json:"quota"`
}

// UserInfo fetches the authenticated user's profile, including the root folder key.
func (c *Client) UserInfo() (*UserInfo, error) {
	req, err := http.NewRequest(http.MethodGet, securityHost+"/security/users", nil)
	if err != nil {
		return nil, err
	}
	c.setCommonHeaders(req.Header)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	body, status, err := c.do(req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("user info failed: status %d: %s", status, body)
	}

	var u UserInfo
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, fmt.Errorf("parsing user info: %w", err)
	}
	if u.RootFolderKey == "" {
		return nil, fmt.Errorf("user info response missing rootFolderKey")
	}
	return &u, nil
}
