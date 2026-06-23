package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const downloadHost = "https://jmng2-dl.jioaicloud.com"

// Download fetches a file from the server and writes it to destPath.
func (c *Client) Download(objectKey, destPath string) error {
	ts := time.Now().UnixMilli()
	url := fmt.Sprintf("%s/download/files/%s?vdc=jmng2&apiKey=%s&devicetype=web&ts=%d", 
		downloadHost, objectKey, c.creds.ApiKey, ts)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	// Apply standard headers.
	c.setCommonHeaders(req.Header)

	// The download endpoint specifically relies on the 'u' cookie.
	cookieVal := fmt.Sprintf("%s:Basic %s:%s:%s", c.creds.UserID, c.creds.AuthCode, c.creds.AppSecret, c.creds.DeviceKey)
	req.AddCookie(&http.Cookie{Name: "u", Value: cookieVal})

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download failed: status %d: %s", resp.StatusCode, string(body))
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
