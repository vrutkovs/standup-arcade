# meet-attendees

A web server that fetches the list of participants who actually joined a Google Meet meeting.

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
# Start on default port (localhost:8080)
./meet-attendees

# Start on a specific address
./meet-attendees localhost:8081
```

Once running, navigate to `http://localhost:8080` in your browser. On first API request, the tool might open a browser-based OAuth consent flow and cache the token at `~/.meet-attendees-token.json` for subsequent runs.

### Environment variables

| Variable                  | Default            | Description                        |
|---------------------------|--------------------|------------------------------------|
| `GOOGLE_CREDENTIALS_FILE` | `credentials.json` | Path to OAuth client secret file   |

## API Usage

The web server exposes an API to retrieve attendees:

```bash
curl "http://localhost:8080/api/attendees?meeting=abc-defg-hij"
```

Example response:

```json
{
  "attendees": [
    "Alice Smith",
    "Bob Jones"
  ]
}
```

## Permissions note

- The Meet API returns participants only for meetings organized within your Google Workspace domain (or meetings you own).
- For personal `@gmail.com` accounts, access may be limited.
- The `meetings.space.readonly` scope provides read-only access to meeting spaces and their participants.

## Troubleshooting

**"No conference records found"** — The meeting must have started (or finished) at least once. Future/scheduled meetings that haven't been joined yet won't have records.

**403 Forbidden** — Ensure the Google Meet API is enabled in your project and the OAuth consent screen includes the correct scope. If your Workspace admin restricts API access, you may need admin approval.

**Token expired** — Delete `~/.meet-attendees-token.json` and re-run to trigger a fresh OAuth flow.
