# energyIQ · dashboard-poc

[Abrir la demo](https://web-production-acda4.up.railway.app) · Acceso con el botón «Entrar a la demo».

Consola de gestión energética con Go, PostgreSQL y React/TypeScript. Importa los CSV originales, detecta señales y contrasta eventos. Inicio con saludo contextual y abeja interactiva, consumo de las últimas 24 h, tres prioridades ordenadas por severidad y evidencia, franja de tres indicadores sin duplicar el consumo reciente, gráfico diario con referencia real y cuadrícula de medidores. El campo de Inicio envía la pregunta a Sonnet y abre la conversación lateral, conservando el historial al cerrarla. Un acceso flotante aparece cuando los accesos de la página quedan fuera de pantalla; en Perfil se sitúa sobre la barra de guardado para no taparla. Las vistas y tarjetas tienen entradas breves, y los skeletons aparecen solo durante la carga inicial de datos. Todas estas animaciones respetan movimiento reducido. La tabla de medidores admite ordenamiento desde sus encabezados. Inicio incorpora una tarjeta de análisis y la comparación de todos los hallazgos. Los controles por proveedor están disponibles en Inicio, Medidores y cada detalle; muestran estados reales y resumen final. Medidores abre directamente la tabla o vista conceptual de planta, con comparaciones debajo. Cada medidor abre con un veredicto y consumo reciente, seguido de cronología, señales comparadas, recomendaciones, cuatro gráficas con cursor horario sincronizado, resumen diario y evidencia desplegable. Se conservan las interpretaciones y la bitácora del proveedor seleccionado. La potencia activa mostrada es media horaria (energía del intervalo de 1 h / 1 h), no instantánea. Las acciones de seguimiento usan la API existente con versión optimista y nota obligatoria; registrar nuevos eventos operativos sigue pendiente. Tema claro por defecto, elección claro/oscuro persistente y transiciones que respetan movimiento reducido.

## Ejecutar

Requisito: Docker Desktop iniciado, con contenedores Linux.

```sh
docker compose up --build -d --wait
```

- Consola: http://localhost:5173
- API y documentación: http://localhost:8080/api/docs
- Contrato OpenAPI: http://localhost:8080/api/openapi.json
- Salud: http://localhost:8080/healthz
- Usuario local: `uribe@energyiq.test`
- Contraseña local: `EnergyDemo2026!`

Los puertos se publican solo en localhost. La base de datos y el historial sobreviven a `docker compose down`. No elimines el volumen si quieres conservar las investigaciones.

Para ver logs: `docker compose logs --tail=100 api`. Para detener: `docker compose down`.

## Despliegue en Railway

El proyecto usa tres servicios: `web` (React servido por Nginx), `api` (Go) y `Postgres`. Solo `web` necesita dominio público; `/api/` se conecta al backend por la red privada y conserva un único origen para las sesiones.

- `api`: contexto raíz del repositorio, Dockerfile `backend/Dockerfile`.
- `web`: contexto raíz del repositorio, Dockerfile `deploy/web.Dockerfile`.
- Ambos escuchan en el puerto `8080`; comprobación de salud `/healthz`, timeout de 120 s y una réplica. La ruta de Dockerfile y la comprobación de salud se configuran en Settings de cada servicio.
- En `api`, `DATABASE_URL` referencia `${{Postgres.DATABASE_URL}}`, `ADDR=[::]:8080`, `DATA_DIR=/app/data`, `PLANT_TZ=America/Bogota` y `COOKIE_SECURE=true`.
- En `web`, `API_UPSTREAM` apunta al dominio privado del servicio API y puerto 8080.
- Las claves `ANTHROPIC_API_KEY` y `OPENAI_API_KEY` se configuran exclusivamente en **Railway → proyecto → api → Variables**. Nunca en `web`, GitHub ni variables `VITE_*`. No pegar todo el `.env` local: las conexiones y cookies de producción ya tienen su propia configuración.
- Conserva `AI_ALLOW_NEW_REQUESTS=false`. Para permitir preguntas nuevas al asistente usa `CHAT_ENABLED=true` y `CHAT_DAILY_LIMIT=20`.
- Mantén los modelos iguales a los usados por los análisis guardados: `ANTHROPIC_MODEL=claude-sonnet-5-5` y `OPENAI_MODEL=gpt-6-luna`.

Los CSV se importan al arrancar. Los análisis, respuestas y bitácoras viven en PostgreSQL: no se reconstruyen al subir el repositorio. Para conservarlos se restaura una copia privada de la base, excluyendo las sesiones activas. Los respaldos y `.env` están excluidos de Git y del contexto de despliegue.

La planta es geometría 3D con iluminación precalculada en Blender, cargada al abrir su vista. El render se detiene en reposo; pulsos y conexiones se dibujan como capas ligeras. No requiere Blender ni GPU en el servidor: el navegador del visitante dibuja la escena.

Las pruebas de GitHub preparan una base efímera con lecturas reales y narrativas sintéticas identificadas como fixtures. `backend/cmd/ci-seed` exige `CI=true` y una base sin análisis previos; no se incluye en la imagen de producción ni llama a proveedores de IA.

## IA generativa opcional

La consola permite elegir **Claude Sonnet**, **GPT** o **Reglas verificables** para cada ejecución. `/api/v1/ai/providers` informa cuáles tienen credenciales configuradas (no comprueba saldo ni acceso al modelo). El servidor registra proveedor y modelo al encolar; dos proveedores sobre los mismos datos producen ejecuciones diferentes. Las credenciales permanecen exclusivamente en el backend.

Para GPT, crea una clave en [OpenAI Platform](https://platform.openai.com/api-keys), configura la facturación/créditos de tu proyecto y guarda en `.env`:

```dotenv
OPENAI_API_KEY=tu_clave_secreta
OPENAI_MODEL=gpt-6-luna
```

Después: `docker compose up -d --force-recreate api`. Conserva también las variables de Anthropic para disponer de ambos proveedores. Referencias: [inicio rápido oficial](https://developers.openai.com/api/docs/quickstart), [GPT-6 Luna](https://developers.openai.com/api/docs/models/gpt-6-luna) y [salidas estructuradas](https://developers.openai.com/api/docs/guides/structured-outputs). El adaptador OpenAI usa Responses API con `store:false`, esquema estricto y validación semántica local; no envía la clave al navegador. Tener una clave configurada no garantiza que la cuenta tenga acceso o crédito.

Si no existe `.env`, copia `.env.example` a `.env`. Configura `ANTHROPIC_API_KEY` con una clave de [Claude Console](https://platform.claude.com/settings/keys). Después ejecuta `docker compose up -d --force-recreate api`. No pongas claves en variables del frontend ni las subas a Git. Si tu clave accede a varios workspaces, configura también `ANTHROPIC_WORKSPACE_ID`; para una clave de un solo workspace puede quedar vacío.

El modelo predeterminado es `claude-sonnet-5-5`, configurable mediante entorno. Referencia: [documentación oficial del modelo](https://platform.claude.com/docs/en/models/sonnet-5-5/overview).

Sin claves, el análisis funciona con `rules`. Con clave, Claude o GPT redactan una lectura del caso, relacionan las señales eléctricas, plantean hipótesis y proponen comprobaciones concretas. **El motor calcula las cifras y la clasificación; la IA escribe la interpretación.** Cada párrafo enlaza evidencia existente. La narrativa es cualitativa: las cifras se muestran en componentes alimentados directamente por los hechos tipados. La validación comprueba estructura, referencias, longitudes y ausencia de dígitos en la prosa; no demuestra causalidad ni garantiza que toda inferencia del modelo sea correcta. El usuario ve las incertidumbres del caso y conserva acceso a todas las mediciones. El chat eléctrico con Sonnet se describe más abajo y usa un contrato separado: permite cifras en texto y verifica los identificadores de las fuentes, sin certificar causalidad.

Una respuesta inválida, timeout o error del proveedor produce un plan de respaldo explícito. Las APIs reales requieren claves para su validación en vivo; las pruebas automatizadas usan un servidor HTTP simulado y cubren éxito y fallos.

Ambos adaptadores generan **una sola petición para toda la planta**: envían contexto compacto de los medidores y la evidencia calculada de todos los hallazgos, sin las series crudas. El lote tiene hasta 12 hallazgos y 128 KiB de entrada; la salida tiene un máximo de 3.000 tokens por hallazgo, con techo de 12.000 por petición y 60 segundos. Estos topes acotan cada solicitud; no son un presupuesto monetario. No hay reintentos de transporte ni correcciones pagas de formato.

### Caché y control de costos

- `AI_ALLOW_NEW_REQUESTS=false` es el valor predeterminado. Permite leer análisis guardados y ejecutar reglas locales; bloquea nuevas llamadas a Claude y GPT tanto en la API como en el worker. Con una base nueva, usa reglas o habilita explícitamente `true` y reinicia la API para generar lo que falte.
- La caché vive en PostgreSQL, sin vencimiento: hash de datos y configuración del motor + proveedor + modelo + versión del contrato narrativo. Volver de Claude a GPT y a Claude recupera cada respuesta original. La migración 3 incorpora los últimos análisis históricos completos y válidos sin volver a llamar al proveedor.
- Para Claude y GPT, el selector y Analizar con IA envían `cached_only:true`: siempre recuperan el análisis existente y nunca generan nuevas llamadas pagas. Para `rules` envían `cached_only:false`: reutilizan la caché si existe y, si falta, permiten procesar localmente sin proveedor externo. No existe un botón de regeneración que ignore la caché.
- Una transacción protege la admisión simultánea. Se reserva de forma persistente un único intento de proveedor antes de salir a internet. Un reinicio tras esa reserva no repite una solicitud cuyo resultado se desconoce; puede perderse esa respuesta, pero no se arriesga un segundo cargo automático.
- También se conserva un resultado de respaldo o un intento fallido; repetir el botón no lo vuelve a cobrar. Para recuperar un fallo hace falta una decisión administrativa explícita, fuera de la UI. No borres el volumen ni las tablas si quieres conservar esta protección.
- Cambiar datos, modelo o versión crea otra identidad y podría requerir una nueva petición si el interruptor está habilitado. La selección de análisis es compartida por la planta en esta demo de un usuario; las notas y estados de investigación siguen siendo actuales, no copias antiguas de caché.

En OpenAI configura también un límite mensual de proyecto en **Project settings → Limits → Spend → Edit spend limit → Enforce a hard limit**. Una alerta de gasto por sí sola no bloquea peticiones. La aplicación del límite puede tener un pequeño retraso: [documentación oficial de límites de gasto](https://developers.openai.com/api/docs/guides/spend-limits). El interruptor local protege esta aplicación; no controla otros programas que usen la misma clave.

La migración 2 conserva las observaciones históricas y añade proveedor/modelo a las ejecuciones. Las anteriores quedan identificadas como `legacy` a nivel de ejecución; sus explicaciones conservan el origen real. Las variables `GEMINI_API_KEY` y `GEMINI_MODEL` ya no se utilizan.

## Recorrido de demostración

1. Pulsa **Entrar a la demo**: el formulario completa las credenciales públicas de demostración y realiza el inicio de sesión normal. También puedes acceder manualmente.
2. **Inicio** reúne consumo del periodo, últimas 24 horas, hallazgos prioritarios, medidores sin alerta, gráfico diario y resumen.
3. Pulsa un medidor para abrir su página completa con explicación, unidades, referencias, hipótesis, recomendaciones y gráficos. El selector alterna energía, corriente, voltaje y factor de potencia. Los medidores normales se identifican como evaluación del motor, sin narrativa individual de IA.
4. **Medidores** comparte sidebar y escala tipográfica con Inicio. Incluye búsqueda, filtros por estado con conteos y participación del consumo en la tabla: el porcentaje usa el total de los medidores, incluso al filtrar; las barras se comparan con el mayor consumo. Tabla y vista de planta conservan los filtros. Los puntos de la planta muestran una ficha al pasar el cursor, enfocar con teclado o tocar; desde ella se abre el análisis completo. Las posiciones son ilustrativas, sin inferir conexiones eléctricas. Debajo, tres tarjetas resumen desviación, calidad y contexto de los datos.
5. Inicio, Medidores y el detalle permiten seleccionar proveedor y analizar. Claude/GPT recuperan la versión guardada; reglas puede iniciar un trabajo local si aún no existe. Se consulta la ejecución por ID mientras está en cola o en curso, mostrando su etapa real; al completar se actualizan los datos y se informa cuántas anomalías y prioridades hay. Los fallos se muestran sin intentar regeneraciones pagas. El acceso a Sonnet conserva el contexto del medidor. Las notas históricas y acciones siguen disponibles.
6. El sidebar se contrae con su flecha superior. **Mi perfil** conserva nombre, foto y preferencias en este navegador. El tema claro/oscuro también se conserva al salir y volver a entrar.
7. El login muestra la abeja 3D original de bee-poc a la izquierda y el acceso sobre un panel verde a la derecha, con movimiento, reacción al cursor y materiales adaptados al tema. Carga Three.js bajo demanda, respeta movimiento reducido y ofrece una ilustración de respaldo. En móvil la abeja aparece encima del acceso a la demo.

Las pruebas de navegador cubren acceso demo, tema persistente, movimiento reducido, detalle de los doce medidores, filtros, vista conceptual, perfil móvil y selección de análisis guardados. El chat de esas pruebas se simula; no generan respuestas de pago ni escriben notas en la demo.

Ver [contexto de los datos y espacios 3D](docs/escenas-y-datos.md).

## Chat eléctrico con Sonnet

El botón **Pregúntale a Sonnet** está disponible en toda la aplicación. Al entrar a un medidor, usa ese contexto y una conversación independiente. Las respuestas enlazan registros existentes. La sesión de conversación vive en memoria del navegador; las respuestas se cachean en PostgreSQL.

Las sugerencias y los botones de consulta envían la pregunta con un clic, con bloqueo inmediato de envíos concurrentes. Las respuestas sobre prioridades muestran hasta dos medidores abiertos de severidad alta, ordenados por el puntaje descendente del motor; los números 1 y 2 indican su posición, no el puntaje interno. Otras respuestas enlazan los medidores citados y existentes en el catálogo. Las tarjetas numéricas usan datos tipados del API, diferenciados del texto de Sonnet. Los encabezados explícitos de hechos, recomendaciones y límites de respuestas guardadas se presentan en bloques, sin reescribir su contenido ni regenerarlo. La prosa no reconocida se conserva; las secciones extensas se pueden ampliar. La animación de escritura es local y respeta movimiento reducido. Manrope se distribuye localmente bajo SIL OFL (licencia en `frontend/public/fonts/`).

- `CHAT_ENABLED=true` habilita consultas nuevas de chat con las credenciales Anthropic existentes. Su valor predeterminado es `false`; la instalación local fue activada a petición del usuario. No modifica `AI_ALLOW_NEW_REQUESTS=false`, que sigue bloqueando regeneraciones pagas de análisis.
- `CHAT_DAILY_LIMIT=20` admite hasta veinte intentos nuevos por cuenta y día UTC. Los fallidos también cuentan. No es un límite monetario de la cuenta del proveedor.
- Cada consulta tiene como máximo 2.000 bytes de pregunta, seis mensajes recientes, 80 KiB de contexto construido por el servidor y 1.000 tokens de salida. El límite de contexto incluye historial, resúmenes y evidencias.
- La caché incorpora pregunta, conversación, hechos recuperados, actor, modelo y versión del prompt. No vence por tiempo; cambia si cambia ese contexto. Consultar una respuesta idéntica guardada no llama al modelo.
- La reserva transaccional antecede a la llamada, admite una consulta simultánea por cuenta y sobrevive a reinicios. Un intento fallido o incierto nunca se repite automáticamente. Después de la admisión, desconectar el navegador no cancela el trabajo: se intenta guardar la respuesta dentro del plazo de 45 segundos.
- Recupera lecturas resumidas, referencias, eventos y hallazgos mediante código y SQL fijos; no ejecuta SQL propuesto por la IA. No es una biblioteca de manuales ni incorpora búsquedas externas. Las fuentes validadas prueban su existencia, no que cada afirmación del modelo sea correcta.
- Ocho medidores no tienen hallazgos ni narrativa individual en los lotes históricos; se muestra esa cobertura con honestidad y se permite pedir a Sonnet una explicación de sus lecturas.

La API usa [salidas estructuradas de Anthropic](https://platform.claude.com/docs/en/build-with-claude/structured-outputs). Las pruebas de proveedor usan HTTP simulado; las de caché y cuota se ejecutan en esquemas PostgreSQL aislados.

## Desarrollo

Go 1.26 o superior (validado con 1.27.2), Node 22 y PostgreSQL 17.

```sh
docker compose up -d db
cd backend
go run ./cmd/api
```

En otra terminal:

```sh
cd frontend
npm ci
npm run dev
```

Detén los contenedores `api` y `web` antes de iniciar sus versiones de desarrollo para liberar los puertos. El backend carga `.env` desde la raíz o su directorio. Las credenciales del usuario demo se crean solo si no existe: cambiar `DEMO_PASSWORD` no reemplaza la contraseña de un usuario existente.

## Pruebas

Desde `backend`, con PostgreSQL accesible:

```powershell
$env:TEST_DATABASE_URL='postgres://energy:energy-local-only@localhost:54329/energy?sslmode=disable'
go vet ./...
go test ./... -count=1 -cover
```

En Bash, usa `export TEST_DATABASE_URL='postgres://energy:energy-local-only@localhost:54329/energy?sslmode=disable'` antes de los comandos Go.

Las pruebas de integración crean un esquema PostgreSQL exclusivo por prueba y lo eliminan al terminar. No modifican las tablas de la demo. Sin `TEST_DATABASE_URL` se omiten explícitamente; no cuentan como integración validada.

Con el sistema completo ejecutándose, desde `frontend`:

```sh
npm ci
npx playwright install chromium
npm run build
npm run test:e2e
```

Los tests de navegador usan acciones simuladas y reutilizan análisis locales. No llaman al proveedor generativo. Mantén `AI_ALLOW_NEW_REQUESTS=false` para bloquear llamadas de pago.

Cobertura funcional: casos del dataset, IDs renombrados, entrada reordenada, eventos desplazados/ausentes, historia insuficiente, baseline sin datos futuros, importación idempotente y atómica, trabajo único ante concurrencia, recuperación de lease, protección ante worker obsoleto, historial inmutable, concurrencia de acciones, sesión expirada/revocada y respuestas inválidas de IA.

## Estructura

```text
backend/cmd/api             Composición y arranque
backend/internal/domain     Contratos de datos y evidencia
backend/internal/ingest     Lectura y validación de CSV
backend/internal/engine     Motor analítico puro
backend/internal/explain    Plan de investigación y adaptador Claude
backend/internal/store      SQL parametrizado, snapshots, sesiones y auditoría
backend/internal/worker     Ejecución con lease y límite temporal
backend/internal/api        API REST y OpenAPI
frontend                    Consola funcional React
data                        Lecturas y eventos de la demo
docs                        Decisiones, metodología y alcance
```

## Alcance

Es una demo de gestión energética para una organización, disponible en local y mediante HTTPS. No incluye gestión multiempresa, recuperación de contraseña, tarifas, control de equipos ni entrenamiento de un modelo predictivo.

Consulta [arquitectura](docs/arquitectura.md), [metodología](docs/metodologia.md) y [contrato de API](docs/api.md).

El sidebar compartido tiene 248 px, logo de 32 px, tarjeta de operación, navegación de texto y perfil inferior con separador. Conserva el control de contracción (76 px), el menú móvil y el mismo componente en todas las rutas.

Mi perfil sigue el layout del diseño: hero de identidad con vista previa, resumen de operación y tarjetas de Datos personales, Visualización y Sesión. El nombre, cargo, foto y detalle técnico usan guardado explícito con opción de descartar; claro/oscuro se aplica inmediatamente y permanece sincronizado con el encabezado. No se muestran ajustes de notificaciones o asistente sin servicio conectado.

Inicio muestra todos los hallazgos con tipo, severidad, confianza y acción, incluido el falso positivo. La confianza agregada es el índice medio de evidencia de la API, no probabilidad de avería. La tabla de Medidores permite alternar ascendente/descendente por ID, estado (prioridad), consumo, referencia, variación y participación; conserva filtros y porcentajes sobre el total de la instalación. Las referencias insuficientes quedan al final de su ordenamiento.
