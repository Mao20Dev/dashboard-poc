# Metodología analítica

## Supuestos explícitos

- `consumption_kwh` representa energía de un intervalo horario; no un contador acumulativo.
- Los timestamps sin offset del CSV se interpretan como `America/Bogota`.
- Los primeros siete días por medidor constituyen la referencia inicial. Se supone que representan operación habitual; esta hipótesis no puede demostrarse solo con catorce días.
- Se necesitan al menos tres muestras de referencia por hora. Con menos historia se devuelve `INSUFFICIENT_HISTORY`, no un estado saludable.
- El dataset no identifica fases, equipos, calibres, tarifas ni causas físicas. Los controles eléctricos son comparaciones de comportamiento, no certificaciones de cumplimiento normativo.

## Referencia y detección

Por medidor y hora se calcula mediana de consumo, voltaje, corriente, factor de potencia y relación consumo/(V·I·FP/1000). La dispersión del consumo se estima con MAD. Solo se evalúan observaciones posteriores al final de la referencia; los datos futuros no alteran ese perfil.

Consumo: desviación relativa de al menos 25% y distancia de al menos cuatro escalas robustas, donde la escala es el máximo de `1.4826*MAD`, `0.5 kWh` y `5% del consumo esperado`. Se agrupan horas consecutivas con la misma dirección. Los cambios de al menos tres horas forman episodios; los segmentos más breves requieren un pico de al menos 80% y se presentan como desviación puntual de prioridad media.

Calidad: si el consumo permanece dentro del 20% de su referencia, se buscan desviaciones de voltaje superiores a 12 V, factor de potencia superiores a 0.15 o cambios en la relación eléctrica superiores a 45%. También se señala energía positiva con variables eléctricas no positivas. Se agrupan señales separadas por hasta seis horas y se exigen dos lecturas afectadas. Estas reglas señalan incoherencias para validación; no prueban un sensor averiado.

Una señal de calidad se separa de una desviación de consumo para evitar presentar la misma lectura como dos diagnósticos incompatibles. Los intervalos irregulares generan advertencias y nunca se rellenan con ceros. El análisis aún supone que cada registro disponible representa una hora; datos con distinta frecuencia requieren otra configuración/modelo de intervalos.

## Contexto de eventos

Se buscan eventos del mismo medidor, próximos al episodio. Un cambio operativo anterior al comienzo y dentro de 24 horas puede ser compatible con un aumento o una disminución: se clasifica como `EXPLAINABLE_ANOMALY`, dejando explícita la necesidad de confirmar dirección, alcance y causalidad.

Una parada programada solo explica una caída contenida dentro de su ventana declarada. Para clasificarla como `FALSE_POSITIVE` se requieren además tres lecturas horarias consecutivas inmediatamente posteriores al episodio, dentro del 20% de su referencia. Sin recuperación demostrada se mantiene como cambio explicable de prioridad media. Un hueco o el final del dataset no prueban recuperación. La duración se extrae únicamente del patrón numérico “N hours/horas” del registro; sin duración suficiente no se descarta automáticamente. Un evento `UNKNOWN` no explica el consumo. `DATA_QUALITY` aporta contexto, pero no fuerza la detección: se requieren señales en las lecturas.

## Evidencia y puntuación

El consumo se suma sobre los intervalos seleccionados; el esperado se suma sobre las mismas horas del perfil. Las variables eléctricas comparan medianas sobre los mismos intervalos. Se conservan los timestamps exactos de las muestras. En problemas de calidad, la ventana puede contener horas no seleccionadas: la agregación no representa necesariamente toda la ventana.

En el motor 1.1.0, los cambios reales de al menos 50%, con al menos tres muestras y sin recuperación confirmada, son de severidad alta (prioridad 80 más un componente de magnitud limitado a 15). Los cambios moderados, picos aislados y episodios recuperados son de severidad media (55). Calidad tiene prioridad 70 y severidad alta desde doce muestras afectadas, media por debajo. Los cambios explicables tienen prioridad 45 y las paradas explicadas con recuperación, 5. Son reglas de negocio versionadas, no una prioridad inferida libremente por un LLM.

La confianza es un índice heurístico de evidencia, con componentes visibles:

| Componente | Peso |
|---|---:|
| Disponibilidad histórica por hora respecto a siete muestras | 30% |
| Cantidad de lecturas afectadas, saturada a doce | 25% |
| Magnitud relativa del cambio, o señal de calidad detectada | 25% |
| Corroboración eléctrica | 20% |

La corroboración del consumo exige que la corriente cambie en la misma dirección: se usa el cociente de cambios relativos limitado a [0,1]; una dirección opuesta puntúa cero. Para calidad, ambas señales de voltaje y factor de potencia aportan corroboración completa; una sola, parcial. La suma ponderada se limita a 0.95. Este techo evita mostrar certeza, pero no calibra el índice. No es una probabilidad de avería, exactitud de clasificación ni confianza calibrada. No hay etiquetas suficientes para justificar esas interpretaciones. Los pesos y umbrales deben calibrarse con más medidores e incidentes reales antes de uso operacional a escala.

## Regresión del dataset

Con la referencia horaria de los primeros siete días, M-109 tiene 5.380,80 kWh observados frente a 2.555,85 kWh esperados en 58 intervalos: +110,53%. Los porcentajes ilustrativos del PDF no se usan como constantes.

Las pruebas verifican los cuatro patrones del conjunto de datos, además de IDs nuevos, orden distinto, eventos ausentes/desplazados y señales sintéticas. Ningún ID concreto participa en las reglas productivas.

La importación y el análisis no entrenan un predictor. El motor combina reglas y estadística robusta; el modelo generativo, cuando está configurado, organiza la investigación sobre esa evidencia.
