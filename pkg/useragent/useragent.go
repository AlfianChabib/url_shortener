package useragent

import (
	"strings"
)

// UserAgentInfo contains parsed user-agent telemetry.
type UserAgentInfo struct {
	DeviceType string `json:"device_type"` // "mobile", "desktop", "bot"
	Browser    string `json:"browser"`
	OS         string `json:"os"`
	IsBot      bool   `json:"is_bot"`
}

// Parse extracts device type, browser, OS, and bot detection from a User-Agent string.
func Parse(ua string) UserAgentInfo {
	if ua == "" {
		return UserAgentInfo{
			DeviceType: "desktop",
			Browser:    "Unknown",
			OS:         "Unknown",
			IsBot:      false,
		}
	}

	lowerUA := strings.ToLower(ua)

	// 1. Detect Bots / Crawlers / HTTP clients
	if isBotUA(lowerUA) {
		return UserAgentInfo{
			DeviceType: "bot",
			Browser:    detectBotName(lowerUA),
			OS:         "Bot",
			IsBot:      true,
		}
	}

	// 2. Detect OS
	os := detectOS(ua, lowerUA)

	// 3. Detect Browser
	browser := detectBrowser(ua, lowerUA)

	// 4. Detect Device Type (Mobile vs Desktop)
	deviceType := "desktop"
	if isMobileUA(lowerUA) {
		deviceType = "mobile"
	}

	return UserAgentInfo{
		DeviceType: deviceType,
		Browser:    browser,
		OS:         os,
		IsBot:      false,
	}
}

func isBotUA(lower string) bool {
	botKeywords := []string{
		"bot", "crawler", "spider", "slurp", "facebookexternalhit",
		"whatsapp", "telegrambot", "twitterbot", "slackbot", "discordbot",
		"curl", "wget", "python-requests", "go-http-client", "postman",
		"headlesschrome", "phantomjs", "axios", "httpclient",
	}

	for _, kw := range botKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func detectBotName(lower string) string {
	switch {
	case strings.Contains(lower, "googlebot"):
		return "Googlebot"
	case strings.Contains(lower, "bingbot"):
		return "Bingbot"
	case strings.Contains(lower, "duckduckbot"):
		return "DuckDuckBot"
	case strings.Contains(lower, "yandex"):
		return "YandexBot"
	case strings.Contains(lower, "facebook"):
		return "FacebookBot"
	case strings.Contains(lower, "twitter"):
		return "TwitterBot"
	case strings.Contains(lower, "curl"):
		return "cURL"
	case strings.Contains(lower, "postman"):
		return "Postman"
	case strings.Contains(lower, "python"):
		return "Python"
	case strings.Contains(lower, "go-http-client"):
		return "Go-HTTP-Client"
	default:
		return "Bot"
	}
}

func isMobileUA(lower string) bool {
	mobileKeywords := []string{
		"mobile", "android", "iphone", "ipod", "ipad", "windows phone",
		"blackberry", "bb10", "rim tablet", "opera mini", "iemobile",
	}

	for _, kw := range mobileKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func detectOS(raw, lower string) string {
	switch {
	case strings.Contains(lower, "iphone") || strings.Contains(lower, "ipad") || strings.Contains(lower, "ipod"):
		return "iOS"
	case strings.Contains(lower, "android"):
		return "Android"
	case strings.Contains(lower, "windows"):
		return "Windows"
	case strings.Contains(lower, "macintosh") || strings.Contains(lower, "mac os x"):
		return "MacOS"
	case strings.Contains(lower, "linux"):
		return "Linux"
	case strings.Contains(lower, "cros"):
		return "ChromeOS"
	default:
		return "Unknown"
	}
}

func detectBrowser(raw, lower string) string {
	switch {
	case strings.Contains(lower, "edg/") || strings.Contains(lower, "edge/"):
		return "Edge"
	case strings.Contains(lower, "opr/") || strings.Contains(lower, "opera"):
		return "Opera"
	case strings.Contains(lower, "firefox") || strings.Contains(lower, "fxios"):
		return "Firefox"
	case strings.Contains(lower, "chrome") || strings.Contains(lower, "crios"):
		return "Chrome"
	case strings.Contains(lower, "safari") && !strings.Contains(lower, "chrome"):
		return "Safari"
	default:
		return "Other"
	}
}
