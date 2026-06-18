package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Credentials holds everything needed to authenticate against the JioAiCloud API.
//
// UserID, AuthCode and DeviceKey come from the per-user cookie string. ApiKey and
// AppSecret are scraped once from the web app's main.*.js bundle and reused.
type Credentials struct {
	UserID    string `json:"userId"`
	AuthCode  string `json:"authCode"`  // the Basic token, without the leading "Basic "
	AppSecret string `json:"appSecret"` // X-App-Secret
	DeviceKey string `json:"deviceKey"`
	ApiKey    string `json:"apiKey"` // X-Api-Key
}

// ParseCookie parses the login string of the form:
//
//	{{USER_ID}}:Basic {{AUTH_CODE}}:{{APP_SECRET}}:{{DEVICE_KEY}}
//
// The AuthCode is base64-ish and never contains ':' or spaces, so we split on the
// first ':', strip the "Basic " prefix, then split the remainder by ':'.
func ParseCookie(s string) (*Credentials, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) != 4 {
		return nil, fmt.Errorf("expected format USER_ID:Basic AUTH_CODE:APP_SECRET:DEVICE_KEY, got %d ':'-separated parts", len(parts))
	}

	userID := strings.TrimSpace(parts[0])
	authField := strings.TrimSpace(parts[1])
	appSecret := strings.TrimSpace(parts[2])
	deviceKey := strings.TrimSpace(parts[3])

	authCode := strings.TrimSpace(strings.TrimPrefix(authField, "Basic"))
	if authCode == authField {
		return nil, fmt.Errorf("second field must start with \"Basic \"")
	}

	if userID == "" || authCode == "" || appSecret == "" || deviceKey == "" {
		return nil, fmt.Errorf("one or more credential fields are empty")
	}

	return &Credentials{
		UserID:    userID,
		AuthCode:  authCode,
		AppSecret: appSecret,
		DeviceKey: deviceKey,
	}, nil
}

// Path returns the location of the credentials file under the user config dir.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "jiocloud", "credentials.json"), nil
}

// Save writes credentials to disk with owner-only permissions.
func Save(c *Credentials) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// Load reads credentials from disk.
func Load() (*Credentials, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not logged in: run `jiocloud login` first")
		}
		return nil, err
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
