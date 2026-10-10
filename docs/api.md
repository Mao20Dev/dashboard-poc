# API de investigación

Base: `/api/v1`. Contrato completo generado por el servidor: `/api/openapi.json`. Explorador sin dependencias externas: `/api/docs`.

| Método | Ruta | Función |
|---|---|---|
| POST | /auth/login | Crear sesión con email/password |
| GET | /auth/me | Consultar usuario de sesión |
| POST | /auth/logout | Revocar sesión |
| GET | /dashboard/summary | KPIs del periodo completo y último intento |
| GET | /meters | Filtrar y ordenar medidores |
| GET | /meters/{id} | Detalle, perfil y hallazgos |
| GET | /meters/{id}/readings | Serie horaria con baseline |
| GET | /meters/{id}/events | Contexto operativo |
| POST | /ai/analyze | Recuperar caché (200) o admitir trabajo (202) |
| GET | /ai/providers | Proveedores, modelo configurado y selección predeterminada; sin claves |
| GET | /ai/chat | Disponibilidad de Sonnet, modelo, cuota diaria y consultas utilizadas |
| POST | /ai/chat | Preguntar a Sonnet con contexto recuperado, fuentes y caché persistente |
| GET | /ai/analysis/latest | Último análisis seleccionado o 404 |
| GET | /ai/analysis/{id} | Estado y resumen de ejecución |
| GET | /anomalies | Hallazgos del último análisis completado |
| GET | /anomalies/{id} | Evidencia, explicación e historial de acciones |
| GET | /anomalies/{id}/history | Observaciones inmutables por ejecución |
| POST | /anomalies/{id}/actions | Registrar transición/nota con versión esperada |

Los listados devuelven `{items,total,limit,offset}`; `limit` está entre 1 y 1000. Medidores acepta `q`, `status`, `sort=meter_id|consumption|variation|severity` y `direction=asc|desc`. Por defecto, números descendentes e identificadores ascendentes; los empates numéricos usan ID ascendente. Anomalías acepta `meter_id`, `severity`, `type` y `status`. Valores de filtros enumerados inválidos devuelven 400.

`POST /ai/analyze` acepta `{ "provider": "claude" }`, `openai` o `rules`. Sin selección usa el primer proveedor configurado (Claude, GPT, reglas). Un proveedor desconocido devuelve 400; uno sin clave devuelve 422 antes de encolar. Se reutilizan trabajos activos y terminados por entrada, proveedor, modelo y versión narrativa. `cached_only:true` solo selecciona una respuesta existente; sin caché devuelve 404, nunca genera. Por defecto `AI_ALLOW_NEW_REQUESTS=false` bloquea una generación pagada nueva con 409; reglas locales siguen disponibles. Los fallos también se conservan para impedir reintentos pagados automáticos. `cache_hits` cuenta reutilizaciones y `selected_at` registra la última selección. Cada Run incluye `provider` y `model`; las observaciones registran el origen efectivo, incluyendo un respaldo `rules` explícito si el proveedor falla.

El detalle de medidor añade `electrical` (última lectura contra la referencia de esa hora) y `daily_history` (consumo, referencia en las horas observadas, muestras y marca de día completo). No se imputan huecos. Las lecturas incluyen referencias para corriente, voltaje y factor de potencia. Las evidencias añaden `minimum`, `maximum` por lectura y `changed`; el rango del consumo es por intervalo, mientras `observed` es la suma de intervalos señalados.

Dashboard incluye `status_counts`, `consumption_24h`, `baseline_24h` y `variation_24h`, además del total de todo el periodo. El estado de medidor refleja investigaciones activas; un incidente resuelto o descartado permanece consultable en el historial.

Lecturas acepta `from` y `to` en RFC3339 con offset. El intervalo es `[from,to)`. Un filtro no cambia el baseline histórico del medidor. Cada lectura contiene `baseline_kwh` e `is_reference`.

Las métricas de medidores comparan las últimas 24 horas disponibles; dashboard suma todo el dataset. Ambas respuestas incluyen el periodo para evitar comparaciones de escalas distintas.

Ejemplo de acción:

```json
{
  "action": "INVESTIGATE",
  "note": "Se solicitó contraste de lecturas al equipo de operación.",
  "expected_version": 1
}
```

Una versión obsoleta o transición inválida devuelve `409`. Notas entre 5 y 2000 caracteres. No se puede resolver directamente un incidente abierto sin pasar por investigación. Cada acción queda registrada con el usuario autenticado.

Los errores JSON incluyen `status`, `code`, `title` y `request_id`. No incluyen SQL ni claves de proveedores. Login, salud y contrato son públicos; los datos requieren cookie de sesión. El puerto interno de PostgreSQL no se usa desde el navegador.


## Interpretación redactada por IA

Las explicaciones nuevas añaden `headline`, `interpretation[]`, `uncertainty`, `summary_evidence_ids[]` y `narrative_version=analyst-2`. `summary`, `hypotheses`, `actions` y `questions` contienen redacción del proveedor con referencias a evidencia existente. `facts` conserva los hechos del motor. Estas propiedades son opcionales para las observaciones históricas y el respaldo por reglas. Una narrativa inválida devuelve el resumen de respaldo con `source=rules` y `fallback_reason=provider_invalid_narrative`. La validación es estructural y de referencias; no certifica causalidad.

### Comprobaciones guiadas

`POST /anomalies/{id}/actions` acepta en una acción `NOTE` el objeto opcional `verification: {run_id, step_code, status}`. El estado es `PENDING`, `CONFIRMED` o `DISCARDED`. La nota y `expected_version` siguen siendo obligatorios. El servidor comprueba que la recomendación pertenece a una observación persistida de ese incidente. Referencias ajenas o versiones obsoletas devuelven 409; formatos inválidos, 400. La acción se guarda de forma atómica y no cambia el estado general del caso. Cada actualización agrega un evento: conserva los resultados anteriores. La última acción para esa pareja análisis/recomendación determina su estado actual.

### Conversación eléctrica

`POST /ai/chat` recibe `{message, meter_id, history:[{role,content}]}`. `meter_id` vacío significa operación completa; un ID inexistente devuelve 404 antes de admitir un intento pagado. Se permiten hasta seis mensajes, únicamente con roles `user` y `assistant`. Pregunta de 2 a 2.000 bytes, historial máximo 14.000 bytes; exceso o roles no permitidos devuelven 400. El servidor construye contexto acotado con lecturas y hallazgos, sin aceptar contexto de sistema desde el navegador.

Devuelve `{answer,sources:[{id,label,meter_id}],model,cached,created_at}`. Solo se aceptan identificadores de fuente presentes en los datos recuperados. Usa exclusivamente Claude Sonnet. Una pregunta eléctrica general puede no necesitar fuentes de medición. El texto generado no es una comprobación física ni un diagnóstico certificado.

`CHAT_ENABLED` controla únicamente nuevas consultas de chat, por separado de la generación de análisis. Respuestas terminadas se conservan en PostgreSQL y pueden recuperarse aunque se deshabiliten consultas nuevas. El límite diario por actor se renueva a medianoche UTC. Código 429 al agotar cuota, 409 para consulta pendiente, fallida previamente o nuevas llamadas desactivadas, y 502 para errores del proveedor. No hay reintentos automáticos. Una desconexión HTTP posterior a la reserva no cancela la llamada admitida; su resultado se persiste cuando sea posible.
