# Evidencia I3 — Análisis Consolidado de Capacidad (Rúbrica 20%)

Este directorio contiene la evidencia y trazabilidad del entregable **I3**, correspondiente a la rúbrica **Análisis de capacidad (20%)** de la Entrega 2, desglosada en:
- **Análisis de capacidad: escenario 1 (10%):** Actividad académica concurrente.
- **Análisis de capacidad: escenario 2 (10%):** Carga, procesamiento y consumo multimedia.

El documento principal consolidado se encuentra publicado en:
👉 [`capacity-planning/pruebas_de_carga_entrega2.md`](../../../../capacity-planning/pruebas_de_carga_entrega2.md)

---

## 1. Matriz de Cumplimiento de Criterios de Evaluación

| Criterio de la Rúbrica Oficial | Evidencia en el Repositorio | Sección en Informe | Estado |
| :--- | :--- | :---: | :---: |
| **Definición de cada escenario y niveles de carga** | Recorridos detallados, endpoints exactos, 200 cuentas sintéticas y 3 perfiles de video | §1.1 y §2.1 | ✅ |
| **Herramientas, versiones e infraestructura efectiva** | Apache JMeter 5.6.3 (`justb4/jmeter`), Go Capacity Engine, FFmpeg 6.1, `mooc-web-server` (`e2-small`), `mooc-worker-server` (`e2-highcpu-2`), Cloud SQL PostgreSQL 16.4 | §1.2 y §2.2 | ✅ |
| **Generador ejecutado fuera de las VMs** | Generador operando desde máquina externa local (Windows 11, Ryzen 5 5500U, midiendo ~74 ms RTT a `us-east1`) | §1.2 y §2.2 | ✅ |
| **Condiciones mantenidas fijas** | Concurrencia de workers fija en 2 (`WORKER_CONCURRENCY=2`), pool de BD fijo en 25 (`DB_MAX_OPEN_CONNS`), mezcla 64% L / 36% E fija | §1.1, §1.2, §2.1, §2.2 | ✅ |
| **Resultados numéricos por corrida y variación** | Tabulación de percentiles p50, p95, p99, throughput (req/s y vid/min), errores y métricas de infraestructura | §1.4 y §2.4 | ✅ |
| **Métricas correlacionadas de aplicación e infraestructura** | CPU de Web Server (Ops Agent), conexiones y CPU de Cloud SQL, profundidad y antigüedad de cola Asynq | §1.5 y §2.4 | ✅ |
| **Punto de degradación o máximo probado** | Escenario 1: degradación entre 50 y 100 usuarios (~28 req/s). Escenario 2: saturación de cola en Nivel 3 ($\lambda > \mu$, 12 tareas en ráfaga N4) | §1.6 y §2.8 | ✅ |
| **Cuello de botella primario sustentado con evidencia** | Escenario 1: CPU compartida en `e2-small` + pool de 25 conexiones en Go. Escenario 2: vCPU física en Worker Server (FFmpeg 100% vCPU) | §1.6 y §2.8 | ✅ |
| **Comprobación de envío duplicado sin doble calificación** | Paso 11 con misma `Idempotency-Key` (mismo `submission_id`) y verificación directa en Cloud SQL (0 duplicados) | §1.7 | ✅ |
| **Decisión de reproductor real (TTFF y stalls)** | Sonda de eventos HTML5/MSE (TTFF 48–52 ms, 0 stalls), justificando por qué HTTP puro no prueba renderizado | §2.6 | ✅ |
| **Cadencia de streaming real vs descarga greedy** | Pacing de 6.0 s ($\pm 0.5$ s) contrastado contra descarga masiva sin pausas (> 120 MB/s), aislando riesgos de egreso | §2.5 | ✅ |
| **Drenaje de cola y verificación terminal de consistencia** | Vaciado cronometrado (6.4 s) y consulta SQL de integridad: 57 videos completados, 0 en fallo / DLQ | §2.7 | ✅ |
| **Limitaciones del experimento** | IP única, ancho de banda del generador, limitador de login por seguridad | §1.8 | ✅ |
| **Propuesta de evolución respaldada con mediciones** | Read Replicas en Cloud SQL, PgBouncer, Cloud CDN para HLS y MIG autoescalable de workers | §1.9 y §2.9 | ✅ |
| **Publicación formal en ruta requerida** | [`capacity-planning/pruebas_de_carga_entrega2.md`](../../../../capacity-planning/pruebas_de_carga_entrega2.md) | Documento raíz | ✅ |
| **Política estricta de seguridad y cero secretos** | Sanitización integral de tokens, claves, credenciales y firmas V4 en evidencias | §5 | ✅ |

---

## 2. Mapa de Archivos y Evidencias Primarias

