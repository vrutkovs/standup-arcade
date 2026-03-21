package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <google-meet-url>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s https://meet.google.com/abc-defg-hij\n", os.Args[0])
		os.Exit(1)
	}

	meetURL := os.Args[1]

	meetingCode, err := extractMeetingCode(meetURL)
	if err != nil {
		log.Fatalf("Invalid Meet URL: %v", err)
	}
	fmt.Printf("Meeting code: %s\n\n", meetingCode)

	ctx := context.Background()

	client, err := getOAuthClient(ctx)
	if err != nil {
		log.Fatalf("Failed to get OAuth client: %v", err)
	}

	meetClient := &MeetClient{HTTPClient: client}

	// Resolve the meeting code to a Space resource to get the canonical space name.
	space, err := meetClient.GetSpace(ctx, meetingCode)
	if err != nil {
		log.Fatalf("Failed to resolve meeting space: %v", err)
	}
	fmt.Printf("Space: %s\n\n", space.Name)

	records, err := meetClient.ListConferenceRecords(ctx, space.Name)
	if err != nil {
		log.Fatalf("Failed to list conference records: %v", err)
	}

	if len(records) == 0 {
		fmt.Println("No conference records found for this meeting.")
		fmt.Println("Note: the meeting must have already started (or ended) to have records.")
		os.Exit(0)
	}

	// Process each conference record (a meeting URL can host multiple sessions).
	for i, record := range records {
		startTime := formatTime(record.StartTime)
		endTime := "ongoing"
		if record.EndTime != "" {
			endTime = formatTime(record.EndTime)
		}
		fmt.Printf("=== Session %d (%s → %s) ===\n", i+1, startTime, endTime)
		fmt.Printf("Conference Record: %s\n\n", record.Name)

		participants, err := meetClient.ListParticipants(ctx, record.Name)
		if err != nil {
			log.Printf("Failed to list participants for %s: %v", record.Name, err)
			continue
		}

		if len(participants) == 0 {
			fmt.Println("  No participants found.")
			continue
		}

		printParticipants(participants)
		fmt.Println()
	}
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

func formatTime(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func printParticipants(participants []Participant) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  #\tNAME\tEMAIL\tJOINED\tLEFT\tDURATION")
	fmt.Fprintln(w, "  -\t----\t-----\t------\t----\t--------")

	for i, p := range participants {
		name := p.DisplayName()
		email := p.Email()
		joined := formatTime(p.EarliestStartTime)
		left := "—"
		duration := "—"

		if p.LatestEndTime != "" {
			left = formatTime(p.LatestEndTime)
			start, err1 := time.Parse(time.RFC3339, p.EarliestStartTime)
			end, err2 := time.Parse(time.RFC3339, p.LatestEndTime)
			if err1 == nil && err2 == nil {
				d := end.Sub(start).Round(time.Second)
				duration = d.String()
			}
		}

		fmt.Fprintf(w, "  %d\t%s\t%s\t%s\t%s\t%s\n",
			i+1, name, email, joined, left, duration)
	}
	w.Flush()
}
