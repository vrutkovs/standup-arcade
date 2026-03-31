package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
)

// oauthCodeCh receives the authorization code delivered to /callback by
// Google when a non-local OAUTH_CALLBACK_URL is used.
var oauthCodeCh = make(chan string, 1)

//go:embed static
var staticFiles embed.FS

type attendeesResponse struct {
	Attendees []string `json:"attendees,omitempty"`
	Error     string   `json:"error,omitempty"`
}

func startWebServer(addr string) {
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to prepare static files: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("/api/attendees", handleAttendees)
	// /callback receives the OAuth authorization code from Google when
	// OAUTH_CALLBACK_URL points at this server (non-local deployments).
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "no authorization code in request", http.StatusBadRequest)
			return
		}
		select {
		case oauthCodeCh <- code:
			fmt.Fprintln(w, "Authorization successful! You can close this tab.")
		default:
			http.Error(w, "unexpected callback — code already received", http.StatusBadRequest)
		}
	})

	log.Printf("Standup server running at http://%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func handleAttendees(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	meeting := r.URL.Query().Get("meeting")
	if meeting == "" {
		json.NewEncoder(w).Encode(attendeesResponse{Error: "meeting parameter required"})
		return
	}

	logErr := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		log.Printf("meeting=%s error: %s", meeting, msg)
		json.NewEncoder(w).Encode(attendeesResponse{Error: msg})
	}

	code, err := extractMeetingCode(meeting)
	if err != nil {
		logErr("%v", err)
		return
	}

	ctx := context.Background()
	client, err := getOAuthClient(ctx)
	if err != nil {
		logErr("auth failed: %v", err)
		return
	}

	mc := &MeetClient{HTTPClient: client}

	space, err := mc.GetSpace(ctx, code)
	if err != nil {
		logErr("could not resolve meeting: %v", err)
		return
	}

	records, err := mc.ListConferenceRecords(ctx, space.Name)
	if err != nil {
		logErr("could not list records: %v", err)
		return
	}

	// Only consider the active (ongoing) conference record — EndTime is empty
	// while the call is in progress. Using past records would surface stale
	// participants from previous sessions.
	var activeRecord *ConferenceRecord
	for i := range records {
		if records[i].EndTime == "" {
			activeRecord = &records[i]
			break
		}
	}
	if activeRecord == nil {
		logErr("meeting has not started yet")
		return
	}

	seen := make(map[string]bool)
	var attendees []string
	participants, err := mc.ListParticipants(ctx, activeRecord.Name)
	if err != nil {
		logErr("could not list participants: %v", err)
		return
	}
	for _, p := range participants {
		name := p.DisplayName()
		if name != "" && name != "(unknown)" && !seen[name] {
			seen[name] = true
			attendees = append(attendees, name)
		}
	}

	log.Printf("meeting=%s attendees=%d %v", meeting, len(attendees), attendees)
	json.NewEncoder(w).Encode(attendeesResponse{Attendees: attendees})
}
