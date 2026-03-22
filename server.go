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

	code, err := extractMeetingCode(meeting)
	if err != nil {
		json.NewEncoder(w).Encode(attendeesResponse{Error: err.Error()})
		return
	}

	ctx := context.Background()
	client, err := getOAuthClient(ctx)
	if err != nil {
		json.NewEncoder(w).Encode(attendeesResponse{Error: "auth failed: " + err.Error()})
		return
	}

	mc := &MeetClient{HTTPClient: client}

	space, err := mc.GetSpace(ctx, code)
	if err != nil {
		json.NewEncoder(w).Encode(attendeesResponse{Error: "could not resolve meeting: " + err.Error()})
		return
	}

	records, err := mc.ListConferenceRecords(ctx, space.Name)
	if err != nil {
		json.NewEncoder(w).Encode(attendeesResponse{Error: "could not list records: " + err.Error()})
		return
	}

	seen := make(map[string]bool)
	var attendees []string
	for _, record := range records {
		participants, err := mc.ListParticipants(ctx, record.Name)
		if err != nil {
			continue
		}
		for _, p := range participants {
			name := p.DisplayName()
			if name != "" && name != "(unknown)" && !seen[name] {
				seen[name] = true
				attendees = append(attendees, name)
			}
		}
	}

	json.NewEncoder(w).Encode(attendeesResponse{Attendees: attendees})
}
