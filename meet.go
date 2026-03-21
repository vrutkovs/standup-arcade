package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const meetAPIBase = "https://meet.googleapis.com/v2"

// MeetClient wraps an authenticated HTTP client for the Google Meet REST API.
type MeetClient struct {
	HTTPClient *http.Client
}

// ---------- Spaces ----------

// Space represents a Google Meet space (a persistent meeting room).
// Docs: https://developers.google.com/meet/api/reference/rest/v2/spaces
type Space struct {
	Name        string `json:"name"`        // e.g. "spaces/MI3F6XXXXXXXXXX"
	MeetingCode string `json:"meetingCode"` // e.g. "abc-defg-hij"
	MeetingURI  string `json:"meetingUri"`
}

// GetSpace resolves a meeting code (e.g. "abc-defg-hij") to its Space resource.
func (c *MeetClient) GetSpace(ctx context.Context, meetingCode string) (*Space, error) {
	endpoint := fmt.Sprintf("%s/spaces/%s", meetAPIBase, meetingCode)
	body, err := c.doGet(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("spaces.get: %w", err)
	}
	var space Space
	if err := json.Unmarshal(body, &space); err != nil {
		return nil, fmt.Errorf("spaces.get decode: %w", err)
	}
	return &space, nil
}

// ---------- Conference Records ----------

// ConferenceRecord represents a single meeting session.
// Docs: https://developers.google.com/meet/api/reference/rest/v2/conferenceRecords
type ConferenceRecord struct {
	Name      string `json:"name"`      // e.g. "conferenceRecords/abc123"
	StartTime string `json:"startTime"` // RFC3339
	EndTime   string `json:"endTime"`   // RFC3339 (empty if ongoing)
	Space     string `json:"space"`     // e.g. "spaces/abc-defg-hij"
}

type conferenceRecordsResponse struct {
	ConferenceRecords []ConferenceRecord `json:"conferenceRecords"`
	NextPageToken     string             `json:"nextPageToken"`
}

// ListConferenceRecords returns all conference records whose space matches
// the given spaceName (e.g. "spaces/abc-defg-hij").
func (c *MeetClient) ListConferenceRecords(ctx context.Context, spaceName string) ([]ConferenceRecord, error) {
	var all []ConferenceRecord
	pageToken := ""

	for {
		params := url.Values{}
		params.Set("filter", fmt.Sprintf(`space.name = "%s"`, spaceName))
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}

		endpoint := fmt.Sprintf("%s/conferenceRecords?%s", meetAPIBase, params.Encode())

		body, err := c.doGet(ctx, endpoint)
		if err != nil {
			return nil, fmt.Errorf("conferenceRecords.list: %w", err)
		}

		var resp conferenceRecordsResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("conferenceRecords.list decode: %w", err)
		}

		all = append(all, resp.ConferenceRecords...)

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return all, nil
}

// ---------- Participants ----------

// SignedinUser is a logged-in Google user.
type SignedinUser struct {
	User        string `json:"user"`        // resource name
	DisplayName string `json:"displayName"` // e.g. "Jane Doe"
}

// AnonymousUser joined without signing in.
type AnonymousUser struct {
	DisplayName string `json:"displayName"`
}

// PhoneUser joined by phone.
type PhoneUser struct {
	DisplayName string `json:"displayName"`
}

// Participant represents someone who joined a meeting.
// Docs: https://developers.google.com/meet/api/reference/rest/v2/conferenceRecords.participants
type Participant struct {
	Name              string         `json:"name"` // resource name
	SignedinUser      *SignedinUser  `json:"signedinUser,omitempty"`
	AnonymousUser     *AnonymousUser `json:"anonymousUser,omitempty"`
	PhoneUser         *PhoneUser     `json:"phoneUser,omitempty"`
	EarliestStartTime string         `json:"earliestStartTime"`
	LatestEndTime     string         `json:"latestEndTime"`
}

// DisplayName returns a human-readable name regardless of user type.
func (p Participant) DisplayName() string {
	switch {
	case p.SignedinUser != nil:
		return p.SignedinUser.DisplayName
	case p.AnonymousUser != nil:
		return p.AnonymousUser.DisplayName + " (anonymous)"
	case p.PhoneUser != nil:
		return p.PhoneUser.DisplayName + " (phone)"
	default:
		return "(unknown)"
	}
}

type participantsResponse struct {
	Participants  []Participant `json:"participants"`
	NextPageToken string        `json:"nextPageToken"`
}

// ListParticipants returns all participants for a conference record.
// recordName is e.g. "conferenceRecords/abc123".
func (c *MeetClient) ListParticipants(ctx context.Context, recordName string) ([]Participant, error) {
	var all []Participant
	pageToken := ""

	for {
		endpoint := fmt.Sprintf("%s/%s/participants", meetAPIBase, recordName)
		if pageToken != "" {
			endpoint += "?pageToken=" + url.QueryEscape(pageToken)
		}

		body, err := c.doGet(ctx, endpoint)
		if err != nil {
			return nil, fmt.Errorf("participants.list: %w", err)
		}

		var resp participantsResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("participants.list decode: %w", err)
		}

		all = append(all, resp.Participants...)

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return all, nil
}

// ---------- helpers ----------

func (c *MeetClient) doGet(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
