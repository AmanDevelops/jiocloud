package api

import (
	"github.com/AmanDevelops/jiocloud/internal/config"
)

// Login parses the cookie string, scrapes the default public API key from the web
// app, and returns the resolved credentials ready to be saved. The app secret is
// taken solely from the user's cookie.
func Login(cookie string) (*config.Credentials, error) {
	creds, err := config.ParseCookie(cookie)
	if err != nil {
		return nil, err
	}

	apiKey, _, err := ScrapeDefaults(scrapeClient())
	if err != nil {
		return nil, err
	}
	creds.ApiKey = apiKey

	return creds, nil
}
