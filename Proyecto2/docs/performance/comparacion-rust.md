# Comparación de rendimiento del servicio Rust

## Configuración de la prueba

- Herramienta: Locust 2.44.4
- Usuarios concurrentes: 50
- Tasa de creación: 5 usuarios por segundo
- Duración: 60 segundos
- Ruta: POST /grpc-202200141
- Gateway: http://8.232.64.139

## Resultados

| Métrica | 1 réplica | 2 réplicas |
|---|---:|---:|
| Solicitudes procesadas | 1242 | 1258 |
| Solicitudes fallidas | 0 | 0 |
| Solicitudes por segundo | 21.10 | 21.28 |
| Tiempo promedio | 237 ms | 224 ms |
| Mediana | 200 ms | 190 ms |
| Percentil 95 | 440 ms | 420 ms |
| Tiempo máximo | 658 ms | 723 ms |

## Análisis

La ejecución con dos réplicas redujo el tiempo promedio de respuesta
de 237 ms a 224 ms, equivalente a una mejora aproximada del 5.49 %.

El percentil 95 se redujo de 440 ms a 420 ms, mientras que el
rendimiento aumentó de 21.10 a 21.28 solicitudes por segundo.

Ambos escenarios finalizaron sin solicitudes fallidas. La diferencia
moderada se debe a que la carga generada no saturó el servicio Rust y
cada usuario de Locust espera entre uno y tres segundos entre peticiones.

El tiempo máximo de 723 ms en el escenario de dos réplicas corresponde
a un valor aislado y no representa el comportamiento general, debido a
que el promedio y los percentiles principales fueron menores.
