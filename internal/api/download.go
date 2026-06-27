package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const downloadHost = "https://jaws-dl.jioaicloud.com"

// Download fetches a file from the server and writes it to destPath.
func (c *Client) Download(objectKey, destPath string, progress func(downloaded, total int64)) error {
	ts := time.Now().UnixMilli()
	url := fmt.Sprintf("%s/download/files/%s?vdc=jaws&apiKey=%s&devicetype=web&ts=%d",
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

	var src io.Reader = resp.Body
	if progress != nil {
		src = &progressReader{
			r:        resp.Body,
			total:    resp.ContentLength,
			progress: progress,
		}
	}
	_, err = io.Copy(out, src)
	return err
}

type progressReader struct {
	r        io.Reader
	downloaded int64
	total    int64
	progress func(int64, int64)
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.r.Read(p)
	if n > 0 {
		pr.downloaded += int64(n)
		pr.progress(pr.downloaded, pr.total)
	}
	return
}
