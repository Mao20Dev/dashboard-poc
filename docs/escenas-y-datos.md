# Contexto de los datos y futura integración 3D

## Qué sabemos del conjunto

Fuente: `data/readings.csv` y `data/events.csv`. Son 4.032 lecturas horarias de 12 medidores del 1 al 14 de septiembre de 2026. El campo de energía es `consumption_kwh`, no producción fotovoltaica.

- Voltaje: mediana 220,46 V; rango completo 201,56–241,23 V. Sin M-112, identificado por problemas de medición, el rango es 213,43–224,97 V.
- Corriente: 38,72–507,43 A.
- Las 1.008 muestras entre 00:00 y 05:00 tienen consumo positivo; suman 29.045,16 kWh.
- M-104 registra literalmente `New production line activated`. M-106 registra mantenimiento programado de doce horas. M-112 registra lecturas intermitentes y saltos eléctricos.

La interpretación más consistente es consumo de cargas de una instalación productiva, medido a voltajes compatibles con baja tensión. Es una inferencia, no una identificación del emplazamiento. El voltaje por sí solo no determina si se mide fase-neutro, fase-fase o mediante transformadores de medida. No hay diagrama unifilar, ubicación física, relación de transformación ni mapeo de equipos.

No hay evidencia para representar estos datos como generación de una granja fotovoltaica. Puede existir generación solar en el emplazamiento, pero el conjunto no lo demuestra. Tampoco demuestra que los doce medidores pertenezcan a una subestación. Para el futuro render se propone una instalación industrial conceptual, identificada como tal hasta contar con información física.

## Escenas integradas

- Login e Inicio: modelo original de bee-poc, con aleteo, flotaci?n y reacci?n suave al cursor. Carga Three.js bajo demanda y libera sus recursos al salir.
- Medidores: `.eq-plant[data-scene="plant"]`. Modelo de una instalaci?n industrial conceptual generado en Blender, con iluminaci?n diurna y nocturna precalculada. Una malla de 116.696 tri?ngulos, sin luces ni sombras en tiempo real. El render ocurre al mover la c?mara, cambiar de tama?o o cambiar de tema; se detiene en reposo y fuera de pantalla.
- Los medidores, pulsos y conexiones son capas DOM/SVG proyectadas sobre la geometr?a. Los estados y valores proceden de la API; las ubicaciones y conexiones son ilustrativas y no representan un circuito el?ctrico comprobado.
- La b?squeda y los filtros son comunes a tabla y planta. Hover, foco de teclado y toque muestran la tarjeta; su enlace abre el detalle del medidor.
- `html[data-theme="dark|light"]` controla ambos temas. La planta interpola sus texturas de iluminaci?n; la abeja adapta sus materiales. Tema claro predeterminado, preferencia persistida como `energyiq.theme`.
- Las animaciones decorativas respetan `prefers-reduced-motion`. Si WebGL falla, la tabla permanece disponible.

Origen del modelo de la abeja: https://github.com/MaoDev20/bee-poc. El archivo se distribuye en `frontend/public/models/bee.glb`. Los activos de la planta est?n en `frontend/public/models/plant/`; las posiciones y vol?menes de oclusi?n est?n en `frontend/src/plant/`.

## Inicio y asistente

El periodo superior describe el conjunto disponible; no simula un filtro de fechas que la API no soporta. El hero se calcula con los hallazgos y su evidencia sin nuevas llamadas generativas. Las prioridades excluyen casos de severidad baja o cerrados y se ordenan por severidad e índice de evidencia. La referencia del gráfico usa `baseline_24h` cuando todos los medidores tienen referencia válida y cobertura horaria completa y alineada con el periodo. No se calcula usando el consumo observado.

Enter o Preguntar desde Inicio abre el panel de Sonnet y envía una sola vez; el historial se conserva por contexto mientras la aplicación permanece abierta. Cerrar el panel no borra la conversación. Los controles de selección Claude/GPT y análisis siguen disponibles en Medidores y en cada detalle. El chat continúa usando Sonnet y los límites existentes.
