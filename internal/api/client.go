package api

import (
	"net/http"
	"time"

	"github.com/AmanDevelops/jiocloud/internal/config"
)

const (
	userAgent     = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	clientDetails = "clientType:WEB; appVersion:86.0.1"
	origin        = "https://www.jioaicloud.com"
	referer       = "https://www.jioaicloud.com/"

	uploadHost   = "https://jmng2-upload.jioaicloud.com"
	apiHost      = "https://jmng2-api.jioaicloud.com" // nmsURL
	securityHost = "https://api.jioaicloud.com"       // securityURL (global DC)
)

// Client is an authenticated JioAiCloud API client.
type Client struct {
	http  *http.Client
	creds *config.Credentials
}

// New returns a client bound to the given credentials.
func New(creds *config.Credentials) *Client {
	return &Client{
		http:  &http.Client{Timeout: 0}, // no overall timeout: large uploads can be slow
		creds: creds,
	}
}

// setCommonHeaders applies the auth + client identification headers shared by
// every authenticated request. Content-Type is left to the caller.
func (c *Client) setCommonHeaders(h http.Header) {
	h.Set("Accept", "application/json; charset=UTF-8")
	h.Set("Authorization", "Basic "+c.creds.AuthCode)
	h.Set("Origin", origin)
	h.Set("Referer", referer)
	h.Set("User-Agent", userAgent)
	h.Set("X-Api-Key", c.creds.ApiKey)
	h.Set("X-App-Secret", c.creds.AppSecret)
	h.Set("X-Client-Details", clientDetails)
	h.Set("X-Device-Key", c.creds.DeviceKey)
	h.Set("X-Device-Type", "W")
	h.Set("X-User-Id", c.creds.UserID)
}

// scrapeClient is a plain client for the unauthenticated scrape step.
func scrapeClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}
