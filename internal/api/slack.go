package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// slackMaxSkew bounds how far a request's timestamp may drift from now. Slack
// recommends five minutes; it is what stops a captured request being replayed.
const slackMaxSkew = 5 * time.Minute

var errSlackUnverified = errors.New("slack signature verification failed")

// verifySlackRequest authenticates a Slack request and returns its form body.
//
// Slack signs the raw body, so the body must be read before parsing and the
// exact bytes fed to the HMAC. The scheme is:
//
//	base      = "v0:" + timestamp + ":" + rawBody
//	signature = "v0=" + hex(hmac_sha256(signingSecret, base))
func verifySlackRequest(r *http.Request, signingSecret string) (url.Values, error) {
	if signingSecret == "" {
		return nil, errors.New("slack signing secret not configured")
	}

	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	ts := r.Header.Get("X-Slack-Request-Timestamp")
	secs, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return nil, errSlackUnverified
	}
	if skew := time.Since(time.Unix(secs, 0)); skew > slackMaxSkew || skew < -slackMaxSkew {
		return nil, errSlackUnverified
	}

	mac := hmac.New(sha256.New, []byte(signingSecret))
	fmt.Fprintf(mac, "v0:%s:%s", ts, raw)
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	// Constant-time compare so a wrong signature leaks nothing by timing.
	if !hmac.Equal([]byte(expected), []byte(r.Header.Get("X-Slack-Signature"))) {
		return nil, errSlackUnverified
	}

	return url.ParseQuery(string(raw))
}

// slackResponse is the immediate reply to a slash command. "ephemeral" shows
// only to the caller; "in_channel" posts to everyone in the channel.
type slackResponse struct {
	ResponseType string `json:"response_type"`
	Text         string `json:"text"`
}

func slackEphemeral(text string) slackResponse {
	return slackResponse{ResponseType: "ephemeral", Text: text}
}

func slackInChannel(text string) slackResponse {
	return slackResponse{ResponseType: "in_channel", Text: text}
}

// slackCommand godoc
// @Summary      Slack slash command
// @Description  Handles /soda slash commands. Authenticated by Slack's request signature, not by API key or JWT.
// @Tags         integrations
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Success      200  {object}  object{response_type=string,text=string}
// @Failure      401  {object}  ErrorResponse
// @Router       /integrations/slack/commands [post]
func (h *handlers) slackCommand(w http.ResponseWriter, r *http.Request) {
	form, err := verifySlackRequest(r, h.slackSigningSecret)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	args := strings.Fields(form.Get("text"))
	sub := "status"
	if len(args) > 0 {
		sub = strings.ToLower(args[0])
	}

	switch sub {
	case "status":
		writeJSON(w, http.StatusOK, h.slackStatus(r))
	case "top":
		writeJSON(w, http.StatusOK, h.slackTop(r))
	case "help":
		writeJSON(w, http.StatusOK, slackEphemeral(slackHelpText))
	default:
		writeJSON(w, http.StatusOK, slackEphemeral(
			fmt.Sprintf("Unknown command %q.\n\n%s", sub, slackHelpText)))
	}
}

const slackHelpText = "*SodaTracker*\n" +
	"`/soda status` — what's in the machine right now\n" +
	"`/soda top` — most dispensed over the last 7 days\n" +
	"`/soda help` — this message"

// slackStatus reports current stock for every machine.
func (h *handlers) slackStatus(r *http.Request) slackResponse {
	ctx := r.Context()

	machines, err := h.store.ListMachines(ctx)
	if err != nil {
		return slackEphemeral("Couldn't reach SodaTracker just now.")
	}
	if len(machines) == 0 {
		return slackEphemeral("No machines are configured yet.")
	}

	var b strings.Builder
	for i, m := range machines {
		if i > 0 {
			b.WriteString("\n")
		}
		status, err := h.svc.GetMachineStatus(ctx, m.ID)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "*%s*\n", m.Name)
		for _, s := range status.Slots {
			name := fmt.Sprintf("Slot %d", s.Slot.Position)
			if s.Product != nil && s.Product.Name != "" {
				name = s.Product.Name
			}
			fmt.Fprintf(&b, "%s %s — %d left\n", slackStatusIcon(s.Status), name, s.CurrentStock)
		}
	}
	return slackInChannel(strings.TrimRight(b.String(), "\n"))
}

func slackStatusIcon(status string) string {
	switch status {
	case "ok":
		return ":large_green_circle:"
	case "low":
		return ":large_yellow_circle:"
	default:
		return ":red_circle:"
	}
}

// slackTop ranks products by dispense count over the last 7 days.
func (h *handlers) slackTop(r *http.Request) slackResponse {
	ctx := r.Context()

	machines, err := h.store.ListMachines(ctx)
	if err != nil || len(machines) == 0 {
		return slackEphemeral("No dispense data yet.")
	}

	since := time.Now().AddDate(0, 0, -7)
	counts := map[string]int{}
	for _, m := range machines {
		stats, err := h.svc.GetDispenseStats(ctx, m.ID, since)
		if err != nil {
			continue
		}
		for _, p := range stats.TopProducts {
			counts[p.ProductName] += p.Count
		}
	}
	if len(counts) == 0 {
		return slackEphemeral("Nothing has been dispensed in the last 7 days.")
	}

	type row struct {
		name  string
		count int
	}
	rows := make([]row, 0, len(counts))
	for name, count := range counts {
		rows = append(rows, row{name, count})
	}
	// Sort by count, then name, so equal counts do not shuffle between calls.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].count != rows[j].count {
			return rows[i].count > rows[j].count
		}
		return rows[i].name < rows[j].name
	})
	if len(rows) > 5 {
		rows = rows[:5]
	}

	var b strings.Builder
	b.WriteString("*Most dispensed, last 7 days*\n")
	for i, r := range rows {
		fmt.Fprintf(&b, "%d. %s — %d\n", i+1, r.name, r.count)
	}
	return slackInChannel(strings.TrimRight(b.String(), "\n"))
}
