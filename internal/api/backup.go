package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/backup"
)

// getBackupStatus godoc
// @Summary      Backup status
// @Description  Local snapshots, the last push to the backup repo, and whether the backup is stale
// @Tags         Admin
// @Produce      json
// @Success      200  {object}  backup.Status
// @Security     ApiKeyAuth || BearerAuth
// @Router       /admin/backup [get]
func (h *handlers) getBackupStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.backups.Status())
}

// runBackup godoc
// @Summary      Back up now
// @Description  Takes a snapshot, prunes old ones, and pushes it to the backup repo when one is configured
// @Tags         Admin
// @Produce      json
// @Success      200  {object}  backup.Status
// @Failure      500  {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /admin/backup [post]
func (h *handlers) runBackup(w http.ResponseWriter, r *http.Request) {
	status, err := h.backups.BackupNow(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, status.LastError)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// configureBackup godoc
// @Summary      Configure backups
// @Description  Sets the GitHub repo and token snapshots are pushed to, the snapshot interval (0 = off) and how many local snapshots to keep. Omit token to keep the stored one, or send "" to clear it. A new repo or token is checked against GitHub before it's saved. The token is never returned.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Param        body  body      backup.Update  true  "Backup settings"
// @Success      200   {object}  backup.Status
// @Failure      400   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /admin/backup/config [put]
func (h *handlers) configureBackup(w http.ResponseWriter, r *http.Request) {
	var u backup.Update
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	status, err := h.backups.Configure(r.Context(), u)
	var invalid *backup.InvalidError
	switch {
	case errors.As(err, &invalid):
		writeErr(w, http.StatusBadRequest, err.Error())
	case err != nil:
		writeErr(w, http.StatusInternalServerError, err.Error())
	default:
		writeJSON(w, http.StatusOK, status)
	}
}

// downloadBackup godoc
// @Summary      Download a backup
// @Description  Returns a fresh gzipped SQLite snapshot of the whole database. It isn't kept or pushed.
// @Tags         Admin
// @Produce      application/gzip
// @Success      200  {file}    file
// @Failure      500  {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /admin/backup/download [get]
func (h *handlers) downloadBackup(w http.ResponseWriter, r *http.Request) {
	// Buffered so a failed snapshot can still answer with an error. The
	// database is small (about 100 KB).
	var buf bytes.Buffer
	if err := h.backups.WriteSnapshot(r.Context(), &buf); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+backup.FileName(time.Now())+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}
