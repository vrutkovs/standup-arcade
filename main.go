package main

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

func main() {
	addr := "localhost:8080"
	if len(os.Args) >= 2 {
		addr = os.Args[1]
	}
	startWebServer(addr)
}

// extractMeetingCode parses a Google Meet URL and returns the meeting code.
// Accepts formats:
//
//	https://meet.google.com/abc-defg-hij
//	meet.google.com/abc-defg-hij
//	abc-defg-hij
func extractMeetingCode(raw string) (string, error) {
	// If it looks like a bare meeting code already.
	codeRe := regexp.MustCompile(`^[a-z]{3}-[a-z]{4}-[a-z]{3}$`)
	if codeRe.MatchString(raw) {
		return raw, nil
	}

	// Normalise to a full URL so url.Parse works.
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("cannot parse URL: %w", err)
	}

	if u.Host != "meet.google.com" {
		return "", fmt.Errorf("expected host meet.google.com, got %q", u.Host)
	}

	code := strings.TrimPrefix(u.Path, "/")
	code = strings.Split(code, "?")[0] // strip query params if any
	if !codeRe.MatchString(code) {
		return "", fmt.Errorf("meeting code %q does not match expected pattern xxx-xxxx-xxx", code)
	}

	return code, nil
}
