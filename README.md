# meet-attendees

A CLI tool that fetches the list of participants who actually joined a Google Meet meeting.

## How it works

1. Extracts the meeting code from a Google Meet URL
2. Uses the [Google Meet REST API v2](https://developers.google.com/meet/api) to look up conference records for that meeting space
3. Lists all participants with their names, join/leave times, and session duration

## Prerequisites

### 1. Enable the Google Meet API

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Create a project (or select an existing one)
3. Navigate to **APIs & Services → Library**
4. Search for **Google Meet API** and enable it

### 2. Create OAuth 2.0 credentials

1. Go to **APIs & Services → Credentials**
2. Click **Create Credentials → OAuth client ID**
3. Choose **Desktop app** as the application type
4. Download the JSON file and save it as `credentials.json` in this directory

### 3. Configure OAuth consent screen

1. Go to **APIs & Services → OAuth consent screen**
2. Add the scope: `https://www.googleapis.com/auth/meetings.space.readonly`
3. Add yourself (or target users) as test users while in "Testing" mode

## Install

```bash
go build -o meet-attendees .
```

## Usage

```bash
# Full URL
./meet-attendees https://meet.google.com/abc-defg-hij

# Just the meeting code
./meet-attendees abc-defg-hij
```

On first run, the tool opens a browser-based OAuth consent flow and caches the token at `~/.meet-attendees-token.json` for subsequent runs.

### Environment variables

| Variable                  | Default            | Description                        |
|---------------------------|--------------------|------------------------------------|
| `GOOGLE_CREDENTIALS_FILE` | `credentials.json` | Path to OAuth client secret file   |

## Example output

```
Meeting code: abc-defg-hij

=== Session 1 (2025-03-20 10:00:00 → 2025-03-20 11:05:32) ===
Conference Record: conferenceRecords/xyz789

  #  NAME            EMAIL                         JOINED              LEFT                DURATION
  -  ----            -----                         ------              ----                --------
  1  Alice Smith     people/1234567890              2025-03-20 10:00:05  2025-03-20 11:05:32  1h5m27s
  2  Bob Jones       people/0987654321              2025-03-20 10:02:11  2025-03-20 11:05:30  1h3m19s
  3  Guest (anon)    —                              2025-03-20 10:15:00  2025-03-20 10:45:00  30m0s
```

## Permissions note

- The Meet API returns participants only for meetings organized within your Google Workspace domain (or meetings you own).
- For personal `@gmail.com` accounts, access may be limited.
- The `meetings.space.readonly` scope provides read-only access to meeting spaces and their participants.

## Troubleshooting

**"No conference records found"** — The meeting must have started (or finished) at least once. Future/scheduled meetings that haven't been joined yet won't have records.

**403 Forbidden** — Ensure the Google Meet API is enabled in your project and the OAuth consent screen includes the correct scope. If your Workspace admin restricts API access, you may need admin approval.

**Token expired** — Delete `~/.meet-attendees-token.json` and re-run to trigger a fresh OAuth flow.
