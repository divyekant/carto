package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SlackSource fetches messages and threads from a Slack channel.
type SlackSource struct {
	channelID    string
	token        string
	baseURL      string
	messageLimit int
	http         http.Client
}

var _ Source = (*SlackSource)(nil)

// NewSlackSource creates an unconfigured Slack source with sensible defaults.
func NewSlackSource() *SlackSource {
	return &SlackSource{
		baseURL:      "https://slack.com/api",
		messageLimit: 100,
		http:         http.Client{Timeout: 15 * time.Second},
	}
}

func (s *SlackSource) Name() string { return "slack" }
func (s *SlackSource) Scope() Scope { return ProjectScope }

func (s *SlackSource) Configure(cfg SourceConfig) error {
	s.channelID = cfg.Settings["channel_id"]
	if t, ok := cfg.Credentials["slack_token"]; ok {
		s.token = t
	}
	if s.channelID == "" {
		return fmt.Errorf("slack: channel_id is required")
	}
	return nil
}

func (s *SlackSource) Fetch(ctx context.Context, req FetchRequest) ([]Artifact, error) {
	messages, err := s.fetchHistory(ctx)
	if err != nil {
		return nil, fmt.Errorf("slack: fetch history: %w", err)
	}

	var artifacts []Artifact
	for _, msg := range messages {
		a := s.messageToArtifact(msg)

		// If the message is a thread starter with replies, fetch the full thread.
		if msg.ThreadTS != "" && msg.ThreadTS == msg.TS && msg.ReplyCount > 0 {
			replies, err := s.fetchReplies(ctx, msg.ThreadTS)
			if err != nil {
				return nil, fmt.Errorf("slack: fetch replies for %s: %w", msg.TS, err)
			}
			a.Body = s.buildThreadBody(msg, replies)
			a.Tags["type"] = "thread"
		}

		artifacts = append(artifacts, a)
	}
	return artifacts, nil
}

// --- Slack API types ---

type slackMessage struct {
	TS         string `json:"ts"`
	ThreadTS   string `json:"thread_ts"`
	Text       string `json:"text"`
	User       string `json:"user"`
	ReplyCount int    `json:"reply_count"`
}

type slackHistoryResponse struct {
	OK       bool           `json:"ok"`
	Error    string         `json:"error"`
	Messages []slackMessage `json:"messages"`
}

type slackRepliesResponse struct {
	OK       bool           `json:"ok"`
	Error    string         `json:"error"`
	Messages []slackMessage `json:"messages"`
}

// --- internal helpers ---

func (s *SlackSource) slackGet(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+path, nil)
	if err != nil {
		return err
	}
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func (s *SlackSource) fetchHistory(ctx context.Context) ([]slackMessage, error) {
	path := fmt.Sprintf("/conversations.history?channel=%s&limit=%d", s.channelID, s.messageLimit)
	var resp slackHistoryResponse
	if err := s.slackGet(ctx, path, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("slack API error: %s", resp.Error)
	}
	return resp.Messages, nil
}

func (s *SlackSource) fetchReplies(ctx context.Context, threadTS string) ([]slackMessage, error) {
	path := fmt.Sprintf("/conversations.replies?channel=%s&ts=%s", s.channelID, threadTS)
	var resp slackRepliesResponse
	if err := s.slackGet(ctx, path, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("slack API error: %s", resp.Error)
	}
	return resp.Messages, nil
}

func (s *SlackSource) messageToArtifact(msg slackMessage) Artifact {
	return Artifact{
		Source:   "slack",
		Category: Context,
		ID:       msg.TS,
		Title:    truncateBody(msg.Text, 80),
		Body:     truncateBody(msg.Text, 2000),
		URL:      "",
		Date:     parseSlackTS(msg.TS),
		Author:   msg.User,
		Tags:     map[string]string{"type": "message"},
	}
}

func (s *SlackSource) buildThreadBody(parent slackMessage, replies []slackMessage) string {
	var b strings.Builder
	b.WriteString(parent.User)
	b.WriteString(": ")
	b.WriteString(parent.Text)
	for _, r := range replies {
		if r.TS == parent.TS {
			continue // skip the parent message which Slack includes in replies
		}
		b.WriteString("\n")
		b.WriteString(r.User)
		b.WriteString(": ")
		b.WriteString(r.Text)
	}
	return truncateBody(b.String(), 2000)
}

// parseSlackTS converts a Slack timestamp string like "1234567890.123456" to time.Time.
func parseSlackTS(ts string) time.Time {
	parts := strings.SplitN(ts, ".", 2)
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}
	}
	var nsec int64
	if len(parts) == 2 {
		// Slack microseconds - pad or trim to 6 digits then convert to nanoseconds.
		frac := parts[1]
		for len(frac) < 6 {
			frac += "0"
		}
		if len(frac) > 6 {
			frac = frac[:6]
		}
		us, err := strconv.ParseInt(frac, 10, 64)
		if err == nil {
			nsec = us * 1000
		}
	}
	// Guard against overflow.
	if sec > math.MaxInt64/int64(time.Second) {
		return time.Time{}
	}
	return time.Unix(sec, nsec)
}
