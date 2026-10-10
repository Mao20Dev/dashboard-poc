# Arquitectura y decisiones

## Un backend desplegable

Monolito modular en Go. El motor analítico recibe datos y devuelve un reporte; no importa HTTP, PostgreSQL ni un SDK de IA. Los adaptadores de persistencia y explicación se conectan en el arranque. React consume una API REST detrás de un proxy del mismo origen.

Se usa `pgx` con SQL explícito y parametrizado. En este primer alcance no incorporamos `sqlc`: las consultas se mantienen en el adaptador y se prueban contra PostgreSQL real. No hay Redis, microservicio Python ni orquestador de agentes.

## Datos y ejecución

Los CSV se validan completos antes de importarlos en una transacción. Lecturas idénticas son idempotentes; lecturas diferentes con el mismo medidor y hora abortan la importación. Las columnas numéricas deben ser finitas y respetar rangos básicos. Una lectura sospechosa dentro de esos rangos se analiza, no se descarta por su etiqueta `OK`.

Al iniciar un análisis se toma una lectura consistente de datos y eventos mediante `REPEATABLE READ`. El trabajo conserva ese input como JSONB. Su SHA-256 incorpora datos, eventos, zona horaria, configuración y versión del motor. La caché persistente usa ese hash, proveedor, modelo y versión narrativa; no vence por tiempo. Un evento nuevo cambia el hash y nunca reutiliza una conclusión anterior.

Un índice único parcial y un UPSERT atómico deduplican trabajos activos con la misma entrada, proveedor y modelo. El proveedor y el modelo quedan congelados al encolar; el worker resuelve únicamente sus credenciales vigentes. La tabla analysis_cache y un bloqueo transaccional deduplican también trabajos terminados. Seleccionar un resultado actualiza selected_at, sin recrear observaciones. El registro de proveedores compartido entre API y worker permite elegir Claude, GPT o reglas sin exponer credenciales.

El worker reclama trabajo con `FOR UPDATE SKIP LOCKED`, una concesión de 120 segundos y un token aleatorio. La ejecución tiene un límite de 90 segundos, menor que el lease. Al persistir, una transacción comprueba que el token sigue vigente: un worker obsoleto no puede publicar resultados. Tras una caída, un lease vencido se recupera hasta tres intentos. La recuperación puede tardar hasta dos minutos. No se promete ejecución exactamente una vez; se evita la publicación duplicada/obsoleta.

El pipeline reporta etapas reales: lectura, detección, explicación y persistencia. Las operaciones analíticas pequeñas se agrupan en detección; no hay esperas artificiales para animar el progreso.

## Incidentes, observaciones y acciones

Un incidente se identifica por medidor, familia de señal e inicio. Su estado operativo vive separado de los resultados analíticos. Cada ejecución agrega una observación inmutable con evidencia y explicación. Los reanálisis no sobrescriben observaciones ni reinician el estado.

El identificador puede cambiar si otra versión del detector cambia el comienzo del episodio. La conciliación de incidentes que se dividen/fusionan queda fuera de este MVP; la versión del motor y los snapshots permiten investigarlo.

Las acciones bloquean el incidente y requieren `expected_version`. Dos operadores que partan de la misma versión no pueden sobrescribirse silenciosamente. Se registra autor, nota, acción y transición en la misma transacción.

- `OPEN` → `INVESTIGATING` → `RESOLVED`.
- `OPEN` o `INVESTIGATING` → `DISMISSED`.
- `RESOLVED` o `DISMISSED` → `OPEN` mediante reapertura.
- Una nota puede añadirse en cualquier estado y también incrementa la versión.

El dashboard usa hallazgos del último análisis completado seleccionado y muestra por separado el último intento. Un intento fallido no borra resultados correctos previos. Las métricas de estado excluyen incidentes resueltos/descartados y paradas explicadas de la cola de atención.

## Límite de confianza del modelo

La integración generativa devuelve un objeto estructurado con titular, resumen, interpretación de las señales, hipótesis, acciones, preguntas e incertidumbre. El modelo redacta estos textos libremente, con referencias de evidencia por párrafo. El catálogo de códigos queda exclusivamente como respaldo determinista; no condiciona la redacción generativa. Los hechos calculados y la clasificación permanecen inmutables.

La prosa cualitativa no admite dígitos, porcentajes literales, controles ni marcado. Los valores exactos se muestran junto a las referencias desde hechos tipados, sin copiarlos de la prosa del modelo. La validación de estructura y referencias no comprueba por sí sola la verdad de una inferencia, ni detecta todas las afirmaciones cuantitativas escritas con palabras. El prompt exige separar hechos, posibilidades e incertidumbres; la interfaz atribuye la interpretación al proveedor y deja las causas pendientes de comprobación.

La procedencia `rules`/`claude`/`openai`, el modelo, `narrative_version` y la causa genérica de respaldo quedan en la observación. Un resultado inválido usa un resumen de reglas identificado explícitamente, sin simular una narrativa de IA. Las observaciones anteriores no se reescriben. OpenAI usa Responses con esquema estricto y almacenamiento desactivado; Claude usa Messages con salida estructurada. Ambos comparten prompt, esquema y validación. Se hace una única petición por planta, con contexto compacto de todos los medidores y evidencia por hallazgo. Máximo 12.000 tokens de salida y 60 segundos; no hay correcciones ni reintentos pagados. El worker reserva de forma durable provider_request_started antes del envío; tras una caída no repite esa solicitud aunque desconozca su resultado. AI_ALLOW_NEW_REQUESTS=false bloquea nuevas solicitudes por defecto. El presupuesto global sigue siendo de noventa segundos. Los errores no incluyen claves ni cuerpos completos del proveedor.

Las credenciales permanecen en el servidor. Los eventos son contexto no confiable y nunca autorizan acciones. Ninguna respuesta de IA modifica estados: las acciones ocurren mediante una petición explícita del usuario autenticado.

## Acceso y operación

Sesiones opacas revocables, token aleatorio de 256 bits, hash SHA-256 en PostgreSQL, duración de 12 horas y cookie HttpOnly/SameSite Strict. Contraseñas bcrypt. Las escrituras de navegador comprueban el origen. El login limita intentos por IP dentro del proceso; para varias réplicas se requiere un límite compartido o en el proxy. Se evita depender de cabeceras de IP reenviadas sin configurar proxies confiables.

Las migraciones están embebidas y se aplican dentro de una transacción con bloqueo. La versión 2 añade proveedor/modelo y sustituye el índice de deduplicación conservando datos y observaciones anteriores. La siguiente evolución debe añadirse como migración versionada. Los logs JSON usan identificadores de petición/ejecución. No hay plataforma externa de observabilidad en este alcance.
