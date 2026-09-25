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
	"strconv"
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
	writeJSON(w, http.StatusOK, h.slackCmds.Run(r.Context(), form.Get("text")))
}
