package api

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
)

const baseSite = "https://www.jioaicloud.com/"

var (
	// matches the hashed main bundle reference, e.g. main.2e5550489ce17e33.js
	mainJSRe = regexp.MustCompile(`main\.[0-9a-f]+\.js`)
	apiKeyRe = regexp.MustCompile(`"X-Api-Key"\s*:\s*"([^"]+)"`)
	// X-App-Secret but NOT X-App-Secret-Jito (negative lookahead isn't supported
	// by Go's regexp, so we match the key boundary with a closing quote).
	appSecretRe = regexp.MustCompile(`"X-App-Secret"\s*:\s*"([^"]+)"`)
)

// ScrapeDefaults fetches the JioAiCloud web app, locates the main JS bundle and
// extracts the default X-Api-Key and X-App-Secret values from it.
func ScrapeDefaults(client *http.Client) (apiKey, appSecret string, err error) {
	html, err := fetch(client, baseSite)
	if err != nil {
		return "", "", fmt.Errorf("fetching landing page: %w", err)
	}

	mainJS := mainJSRe.FindString(string(html))
	if mainJS == "" {
		return "", "", fmt.Errorf("could not locate main.*.js reference in landing page")
	}

	js, err := fetch(client, baseSite+mainJS)
	if err != nil {
		return "", "", fmt.Errorf("fetching %s: %w", mainJS, err)
	}

	if m := apiKeyRe.FindSubmatch(js); m != nil {
		apiKey = string(m[1])
	}
	if m := appSecretRe.FindSubmatch(js); m != nil {
		appSecret = string(m[1])
	}

	if apiKey == "" || appSecret == "" {
		return "", "", fmt.Errorf("could not extract X-Api-Key/X-App-Secret from %s", mainJS)
	}
	return apiKey, appSecret, nil
}

func fetch(client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
