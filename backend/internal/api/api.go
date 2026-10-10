package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"energy/internal/domain"
	"energy/internal/engine"
	"energy/internal/explain"
	"energy/internal/store"
	"github.com/jackc/pgx/v5"
)

type contextKey string

const actorKey contextKey = "actor"

type attempt struct {
	Count int
	Until time.Time
}
type API struct {
	DB        *store.Store
	Secure    bool
	mu        sync.Mutex
	attempts  map[string]attempt
	Providers explain.Registry
}

func New(db *store.Store, secure bool, registries ...explain.Registry) http.Handler {
	a := &API{DB: db, Secure: secure, attempts: map[string]attempt{}}
	if len(registries) > 0 {
		a.Providers = registries[0]
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Pool.Ping(r.Context()); err != nil {
			problem(w, 503, "database_unavailable", "La base de datos no está disponible.")
			return
		}
		send(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/openapi.json", a.openapi)
	mux.HandleFunc("GET /api/docs", a.docs)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	protected := func(pattern string, h http.HandlerFunc) { mux.Handle(pattern, a.auth(h)) }
	protected("GET /api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, map[string]any{"email": r.Context().Value(actorKey)})
	})
	protected("POST /api/v1/auth/logout", a.logout)
	protected("GET /api/v1/dashboard/summary", a.dashboard)
	protected("GET /api/v1/ai/providers", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, map[string]any{"items": a.Providers.List(), "default": a.Providers.Default(), "allow_new_requests": a.DB.AllowNewAIRequests})
	})
	protected("GET /api/v1/meters", a.meters)
	protected("GET /api/v1/meters/{id}", a.meter)
	protected("GET /api/v1/meters/{id}/readings", a.readings)
	protected("GET /api/v1/meters/{id}/events", a.events)
	protected("GET /api/v1/anomalies", a.anomalies)
	protected("GET /api/v1/anomalies/{id}", a.anomaly)
	protected("GET /api/v1/anomalies/{id}/history", a.history)
	protected("POST /api/v1/anomalies/{id}/actions", a.action)
	protected("POST /api/v1/ai/analyze", a.analyze)
	protected("POST /api/v1/ai/chat", a.chat)
	protected("GET /api/v1/ai/chat", a.chatStatus)
	protected("GET /api/v1/ai/analysis/latest", a.latest)
	protected("GET /api/v1/ai/analysis/{id}", a.analysis)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := store.ID()
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "same-origin")
		defer func() {
			if v := recover(); v != nil {
				slog.Error("http panic", "request_id", requestID)
				problem(w, 500, "internal_error", "Error interno.")
			}
		}()
		if r.Method != "GET" && r.Method != "HEAD" {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host {
					problem(w, 403, "origin_rejected", "Origen no autorizado.")
					return
				}
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout(r.URL.Path))
		defer cancel()
		mux.ServeHTTP(w, r.WithContext(ctx))
	})
}
func send(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func problem(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "status": status, "code": code, "title": message, "request_id": w.Header().Get("X-Request-ID")})
}
func fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, pgx.ErrNoRows) || errors.Is(err, store.ErrNotFound):
		problem(w, 404, "not_found", "No se encontró el recurso.")
	case errors.Is(err, store.ErrConflict):
		problem(w, 409, "conflict", "El estado cambió o la transición no es válida. Actualiza y vuelve a intentarlo.")
	default:
		slog.Error("request failed", "request_id", w.Header().Get("X-Request-ID"), "error", err)
		problem(w, 500, "internal_error", "No se pudo completar la operación.")
	}
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		problem(w, 415, "json_required", "Se requiere Content-Type application/json.")
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		problem(w, 400, "invalid_json", "JSON inválido o campos no permitidos.")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		problem(w, 400, "invalid_json", "Se permite un único objeto JSON.")
		return false
	}
	return true
}
func (a *API) auth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("energy_session")
		if err != nil {
			problem(w, 401, "unauthorized", "Inicia sesión.")
			return
		}
		email, err := a.DB.Session(r.Context(), cookie.Value)
		if err != nil {
			problem(w, 401, "unauthorized", "La sesión expiró o no es válida.")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), actorKey, email)))
	})
}
func (a *API) login(w http.ResponseWriter, r *http.Request) {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	a.mu.Lock()
	now := time.Now()
	for key, v := range a.attempts {
		if now.After(v.Until) {
			delete(a.attempts, key)
		}
	}
	v := a.attempts[ip]
	if v.Count >= 10 {
		a.mu.Unlock()
		w.Header().Set("Retry-After", "60")
		problem(w, 429, "rate_limited", "Demasiados intentos. Espera un minuto.")
		return
	}
	if v.Count == 0 {
		v.Until = now.Add(time.Minute)
	}
	v.Count++
	a.attempts[ip] = v
	a.mu.Unlock()
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	if len(body.Password) > 72 || len(body.Email) > 254 {
		problem(w, 400, "invalid_credentials", "Credenciales inválidas.")
		return
	}
	token, err := a.DB.Login(r.Context(), strings.ToLower(strings.TrimSpace(body.Email)), body.Password)
	if err != nil {
		problem(w, 401, "invalid_credentials", "Correo o contraseña incorrectos.")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "energy_session", Value: token, Path: "/", HttpOnly: true, Secure: a.Secure, SameSite: http.SameSiteStrictMode, MaxAge: 43200})
	send(w, 200, map[string]string{"email": body.Email})
}
func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie("energy_session")
	if err := a.DB.Logout(r.Context(), c.Value); err != nil {
		fail(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "energy_session", Path: "/", HttpOnly: true, Secure: a.Secure, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	send(w, 200, map[string]bool{"ok": true})
}
func page(r *http.Request) (int, int, error) {
	limit, offset := 100, 0
	var err error
	if s := r.URL.Query().Get("limit"); s != "" {
		limit, err = strconv.Atoi(s)
		if err != nil {
			return 0, 0, err
		}
	}
	if s := r.URL.Query().Get("offset"); s != "" {
		offset, err = strconv.Atoi(s)
		if err != nil {
			return 0, 0, err
		}
	}
	if limit < 1 || limit > 1000 || offset < 0 {
		return 0, 0, fmt.Errorf("bad pagination")
	}
	return limit, offset, nil
}
func list[T any](w http.ResponseWriter, r *http.Request, items []T) {
	limit, offset, err := page(r)
	if err != nil {
		problem(w, 400, "invalid_pagination", "limit debe estar entre 1 y 1000; offset debe ser positivo.")
		return
	}
	start := min(offset, len(items))
	end := min(start+limit, len(items))
	send(w, 200, map[string]any{"items": items[start:end], "total": len(items), "limit": limit, "offset": offset})
}
func (a *API) analyze(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider   string `json:"provider"`
		CachedOnly bool   `json:"cached_only"`
	}
	if r.ContentLength != 0 && !decode(w, r, &body) {
		return
	}
	if body.Provider == "" {
		body.Provider = a.Providers.Default()
	}
	if body.Provider != "claude" && body.Provider != "openai" && body.Provider != "rules" {
		problem(w, 400, "invalid_provider", "Selecciona Claude, GPT o reglas verificables.")
		return
	}
	client, ok := a.Providers.Resolve(body.Provider)
	if !ok {
		problem(w, 422, "provider_not_configured", "Falta configurar la clave de este proveedor en el servidor.")
		return
	}
	run, err := a.DB.SelectAnalysis(r.Context(), body.Provider, client.ModelName(), body.CachedOnly)
	if errors.Is(err, store.ErrCacheMiss) {
		problem(w, 404, "cache_miss", "Este proveedor todavía no tiene un análisis guardado para estos datos. No se hizo ninguna llamada a la IA.")
		return
	}
	if errors.Is(err, store.ErrAIRequestsDisabled) {
		problem(w, 409, "ai_requests_disabled", "Las nuevas llamadas de pago están bloqueadas. Solo se pueden consultar análisis guardados.")
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Location", "/api/v1/ai/analysis/"+run.ID)
	status := 202
	if run.Status == "COMPLETED" || run.Status == "FAILED" {
		status = 200
	}
	send(w, status, run)
}
func (a *API) analysis(w http.ResponseWriter, r *http.Request) {
	v, e := a.DB.Run(r.Context(), r.PathValue("id"))
	if e != nil {
		fail(w, e)
		return
	}
	send(w, 200, v)
}
func (a *API) latest(w http.ResponseWriter, r *http.Request) {
	v, e := a.DB.LatestRun(r.Context())
	if e != nil {
		fail(w, e)
		return
	}
	send(w, 200, v)
}
func (a *API) anomalies(w http.ResponseWriter, r *http.Request) {
	fs, err := a.DB.Findings(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	out := []domain.Finding{}
	q := r.URL.Query()
	for _, f := range fs {
		if q.Get("meter_id") != "" && f.MeterID != q.Get("meter_id") {
			continue
		}
		if q.Get("severity") != "" && f.Severity != q.Get("severity") {
			continue
		}
		if q.Get("type") != "" && f.Type != q.Get("type") {
			continue
		}
		if q.Get("status") != "" && f.Status != q.Get("status") {
			continue
		}
		out = append(out, f)
	}
	if !allowed(q.Get("severity"), "HIGH", "MEDIUM", "LOW") || !allowed(q.Get("type"), "REAL_ANOMALY", "DATA_QUALITY", "EXPLAINABLE_ANOMALY", "FALSE_POSITIVE") || !allowed(q.Get("status"), "OPEN", "INVESTIGATING", "RESOLVED", "DISMISSED") {
		problem(w, 400, "invalid_filter", "Filtro de hallazgos no válido.")
		return
	}
	list(w, r, out)
}
func (a *API) anomaly(w http.ResponseWriter, r *http.Request) {
	f, err := a.DB.Finding(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	actions, err := a.DB.Actions(r.Context(), f.ID)
	if err != nil {
		fail(w, err)
		return
	}
	send(w, 200, map[string]any{"finding": f, "actions": actions})
}
func (a *API) history(w http.ResponseWriter, r *http.Request) {
	if _, err := a.DB.Finding(r.Context(), r.PathValue("id")); err != nil {
		fail(w, err)
		return
	}
	items, err := a.DB.History(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	list(w, r, items)
}
func (a *API) action(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Action          string               `json:"action"`
		Note            string               `json:"note"`
		ExpectedVersion int                  `json:"expected_version"`
		Verification    *domain.Verification `json:"verification"`
	}
	if !decode(w, r, &b) {
		return
	}
	if b.ExpectedVersion < 1 || len(strings.TrimSpace(b.Note)) < 5 || len(b.Note) > 2000 {
		problem(w, 400, "invalid_action", "Incluye versión y una nota de 5 a 2000 caracteres.")
		return
	}
	switch b.Action {
	case "NOTE", "INVESTIGATE", "RESOLVE", "DISMISS", "REOPEN":
	default:
		problem(w, 400, "invalid_action", "Acción no válida.")
		return
	}
	if b.Verification != nil && (b.Action != "NOTE" || len(b.Verification.RunID) > 64 || len(b.Verification.StepCode) > 80 || b.Verification.RunID == "" || b.Verification.StepCode == "" || (b.Verification.Status != "CONFIRMED" && b.Verification.Status != "DISCARDED" && b.Verification.Status != "PENDING")) {
		problem(w, 400, "invalid_verification", "Incluye una recomendación válida y su resultado: pendiente, confirmada o descartada.")
		return
	}
	err := a.DB.ActWithVerification(r.Context(), r.PathValue("id"), b.Action, b.Note, r.Context().Value(actorKey).(string), b.ExpectedVersion, b.Verification)
	if err != nil {
		fail(w, err)
		return
	}
	a.anomaly(w, r)
}

type Meter struct {
	Severity      string    `json:"severity,omitempty"`
	AnomalyType   string    `json:"anomaly_type,omitempty"`
	IncidentID    string    `json:"incident_id,omitempty"`
	LastReading   time.Time `json:"last_reading"`
	ID            string    `json:"meter_id"`
	Consumption   float64   `json:"consumption_kwh"`
	Baseline      float64   `json:"baseline_kwh"`
	Variation     float64   `json:"variation_pct"`
	Status        string    `json:"status"`
	Priority      int       `json:"priority"`
	PeriodStart   time.Time `json:"period_start"`
	PeriodEnd     time.Time `json:"period_end"`
	Readings      int       `json:"reading_count"`
	BaselineValid bool      `json:"baseline_valid"`
}

func meterSummaries(in domain.Input, fs []domain.Finding, analyzed bool) []Meter {
	out := []Meter{}
	loc, _ := time.LoadLocation(in.Timezone)
	for id, rs := range engine.Groups(in) {
		p := engine.BuildProfile(rs, engine.DefaultConfig(), loc)
		m := Meter{ID: id, Status: "NOT_ANALYZED", Readings: len(rs), PeriodEnd: rs[len(rs)-1].Timestamp.Add(time.Hour), BaselineValid: p.Valid}
		m.LastReading = rs[len(rs)-1].Timestamp
		m.PeriodStart = m.PeriodEnd.Add(-24 * time.Hour)
		if analyzed && p.Valid {
			m.Status = "NORMAL"
		}
		if !p.Valid {
			m.Status = "INSUFFICIENT_DATA"
		}
		for _, r := range rs {
			if !r.Timestamp.Before(m.PeriodStart) {
				m.Consumption += r.Consumption
				m.Baseline += p.Slots[r.Timestamp.In(loc).Hour()].Consumption
			}
		}
		if m.Baseline > 0 {
			m.Variation = (m.Consumption/m.Baseline - 1) * 100
		}
		for _, f := range fs {
			if f.MeterID == id && f.Status != "RESOLVED" && f.Status != "DISMISSED" && f.Type != "FALSE_POSITIVE" {
				if f.Priority > m.Priority {
					m.Priority = f.Priority
					m.Severity = f.Severity
					m.AnomalyType = f.Type
					m.IncidentID = f.ID
					m.Status = "ALERT"
					if f.Severity == "HIGH" {
						m.Status = "CRITICAL"
					}
				}
			}
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (a *API) state(ctx context.Context) (domain.Input, []domain.Finding, []Meter, error) {
	in, err := a.DB.Input(ctx)
	if err != nil {
		return in, nil, nil, err
	}
	fs, err := a.DB.Findings(ctx)
	if err != nil {
		return in, nil, nil, err
	}
	var analyzed bool
	err = a.DB.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM analysis_runs WHERE status='COMPLETED')").Scan(&analyzed)
	return in, fs, meterSummaries(in, fs, analyzed), err
}
func (a *API) meters(w http.ResponseWriter, r *http.Request) {
	_, _, ms, err := a.state(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	out := []Meter{}
	q := r.URL.Query()
	for _, m := range ms {
		if q.Get("status") != "" && m.Status != q.Get("status") {
			continue
		}
		if !strings.Contains(strings.ToLower(m.ID), strings.ToLower(q.Get("q"))) {
			continue
		}
		out = append(out, m)
	}
	if !allowed(q.Get("status"), "NORMAL", "CRITICAL", "ALERT", "NOT_ANALYZED", "INSUFFICIENT_DATA") {
		problem(w, 400, "invalid_status", "Estado no válido.")
		return
	}
	if !allowed(q.Get("direction"), "asc", "desc") {
		problem(w, 400, "invalid_direction", "Usa asc o desc.")
		return
	}
	if !allowed(q.Get("sort"), "meter_id", "consumption", "variation", "severity") {
		problem(w, 400, "invalid_sort", "Ordenamiento no válido.")
		return
	}
	sortMeters(out, q.Get("sort"), q.Get("direction"))
	list(w, r, out)
}
func (a *API) meter(w http.ResponseWriter, r *http.Request) {
	in, fs, ms, err := a.state(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	for _, m := range ms {
		if m.ID == r.PathValue("id") {
			loc, _ := time.LoadLocation(in.Timezone)
			items := []domain.Finding{}
			for _, f := range fs {
				if f.MeterID == m.ID {
					items = append(items, f)
				}
			}
			rs := engine.Groups(in)[m.ID]
			profile := engine.BuildProfile(rs, engine.DefaultConfig(), loc)
			send(w, 200, map[string]any{"meter": m, "profile": profile, "anomalies": items, "timezone": in.Timezone, "daily_history": dailyHistory(rs, profile, loc), "electrical": electrical(rs, profile, loc)})
			return
		}
	}
	fail(w, store.ErrNotFound)
}
func (a *API) readings(w http.ResponseWriter, r *http.Request) {
	in, err := a.DB.Input(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	rs, ok := engine.Groups(in)[r.PathValue("id")]
	if !ok {
		fail(w, store.ErrNotFound)
		return
	}
	loc, _ := time.LoadLocation(in.Timezone)
	p := engine.BuildProfile(rs, engine.DefaultConfig(), loc)
	var from, to time.Time
	for key, target := range map[string]*time.Time{"from": &from, "to": &to} {
		if val := r.URL.Query().Get(key); val != "" {
			v, e := time.Parse(time.RFC3339, val)
			if e != nil {
				problem(w, 400, "invalid_time", "Usa fechas RFC3339 con zona horaria.")
				return
			}
			*target = v
		}
	}
	if !from.IsZero() && !to.IsZero() && !to.After(from) {
		problem(w, 400, "invalid_period", "to debe ser posterior a from.")
		return
	}
	type point struct {
		domain.Reading
		Baseline        float64 `json:"baseline_kwh"`
		Reference       bool    `json:"is_reference"`
		BaselineVoltage float64 `json:"baseline_voltage_v"`
		BaselineCurrent float64 `json:"baseline_current_a"`
		BaselinePF      float64 `json:"baseline_power_factor"`
	}
	out := []point{}
	for _, v := range rs {
		if !from.IsZero() && v.Timestamp.Before(from) {
			continue
		}
		if !to.IsZero() && !v.Timestamp.Before(to) {
			continue
		}
		s := p.Slots[v.Timestamp.In(loc).Hour()]
		out = append(out, point{v, s.Consumption, v.Timestamp.Before(p.End), s.Voltage, s.Current, s.PF})
	}
	list(w, r, out)
}
func (a *API) events(w http.ResponseWriter, r *http.Request) {
	in, err := a.DB.Input(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	if _, ok := engine.Groups(in)[r.PathValue("id")]; !ok {
		fail(w, store.ErrNotFound)
		return
	}
	out := []domain.Event{}
	for _, e := range in.Events {
		if e.MeterID == r.PathValue("id") {
			out = append(out, e)
		}
	}
	list(w, r, out)
}
func (a *API) dashboard(w http.ResponseWriter, r *http.Request) {
	in, fs, ms, err := a.state(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	total := 0.0
	var start, end time.Time
	daily := map[string]float64{}
	loc, _ := time.LoadLocation(in.Timezone)
	for _, v := range in.Readings {
		total += v.Consumption
		key := v.Timestamp.In(loc).Format("2006-01-02")
		daily[key] += v.Consumption
		if start.IsZero() || v.Timestamp.Before(start) {
			start = v.Timestamp
		}
		if v.Timestamp.After(end) {
			end = v.Timestamp
		}
	}
	high, open := 0, 0
	confidence := 0.0
	top := []domain.Finding{}
	for _, f := range fs {
		confidence += f.Confidence
		if f.Status != "RESOLVED" && f.Status != "DISMISSED" && f.Type != "FALSE_POSITIVE" {
			open++
			if f.Severity == "HIGH" {
				high++
			}
			if len(top) < 3 {
				top = append(top, f)
			}
		}
	}
	if len(fs) > 0 {
		confidence /= float64(len(fs))
	}
	var last any
	run, e := a.DB.LatestRun(r.Context())
	if e == nil {
		last = run
	} else if !errors.Is(e, pgx.ErrNoRows) {
		fail(w, e)
		return
	}
	counts := map[string]int{}
	recent, baseline := 0.0, 0.0
	for _, m := range ms {
		counts[m.Status]++
		recent += m.Consumption
		baseline += m.Baseline
	}
	variation := 0.0
	if baseline > 0 {
		variation = (recent/baseline - 1) * 100
	}
	send(w, 200, map[string]any{"meters": len(ms), "readings": len(in.Readings), "consumption_kwh": total, "anomalies": len(fs), "open_anomalies": open, "high_priority": high, "evidence_score": confidence, "confidence_meaning": "Promedio del índice de evidencia; no es precisión ni probabilidad.", "last_analysis": last, "top_findings": top, "daily_consumption": daily, "period_start": start, "period_end": end.Add(time.Hour), "timezone": in.Timezone, "engine_version": engine.Version, "status_counts": counts, "consumption_24h": recent, "baseline_24h": baseline, "variation_24h": variation})
}
