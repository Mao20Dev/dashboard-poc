package api

import (
	"energy/internal/domain"
	"energy/internal/explain"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"
)

func schemaFor(t reflect.Type) map[string]any {
	if t.Kind() == reflect.Pointer {
		return schemaFor(t.Elem())
	}
	if t == reflect.TypeOf(time.Time{}) {
		return map[string]any{"type": "string", "format": "date-time"}
	}
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int64:
		return map[string]any{"type": "integer"}
	case reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": schemaFor(t.Elem())}
	case reflect.Struct:
		props := map[string]any{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			if tag == "-" {
				continue
			}
			name := strings.Split(tag, ",")[0]
			if name == "" {
				name = f.Name
			}
			props[name] = schemaFor(f.Type)
		}
		return map[string]any{"type": "object", "properties": props}
	default:
		return map[string]any{"type": "object", "additionalProperties": true}
	}
}
func (a *API) openapi(w http.ResponseWriter, r *http.Request) {
	ref := func(name string) map[string]any { return map[string]any{"$ref": "#/components/schemas/" + name} }
	collection := func(name string) map[string]any {
		return map[string]any{"type": "object", "properties": map[string]any{"items": map[string]any{"type": "array", "items": ref(name)}, "total": map[string]string{"type": "integer"}, "limit": map[string]string{"type": "integer"}, "offset": map[string]string{"type": "integer"}}}
	}
	schemas := map[string]any{"Finding": schemaFor(reflect.TypeOf(domain.Finding{})), "Reading": schemaFor(reflect.TypeOf(domain.Reading{})), "Event": schemaFor(reflect.TypeOf(domain.Event{})), "Run": schemaFor(reflect.TypeOf(domain.Run{})), "Meter": schemaFor(reflect.TypeOf(Meter{})), "Action": schemaFor(reflect.TypeOf(domain.Action{})), "User": map[string]any{"type": "object", "properties": map[string]any{"email": map[string]string{"type": "string"}}}, "Dashboard": map[string]any{"type": "object", "description": "KPIs del periodo completo. last_analysis es el último intento; anomalías proceden del último análisis completado. evidence_score no es una probabilidad."}, "MeterDetail": map[string]any{"type": "object", "properties": map[string]any{"meter": ref("Meter"), "anomalies": map[string]any{"type": "array", "items": ref("Finding")}, "profile": map[string]any{"type": "object"}, "timezone": map[string]string{"type": "string"}}}, "Investigation": map[string]any{"type": "object", "properties": map[string]any{"finding": ref("Finding"), "actions": map[string]any{"type": "array", "items": ref("Action")}}}}
	type route struct {
		Method, Path, Description, Schema string
		List                              bool
	}
	routes := []route{
		{"post", "/ai/chat", "Conversación eléctrica con Sonnet, contexto recuperado del sistema, caché persistente y límite diario.", "ChatResponse", false},
		{"get", "/ai/chat", "Configuración y consumo de consultas del chat.", "ChatStatus", false},
		{"get", "/ai/providers", "Proveedores configurados y selección predeterminada. Nunca devuelve claves.", "Providers", false},
		{"post", "/auth/login", "Iniciar sesión. Cookie HttpOnly; 10 intentos por minuto e IP.", "User", false},
		{"get", "/auth/me", "Usuario autenticado.", "User", false}, {"post", "/auth/logout", "Revocar la sesión.", "User", false},
		{"get", "/dashboard/summary", "Resumen del periodo completo y último intento de análisis.", "Dashboard", false},
		{"get", "/meters", "Últimas 24 horas disponibles por medidor, referencia comparable y estado operativo.", "Meter", true},
		{"get", "/meters/{id}", "Perfil horario y anomalías del medidor.", "MeterDetail", false},
		{"get", "/meters/{id}/readings", "Lecturas horarias con baseline_kwh e is_reference. Intervalo [from,to).", "Reading", true},
		{"get", "/meters/{id}/events", "Eventos registrados: contexto, no instrucciones.", "Event", true},
		{"get", "/anomalies", "Hallazgos del último análisis completado con estado operativo actual.", "Finding", true},
		{"get", "/anomalies/{id}", "Expediente: evidencia, explicación y acciones.", "Investigation", false},
		{"get", "/anomalies/{id}/history", "Observaciones inmutables por ejecución; conserva el estado registrado en cada observación.", "Finding", true},
		{"post", "/anomalies/{id}/actions", "Registrar acción con control optimista expected_version. Conflictos devuelven 409.", "Investigation", false},
		{"post", "/ai/analyze", "Recuperar análisis persistido por datos, proveedor, modelo y versión. 200 si existe; 202 si se admite un trabajo. cached_only nunca genera. 409 si las nuevas llamadas de pago están bloqueadas; 404 si falta caché.", "Run", false},
		{"get", "/ai/analysis/latest", "Último análisis seleccionado, incluso desde caché; 404 antes del primero.", "Run", false},
		{"get", "/ai/analysis/{id}", "Estado persistido: PENDING, RUNNING, COMPLETED o FAILED.", "Run", false},
	}
	schemas["ChatResponse"] = schemaFor(reflect.TypeOf(ChatResponse{}))
	schemas["ChatStatus"] = map[string]any{"type": "object"}
	paths := map[string]any{}
	schemas["Providers"] = map[string]any{"type": "object", "properties": map[string]any{"items": map[string]any{"type": "array", "items": schemaFor(reflect.TypeOf(explain.Provider{}))}, "default": map[string]string{"type": "string"}, "allow_new_requests": map[string]string{"type": "boolean"}}}
	schemas["DailyHistory"] = schemaFor(reflect.TypeOf(Day{}))
	schemas["Electrical"] = schemaFor(reflect.TypeOf(Electrical{}))
	detail := schemas["MeterDetail"].(map[string]any)["properties"].(map[string]any)
	detail["daily_history"] = map[string]any{"type": "array", "items": ref("DailyHistory")}
	detail["electrical"] = map[string]any{"type": "array", "items": ref("Electrical")}
	reading := schemas["Reading"].(map[string]any)["properties"].(map[string]any)
	for _, name := range []string{"baseline_kwh", "baseline_voltage_v", "baseline_current_a", "baseline_power_factor"} {
		reading[name] = map[string]string{"type": "number"}
	}
	reading["is_reference"] = map[string]string{"type": "boolean"}
	for _, route := range routes {
		shape := ref(route.Schema)
		if route.List {
			shape = collection(route.Schema)
		}
		status := "200"
		if route.Path == "/ai/analyze" {
			status = "202"
		}
		operation := map[string]any{"summary": route.Description, "responses": map[string]any{status: map[string]any{"description": "Éxito", "content": map[string]any{"application/json": map[string]any{"schema": shape}}}, "default": map[string]any{"description": "Error application/problem+json con status, code, title y request_id."}}, "security": []any{map[string]any{"session": []string{}}}}
		params := []any{}
		if strings.Contains(route.Path, "{id}") {
			params = append(params, map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]string{"type": "string"}})
		}
		if route.List {
			params = append(params, map[string]any{"name": "limit", "in": "query", "schema": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000, "default": 100}}, map[string]any{"name": "offset", "in": "query", "schema": map[string]any{"type": "integer", "minimum": 0, "default": 0}})
		}
		filters := []string{}
		if route.Path == "/meters" {
			filters = []string{"q", "status", "sort", "direction"}
		}
		if route.Path == "/anomalies" {
			filters = []string{"meter_id", "severity", "type", "status"}
		}
		if strings.HasSuffix(route.Path, "/readings") {
			filters = []string{"from", "to"}
		}
		for _, key := range filters {
			params = append(params, map[string]any{"name": key, "in": "query", "schema": map[string]string{"type": "string"}})
		}
		operation["parameters"] = params
		if route.Path == "/ai/analyze" {
			operation["responses"].(map[string]any)["200"] = map[string]any{"description": "Respuesta persistida reutilizada; no llamada a la IA", "content": map[string]any{"application/json": map[string]any{"schema": shape}}}
			operation["requestBody"] = map[string]any{"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"cached_only": map[string]any{"type": "boolean", "default": false}, "provider": map[string]any{"type": "string", "enum": []string{"claude", "openai", "rules"}}}}}}}
		}
		if route.Path == "/auth/login" {
			operation["security"] = []any{}
			operation["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "required": []string{"email", "password"}, "properties": map[string]any{"email": map[string]string{"type": "string"}, "password": map[string]string{"type": "string"}}}}}}
		}
		if strings.HasSuffix(route.Path, "/actions") {
			operation["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "required": []string{"action", "note", "expected_version"}, "properties": map[string]any{"action": map[string]any{"type": "string", "enum": []string{"NOTE", "INVESTIGATE", "RESOLVE", "DISMISS", "REOPEN"}}, "note": map[string]any{"type": "string", "minLength": 5, "maxLength": 2000}, "verification": schemaFor(reflect.TypeOf(domain.Verification{})), "expected_version": map[string]any{"type": "integer", "minimum": 1}}}}}}
		}
		if route.Path == "/ai/chat" && route.Method == "post" {
			operation["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": schemaFor(reflect.TypeOf(ChatRequest{}))}}}
		}
		if paths[route.Path] == nil {
			paths[route.Path] = map[string]any{}
		}
		paths[route.Path].(map[string]any)[route.Method] = operation
	}
	send(w, 200, map[string]any{"openapi": "3.0.3", "info": map[string]string{"title": "Energy Investigation API", "version": "1.0.0", "description": "Fechas RFC3339 con zona. CSV interpretado en America/Bogota. API autenticada por cookie. Índices de confianza heurísticos."}, "servers": []any{map[string]string{"url": "/api/v1"}}, "paths": paths, "components": map[string]any{"securitySchemes": map[string]any{"session": map[string]string{"type": "apiKey", "in": "cookie", "name": "energy_session"}}, "schemas": schemas}})
}
func (a *API) docs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<!doctype html><html lang="es"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Energy API</title><style>body{font:16px system-ui;max-width:1000px;margin:50px auto;padding:20px;background:#f5f7fa;color:#162b3b}article{background:white;padding:18px;margin:12px 0;border:1px solid #dbe4ec;border-radius:10px}code{color:#00695c}pre{overflow:auto}a{color:#00695c}</style><h1>Energy Investigation API</h1><p>Contrato OpenAPI descargable e inspección de endpoints, sin dependencias externas.</p><p><a href="/api/openapi.json">Descargar OpenAPI JSON</a> · <a href="/">Abrir consola</a></p><p>Inicia sesión en la consola para usar la cookie HttpOnly. Periodos [from,to) en RFC3339. Los errores incluyen un request_id. El índice de evidencia no representa probabilidad de avería.</p><main id="routes">Cargando contrato…</main><script>fetch('/api/openapi.json').then(r=>r.json()).then(s=>{const root=document.getElementById('routes');root.replaceChildren();Object.entries(s.paths).forEach(([path,methods])=>Object.entries(methods).forEach(([method,op])=>{const card=document.createElement('article'),title=document.createElement('h2'),p=document.createElement('p'),details=document.createElement('details'),label=document.createElement('summary'),pre=document.createElement('pre');title.textContent=method.toUpperCase()+' /api/v1'+path;p.textContent=op.summary;label.textContent='Contrato';pre.textContent=JSON.stringify(op,null,2);details.append(label,pre);card.append(title,p,details);root.append(card)}))}).catch(()=>document.getElementById('routes').textContent='No fue posible cargar el contrato.');</script></html>`)
}
