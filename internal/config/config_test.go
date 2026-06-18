package config

import "testing"

func TestParseCookie(t *testing.T) {
	c, err := ParseCookie("user-id-1234:Basic QVVUSF9DT0RFX0VYQU1QTEU=:APP_SECRET_EXAMPLE:device-key-5678")
	if err != nil {
		t.Fatal(err)
	}
	if c.UserID != "user-id-1234" {
		t.Errorf("UserID = %q", c.UserID)
	}
	if c.AuthCode != "QVVUSF9DT0RFX0VYQU1QTEU=" {
		t.Errorf("AuthCode = %q", c.AuthCode)
	}
	if c.AppSecret != "APP_SECRET_EXAMPLE" {
		t.Errorf("AppSecret = %q", c.AppSecret)
	}
	if c.DeviceKey != "device-key-5678" {
		t.Errorf("DeviceKey = %q", c.DeviceKey)
	}
}

func TestParseCookieBad(t *testing.T) {
	for _, s := range []string{
		"only:two:parts",
		"user:notbasic token:secret:device",
		"user:Basic :secret:device",
	} {
		if _, err := ParseCookie(s); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
}
