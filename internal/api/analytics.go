package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jeffwnelson/sodatracker/internal/service"
)

// parseAnalyticsQuery reads the query parameters shared by every analytics
// endpoint: machineId (required), from/to (RFC3339), tz (IANA name) and bucket.
func parseAnalyticsQuery(r *http.Request) (service.AnalyticsQuery, error) {
	v := r.URL.Query()
	q := service.AnalyticsQuery{MachineID: v.Get("machineId"), Location: time.UTC}
	if q.MachineID == "" {
		return q, fmt.Errorf("machineId is required")
	}
	for name, dst := range map[string]*time.Time{"from": &q.From, "to": &q.To} {
		if s := v.Get(name); s != "" {
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				return q, fmt.Errorf("%s must be an RFC3339 time", name)
			}
			*dst = t
		}
	}
	if !q.From.IsZero() && !q.To.IsZero() && !q.From.Before(q.To) {
		return q, fmt.Errorf("from must be before to")
	}
	if tz := v.Get("tz"); tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return q, fmt.Errorf("unknown timezone %q", tz)
		}
		q.Location = loc
	}
	switch b := v.Get("bucket"); b {
	case "", service.BucketDay, service.BucketWeek, service.BucketMonth:
		q.Bucket = b
	default:
		return q, fmt.Errorf("bucket must be day, week or month")
	}
	return q, nil
}

// serveAnalytics runs one analytics computation and writes the result, with
// 400 for bad parameters and 404 for an unknown machine.
func serveAnalytics[T any](w http.ResponseWriter, r *http.Request, fn func(context.Context, service.AnalyticsQuery) (*T, error)) {
	q, err := parseAnalyticsQuery(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := fn(r.Context(), q)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if out == nil {
		writeErr(w, http.StatusNotFound, "machine not found")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// getConsumptionAnalytics godoc
// @Summary      Consumption analytics
// @Description  Dispense volume over time with a moving-average trend, a weekday-by-hour heatmap, and weekday vs weekend rates. The window's start is clamped to when tracking began.
// @Tags         Analytics
// @Produce      json
// @Param        machineId  query     string  true   "Machine ID"
// @Param        from       query     string  false  "RFC3339 start (default: 30 days before to)"
// @Param        to         query     string  false  "RFC3339 end (default and maximum: now)"
// @Param        tz         query     string  false  "IANA timezone for buckets and hours (default: UTC)"
// @Param        bucket     query     string  false  "day, week or month (default: from window length)"
// @Success      200        {object}  service.ConsumptionAnalytics
// @Failure      400        {object}  ErrorResponse
// @Failure      404        {object}  ErrorResponse
// @Failure      500        {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /analytics/consumption [get]
func (h *handlers) getConsumptionAnalytics(w http.ResponseWriter, r *http.Request) {
	serveAnalytics(w, r, h.svc.GetConsumptionAnalytics)
}

// getProductAnalytics godoc
// @Summary      Product analytics
// @Description  Product ranking and share with the previous window's rank, per-bucket counts by product, and counts by part of day. Products in a slot appear even with no dispenses.
// @Tags         Analytics
// @Produce      json
// @Param        machineId  query     string  true   "Machine ID"
// @Param        from       query     string  false  "RFC3339 start (default: 30 days before to)"
// @Param        to         query     string  false  "RFC3339 end (default and maximum: now)"
// @Param        tz         query     string  false  "IANA timezone for buckets and hours (default: UTC)"
// @Param        bucket     query     string  false  "day, week or month (default: from window length)"
// @Success      200        {object}  service.ProductAnalytics
// @Failure      400        {object}  ErrorResponse
// @Failure      404        {object}  ErrorResponse
// @Failure      500        {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /analytics/products [get]
func (h *handlers) getProductAnalytics(w http.ResponseWriter, r *http.Request) {
	serveAnalytics(w, r, h.svc.GetProductAnalytics)
}

// getInventoryAnalytics godoc
// @Summary      Inventory analytics
// @Description  Stock levels over time rebuilt from restocks and dispenses, hours each slot sat empty, restock intervals, and sell-through.
// @Tags         Analytics
// @Produce      json
// @Param        machineId  query     string  true   "Machine ID"
// @Param        from       query     string  false  "RFC3339 start (default: 30 days before to)"
// @Param        to         query     string  false  "RFC3339 end (default and maximum: now)"
// @Param        tz         query     string  false  "IANA timezone for buckets and hours (default: UTC)"
// @Param        bucket     query     string  false  "day, week or month (default: from window length)"
// @Success      200        {object}  service.InventoryAnalytics
// @Failure      400        {object}  ErrorResponse
// @Failure      404        {object}  ErrorResponse
// @Failure      500        {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /analytics/inventory [get]
func (h *handlers) getInventoryAnalytics(w http.ResponseWriter, r *http.Request) {
	serveAnalytics(w, r, h.svc.GetInventoryAnalytics)
}

// getSpendAnalytics godoc
// @Summary      Spend analytics
// @Description  Spend per bucket with a running total, spend by product, the previous window's total, and a projected monthly figure.
// @Tags         Analytics
// @Produce      json
// @Param        machineId  query     string  true   "Machine ID"
// @Param        from       query     string  false  "RFC3339 start (default: 30 days before to)"
// @Param        to         query     string  false  "RFC3339 end (default and maximum: now)"
// @Param        tz         query     string  false  "IANA timezone for buckets and hours (default: UTC)"
// @Param        bucket     query     string  false  "day, week or month (default: from window length)"
// @Success      200        {object}  service.SpendAnalytics
// @Failure      400        {object}  ErrorResponse
// @Failure      404        {object}  ErrorResponse
// @Failure      500        {object}  ErrorResponse
// @Security     ApiKeyAuth || BearerAuth
// @Router       /analytics/spend [get]
func (h *handlers) getSpendAnalytics(w http.ResponseWriter, r *http.Request) {
	serveAnalytics(w, r, h.svc.GetSpendAnalytics)
}
