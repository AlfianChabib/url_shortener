package useragent_test

import (
	"testing"
	"url_shortener/pkg/useragent"

	"github.com/stretchr/testify/assert"
)

func TestParseUserAgent(t *testing.T) {
	tests := []struct {
		name       string
		ua         string
		deviceType string
		browser    string
		os         string
		isBot      bool
	}{
		{
			name:       "iPhone Safari",
			ua:         "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
			deviceType: "mobile",
			browser:    "Safari",
			os:         "iOS",
			isBot:      false,
		},
		{
			name:       "Android Chrome",
			ua:         "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
			deviceType: "mobile",
			browser:    "Chrome",
			os:         "Android",
			isBot:      false,
		},
		{
			name:       "Windows Chrome Desktop",
			ua:         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
			deviceType: "desktop",
			browser:    "Chrome",
			os:         "Windows",
			isBot:      false,
		},
		{
			name:       "Mac Firefox Desktop",
			ua:         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:123.0) Gecko/20100101 Firefox/123.0",
			deviceType: "desktop",
			browser:    "Firefox",
			os:         "MacOS",
			isBot:      false,
		},
		{
			name:       "Googlebot",
			ua:         "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			deviceType: "bot",
			browser:    "Googlebot",
			os:         "Bot",
			isBot:      true,
		},
		{
			name:       "cURL HTTP client",
			ua:         "curl/7.88.1",
			deviceType: "bot",
			browser:    "cURL",
			os:         "Bot",
			isBot:      true,
		},
		{
			name:       "Empty User-Agent",
			ua:         "",
			deviceType: "desktop",
			browser:    "Unknown",
			os:         "Unknown",
			isBot:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := useragent.Parse(tt.ua)
			assert.Equal(t, tt.deviceType, info.DeviceType)
			assert.Equal(t, tt.browser, info.Browser)
			assert.Equal(t, tt.os, info.OS)
			assert.Equal(t, tt.isBot, info.IsBot)
		})
	}
}