```
Plataforma-MOOC/
├── capacity-planning/
│   ├── pruebas_de_carga_entrega2.md       <-- INFORME CONSOLIDADO OFICIAL (20%)
│   ├── escenario1.md                      <-- Plan acordado Escenario 1
│   └── escenario2.md                      <-- Plan acordado Escenario 2
├── docs/entrega2/evidencias/
│   ├── H1/                                <-- Instrumentación de carga, Ops Agent y JMeter
│   │   ├── ops-agent-config-web.yaml
│   │   ├── ops-agent-config-worker.yaml
│   │   ├── ops-agent-policy.yaml
│   │   ├── smoke_test.jmx
│   │   └── resultados/smoke_test_20260927_164136.jtl
│   ├── H2/                                <-- Scripts, piloto y corridas Escenario 1
│   │   ├── escenario1.jmx                 <-- Plan de pruebas JMeter (14 pasos)
│   │   ├── escenario1_login_rafaga.jmx    <-- Variante ráfaga de login
│   │   ├── verificacion_estado_bd_piloto_nube.txt
│   │   └── resultados/
│   │       ├── nube-piloto_20260927_211339/   <-- Piloto limpio en la nube (126 reqs, 0 errores)
│   │       ├── local-piloto-10-usuarios_.../  <-- Corrida 10 usuarios
│   │       ├── local-login-rafaga_.../        <-- Ráfaga de login (10 OK, 5 429)
│   │       └── local-sin-reinicio_.../        <-- Control negativo (18 409, 9 fallos validación)
│   ├── H4/                                <-- Plan y piloto instrumentado Escenario 2
│   │   └── piloto_etapas_instrumentadas.txt
│   ├── H5/                                <-- Corridas formales, drenaje y cuello botella Escenario 2
│   │   ├── corrida_niveles_escenario2.txt     <-- Log de los 5 niveles (0 a 4)
│   │   ├── drenaje_cola_verificacion.txt      <-- Traza segundo a segundo del drenaje
│   │   ├── resumen_ejecutivo_escenario2.md    <-- Resumen desacoplado por etapa
│   │   └── analisis_cuello_de_botella.md      <-- Sustentación analítica del cuello de botella
│   └── I3/                                <-- ESTE DIRECTORIO (Cierre y consolidación)
│       ├── README.md                          <-- Matriz de trazabilidad y gobernanza
│       └── resumen_ejecutivo_capacidad.md     <-- Resumen ejecutivo de alto nivel
└── scripts/
    ├── run_escenario1.sh                  <-- Runner JMeter Docker para Escenario 1
    ├── capacity_login_tokens.sh           <-- Generador de tokens preautenticados
    ├── capacity_resumen_jtl.py            <-- Clasificador semántico de resultados .jtl
    ├── h2_nube.sh                         <-- Atajos de ejecución contra la nube
    ├── run_capacity_escenario2.sh         <-- Orquestador automatizado Escenario 2
    └── capacity_escenario2/               <-- Motor en Go de Escenario 2
        └── main.go
```

---

## 3. Guía Rápida para Reproducir las Pruebas

### Escenario 1: Actividad Académica Concurrente
```bash
# 1. Obtener tokens de prueba para N usuarios (ej. 25)
BASE_URL=https://34.24.52.111.sslip.io bash scripts/capacity_login_tokens.sh 25

# 2. Resetear estado de las cuentas sintéticas en Cloud SQL
bash scripts/cargar_cuentas_carga_nube.sh scripts/seeds/capacity_reset.sql

# 3. Lanzar la prueba de JMeter en contenedor
BASE_URL=https://34.24.52.111.sslip.io bash scripts/run_escenario1.sh 25 nivel25

# 4. Verificar consistencia terminal directamente en Cloud SQL
bash scripts/cargar_cuentas_carga_nube.sh scripts/seeds/capacity_verificar_estado.sql
```

### Escenario 2: Procesamiento y Streaming Multimedia
```bash
# Ejecutar los 5 niveles de carga, streaming HLS y medición de drenaje de cola
bash ./scripts/run_capacity_escenario2.sh https://34.24.52.111.sslip.io
```

---

## 4. Política Estricta de Seguridad y Auditoría de Secretos

De conformidad con los requisitos del curso y la rúbrica de entrega:
1. **Credenciales y Contraseñas:** Ningún archivo de configuración, evidencia o log incluye contraseñas de PostgreSQL, credenciales SMTP ni contraseñas de usuarios. Las contraseñas de las cuentas sintéticas se inyectan en tiempo de ejecución mediante variables de entorno efímeras.
2. **Tokens de Sesión:** El archivo `tokens.csv` reside exclusivamente en `capacity-planning/datos/`, incluido en `.gitignore`, y no se rastrea en git.
3. **Firmas Criptográficas V4:** Las URLs prefirmadas de subida directa a Cloud Storage registradas en las trazas de depuración tienen sus parámetros de firma (`X-Goog-Signature`, `X-Goog-Credential`, etc.) reemplazados automáticamente por el valor sanitizado `REDACTED`.
4. **Direcciones de Red:** Las direcciones IP privadas documentadas (`10.0.1.4`, `10.0.1.5`, `10.171.240.3`) pertenecen a rangos RFC 1918 de la VPC privada y no son alcanzables desde internet.
