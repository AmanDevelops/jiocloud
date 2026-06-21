package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Object types returned by the NMS metadata API.
const (
	TypeFolder = "FR"
	TypeFile   = "FE"
)

// listLimit is the page size used when listing a folder. The web client caps at
// 1000; we warn if a listing returns exactly this many entries (possible truncation).
const listLimit = 1000

// Object is a folder (FR) or file (FE) entry within a folder listing.
type Object struct {
	ObjectKey        string `json:"objectKey"`
	ObjectType       string `json:"objectType"`
	ObjectName       string `json:"objectName"`
	Hash             string `json:"hash"` // md5, present for files
	SizeInBytes      int64  `json:"sizeInBytes"`
	ParentObjectKey  string `json:"parentObjectKey"`
	ParentObjectType string `json:"parentObjectType"`
}

type listResp struct {
	Objects []Object `json:"objects"`
}

// ListFolder returns the immediate children of the given folder.
func (c *Client) ListFolder(folderKey string) ([]Object, error) {
	q := url.Values{}
	q.Set("limit", fmt.Sprintf("%d", listLimit))
	q.Set("folderKey", folderKey)
	q.Set("sort", "-fileCreatedDate")

	req, err := http.NewRequest(http.MethodGet, apiHost+"/nms/metadata?"+q.Encode(), nil)
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
		return nil, fmt.Errorf("list folder %s failed: status %d: %s", folderKey, status, body)
	}

	var lr listResp
	if err := json.Unmarshal(body, &lr); err != nil {
		return nil, fmt.Errorf("parsing folder listing: %w", err)
	}
	if len(lr.Objects) >= listLimit {
		fmt.Printf("warning: folder %s has >=%d entries; listing may be truncated\n", folderKey, listLimit)
	}
	return lr.Objects, nil
}

// CreateFolder creates a new folder under parentKey and returns its object key.
func (c *Client) CreateFolder(name, parentKey string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"objectName":      name,
		"parentObjectKey": parentKey,
		"sourceName":      "DRIVE",
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, apiHost+"/nms/folders", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	c.setCommonHeaders(req.Header)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	respBody, status, err := c.do(req)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return "", fmt.Errorf("create folder %q failed: status %d: %s", name, status, respBody)
	}

	var created Object
	if err := json.Unmarshal(respBody, &created); err != nil {
		return "", fmt.Errorf("parsing create folder response: %w", err)
	}
	if created.ObjectKey == "" {
		return "", fmt.Errorf("create folder %q: empty objectKey in response", name)
	}
	return created.ObjectKey, nil
}
