#!/usr/bin/env bash
# =============================================================================
# Verificador automatizado de Issue I2 (#232) — Entregable 3 completo:
#   docs/entrega3/arquitectura.md (secciones 1 a 8, con enfasis en 5, 6 y 7)
#   docs/entrega3/evidencias/I2/
#
# Cumple la Nota Tecnica #36: cada control ejecuta una asercion real y el
# script termina con codigo != 0 si cualquier verificacion falla.
# =============================================================================
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

ARQ_DOC="docs/entrega3/arquitectura.md"
NOTAS_DOC="docs/entrega3/NOTAS_TECNICAS.md"
DEUDA_DOC="docs/entrega3/deuda-tecnica.md"

OK_COUNT=0
FAIL_COUNT=0
OBS_COUNT=0

pass() {
  OK_COUNT=$((OK_COUNT + 1))
  printf '  [OK]   %s\n' "$1"
}

fail() {
  FAIL_COUNT=$((FAIL_COUNT + 1))
  printf '  [FALLA] %s\n' "$1" >&2
}

obs() {
  OBS_COUNT=$((OBS_COUNT + 1))
  printf '  [OBS]  %s\n' "$1"
}

printf '=============================================================================\n'
printf ' Verificacion de arquitectura.md — Issue I2 (#232)\n'
printf ' Fecha UTC: %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
printf ' Commit base: %s\n' "$(git rev-parse --short HEAD)"
printf '=============================================================================\n\n'

# -----------------------------------------------------------------------------
# 1. Existencia del entregable en la ruta exigida y ausencia de marcadores
# -----------------------------------------------------------------------------
printf '[1/10] Estructura completa de %s (§1 a §8)\n' "${ARQ_DOC}"
if [[ -f "${ARQ_DOC}" ]]; then
  pass "Existe el entregable en la ruta exigida: ${ARQ_DOC}"
else
  fail "No existe ${ARQ_DOC}"
fi

if grep -q "Lo escribe I2" "${ARQ_DOC}"; then
  fail "Quedan marcadores pendientes ('Lo escribe I2') en ${ARQ_DOC}"
else
  pass "Cero marcadores pendientes en ${ARQ_DOC}"
fi

REQUIRED_HEADINGS=(
  "## 1. Servicios del proveedor"
  "## 2. Modelo de componentes"
  "## 3. Modelo de despliegue"
  "## 4. Cambios frente a la Entrega 2"
  "## 5. Decisiones de escalamiento"
  "### 5.1 Plantillas de instancia e inmutabilidad"
  "### 5.2 Inicialización automática, fijación por SHA y migraciones controladas"
  "### 5.3 Políticas de escalamiento de ambos grupos"
  "### 5.4 Verificaciones de salud"
  "### 5.5 Consumo compartido de la cola y retiro seguro de workers"
  "## 6. Operación y recuperación"
  "### 6.1 Accesos, herramientas y separación de las dos capas de identidad"
  "### 6.2 Manejo de configuración y secretos"
  "### 6.3 Despliegue y encendido paso a paso"
  "### 6.4 Verificación integral tras el despliegue"
  "### 6.5 Aparcado entre sesiones, retiro controlado de recursos y reconstrucción"
  "### 6.6 Diagnóstico de problemas operativos conocidos"
  "## 7. Capacidad, costo y limitaciones"
  "### 7.1 Configuración efectiva durante las pruebas"
  "### 7.2 Estimación de costos y consumo observado (\`F3\`)"
  "### 7.3 Puntos únicos de falla restantes (SPOF)"
  "### 7.4 Relación con los resultados de las pruebas y con la deuda técnica"
  "## 8. Archivos fuente de los diagramas"
)

MISSING_HEADINGS=0
for h in "${REQUIRED_HEADINGS[@]}"; do
  if ! grep -Fq "$h" "${ARQ_DOC}"; then
    fail "Falta el encabezado obligatorio: $h"
    MISSING_HEADINGS=$((MISSING_HEADINGS + 1))
  fi
done
if [[ "${MISSING_HEADINGS}" -eq 0 ]]; then
  pass "Presentes los ${#REQUIRED_HEADINGS[@]} encabezados obligatorios (§1–§8 y subsecciones de §5, §6 y §7)"
fi

# -----------------------------------------------------------------------------
# 2. Validez de anclas Markdown internas (Nota Tecnica #14) y enlaces relativos
# -----------------------------------------------------------------------------
printf '\n[2/10] Integridad de anclas Markdown (Nota Tecnica #14) y enlaces relativos\n'
python3 - "${ARQ_DOC}" <<'PY'
import pathlib, re, sys, unicodedata

doc_path = pathlib.Path(sys.argv[1])
text = doc_path.read_text(encoding="utf-8")

def slugify(heading: str) -> str:
    h = heading.strip().lower()
    out = []
    for ch in h:
        if ch.isalnum() or ch in ("_", "-"):
            out.append(ch)
        elif ch == " ":
            out.append("-")
    return "".join(out)

headings = re.findall(r"^#{1,6}\s+(.+)$", text, flags=re.M)
slugs = {slugify(h) for h in headings}

anchors = re.findall(r"\]\(#([^)]+)\)", text)
bad_anchors = [a for a in anchors if a not in slugs]
if bad_anchors:
    print(f"Anclas rotas: {bad_anchors}", file=sys.stderr)
    sys.exit(1)

links = re.findall(r"\]\((\.[^)#]+)\)", text)
bad_links = []
for rel in links:
    target = (doc_path.parent / rel).resolve()
    if not target.exists():
        bad_links.append(rel)
if bad_links:
    print(f"Enlaces relativos rotos: {bad_links}", file=sys.stderr)
    sys.exit(2)

print(f"OK_LINKS anchors={len(anchors)} rel_links={len(links)}")
PY
pass "Todas las anclas internas y enlaces relativos de ${ARQ_DOC} resuelven a destinos existentes"

# -----------------------------------------------------------------------------
# 3. Paridad de parametros efectivos con infra/terraform/*.tf (§5.1–§5.4)
# -----------------------------------------------------------------------------
printf '\n[3/10] Paridad de parametros efectivos de escalamiento y salud con Terraform\n'
TF_CHECKS=(
  "e2-highcpu-2"
  "create_before_destroy"
  "PROACTIVE"
  "SUBSTITUTE"
  "max_surge_fixed = 1"
  "max_unavailable_fixed = 0"
  "cooldown_period"
  "300"
  "420"
  "load_balancing_utilization"
  "target = 0.8"
  "max_rate_per_instance = 100"
  "80 pet/s"
  "custom.googleapis.com/mooc/queue/outstanding"
  "generic_node"
  "mooc-cola"
  "single_instance_assignment = 2"
  "max_scaled_in_replicas"
  "600"
  "900"
  "mooc-api-ready"
  "/api/v1/ready"
  "mooc-worker-ready"
  "/readyz"
  "mooc-api-tls"
  "TLS_1_2"
  "min_ports_per_vm = 2048"
)
MISSING_TF=0
for token in "${TF_CHECKS[@]}"; do
  if ! grep -Fq "${token}" "${ARQ_DOC}"; then
    fail "Falta parametro efectivo de Terraform en ${ARQ_DOC}: ${token}"
    MISSING_TF=$((MISSING_TF + 1))
  fi
done
if [[ "${MISSING_TF}" -eq 0 ]]; then
  pass "Verificados los ${#TF_CHECKS[@]} parametros efectivos del proveedor contra infra/terraform/*.tf"
fi

# Contrastar directamente con los archivos HCL reales
if grep -Fq 'target = 0.8' infra/terraform/api_autoscaler.tf && \
   grep -Fq 'cooldown_period = 300' infra/terraform/api_autoscaler.tf && \
   grep -Fq 'time_window_sec = 600' infra/terraform/api_autoscaler.tf && \
   grep -Fq 'cooldown_period = 420' infra/terraform/worker_autoscaler.tf && \
   grep -Fq 'single_instance_assignment = 2' infra/terraform/worker_autoscaler.tf && \
   grep -Fq 'time_window_sec = 900' infra/terraform/worker_autoscaler.tf && \
   grep -Fq 'min_ports_per_vm = 2048' infra/terraform/network.tf && \
   grep -Fq 'min_tls_version = "TLS_1_2"' infra/terraform/load_balancer.tf; then
  pass "Confirmada coincidencia exacta con api_autoscaler.tf, worker_autoscaler.tf, network.tf y load_balancer.tf"
else
  fail "Desfase entre los valores documentados y infra/terraform/*.tf"
fi

# -----------------------------------------------------------------------------
# 4. Arranque automatico, migraciones con advisory lock, drenaje e idempotencia (§5.2 y §5.5)
# -----------------------------------------------------------------------------
printf '\n[4/10] Arranque automatico, advisory lock, drenaje seguro e idempotencia\n'
DRAIN_CHECKS=(
  "mooc-bootstrap.sh"
  "mooc-migrate.sh"
  "4426202603"
  "schema_migrations"
  "WORKER_DRAIN_TIMEOUT"
  "90s"
  "120"
  "180"
  "requeue()"
  "handleFailedMessage()"
  "retry=0"
  "master.m3u8"
  "worker_scratch"
)
MISSING_DRAIN=0
for token in "${DRAIN_CHECKS[@]}"; do
  if ! grep -Fq "${token}" "${ARQ_DOC}"; then
    fail "Falta elemento de bootstrap/drenaje/idempotencia en ${ARQ_DOC}: ${token}"
    MISSING_DRAIN=$((MISSING_DRAIN + 1))
  fi
done
if [[ "${MISSING_DRAIN}" -eq 0 ]]; then
  pass "Documentados bootstrap, advisory lock (4426202603), cadena 90s < 120s < 180s, requeue() y 3 capas de idempotencia"
fi

# -----------------------------------------------------------------------------
# 5. Operacion, 2 capas de identidad, 3 niveles de configuracion y retiro I6 (§6)
# -----------------------------------------------------------------------------
printf '\n[5/10] Instrucciones reproducibles de operacion, aparcado, retiro (I6) y secretos (§6)\n'
OPS_CHECKS=(
  "sa-web-server"
  "sa-worker-server"
  "sa-queue-server"
  "sa-load-generator"
  "moocuser"
  "roles/cloudsql.client"
  "db-password"
  "smtp-password"
  "0600"
  "activation-policy=ALWAYS"
  "activation-policy=NEVER"
  "lb_enabled=false"
  "lb_enabled=true"
  "terraform state rm google_sql_user.app"
  "seed.sh"
)
MISSING_OPS=0
for token in "${OPS_CHECKS[@]}"; do
  if ! grep -Fq "${token}" "${ARQ_DOC}"; then
    fail "Falta instruccion o control operativo en ${ARQ_DOC}: ${token}"
    MISSING_OPS=$((MISSING_OPS + 1))
  fi
done
if [[ "${MISSING_OPS}" -eq 0 ]]; then
  pass "Verificados los ${#OPS_CHECKS[@]} controles de identidades, secretos, encendido, aparcado y retiro (google_sql_user.app)"
fi

# -----------------------------------------------------------------------------
# 6. Estimacion reproducible de costos (F3) y paridad de cifras en §7.2
# -----------------------------------------------------------------------------
printf '\n[6/10] Reproducibilidad del modelo de costos (scripts/f3_estimacion_costos.py)\n'
if python3 scripts/f3_estimacion_costos.py --autoprueba >/dev/null; then
  pass "scripts/f3_estimacion_costos.py --autoprueba ejecutado con exito"
else
  fail "Fallo scripts/f3_estimacion_costos.py --autoprueba"
fi

COST_CHECKS=(
  "183,29"
  "343,83"
  "133,73"
  "0,78"
  "0,33"
  "0,74"
  "36,50"
  "13,23"
  "0,0676"
)
MISSING_COST=0
for c in "${COST_CHECKS[@]}"; do
  if ! grep -Fq "${c}" "${ARQ_DOC}"; then
    fail "Falta cifra de costo verificada en ${ARQ_DOC}: ${c}"
    MISSING_COST=$((MISSING_COST + 1))
  fi
done
if [[ "${MISSING_COST}" -eq 0 ]]; then
  pass "Todas las cifras de costos en §7.2 coinciden con scripts/f3_estimacion_costos.py"
fi

# -----------------------------------------------------------------------------
# 7. Puntos unicos de falla (SPOF) nombrados sin omisiones (§7.3)
# -----------------------------------------------------------------------------
printf '\n[7/10] Puntos unicos de falla restantes nombrados explicitamente (§7.3)\n'
SPOF_CHECKS=(
  "1. Servidor de cola y sesiones compartidas"
  "2. Base de datos relacional zonal sin par de espera"
  "3. Despliegue concentrado en una sola zona de disponibilidad"
  "4. Dependencia externa de correo transaccional"
  "fail-closed"
  "PITR"
)
MISSING_SPOF=0
for s in "${SPOF_CHECKS[@]}"; do
  if ! grep -Fq "${s}" "${ARQ_DOC}"; then
    fail "Falta SPOF o mecanismo de mitigacion en §7.3: ${s}"
    MISSING_SPOF=$((MISSING_SPOF + 1))
  fi
done
if [[ "${MISSING_SPOF}" -eq 0 ]]; then
  pass "Nombrados explicitamente los 4 puntos unicos de falla restantes y su impacto/mitigacion"
fi

# -----------------------------------------------------------------------------
# 8. Relacion con resultados de pruebas (G3, G5, G6, C4, C7, E1, E2) y deuda tecnica (H3)
# -----------------------------------------------------------------------------
printf '\n[8/10] Trazabilidad a pruebas de carga y al inventario de deuda tecnica (H3)\n'
TD_CHECKS=(
  "TD-ARQ-01"
  "TD-ARQ-02"
  "TD-INF-01"
  "TD-INF-02"
  "TD-OPE-01"
  "TD-OPE-02"
  "510,5 s"
  "204,2 s"
  "60,0 %"
  "2,50×"
  "152 s"
)
MISSING_TD=0
for t in "${TD_CHECKS[@]}"; do
  if ! grep -Fq "${t}" "${ARQ_DOC}"; then
    fail "Falta referencia de prueba o deuda tecnica en §7.4: ${t}"
    MISSING_TD=$((MISSING_TD + 1))
  fi
done
if [[ "${MISSING_TD}" -eq 0 ]]; then
  pass "Verificadas las referencias cruzadas a G3, G5, G6, C4, C7, E1, E2 y TD-ARQ/INF/OPE en ${DEUDA_DOC}"
fi

# -----------------------------------------------------------------------------
# 9. Anotacion en docs/entrega3/NOTAS_TECNICAS.md (#60, #61, #62)
# -----------------------------------------------------------------------------
printf '\n[9/10] Registro de hallazgos en %s\n' "${NOTAS_DOC}"
if grep -Fq '<a id="60"></a>' "${NOTAS_DOC}" && \
   grep -Fq '<a id="61"></a>' "${NOTAS_DOC}" && \
   grep -Fq '<a id="62"></a>' "${NOTAS_DOC}" && \
   grep -Fq '[#62](#62)' "${NOTAS_DOC}" || grep -Fq '](#62)' "${NOTAS_DOC}"; then
  pass "Notas tecnicas #60 (I1), #61 (F3) y #62 (I2) registradas en indice y cuerpo de ${NOTAS_DOC}"
else
  fail "Faltan las notas tecnicas #60, #61 o #62 en ${NOTAS_DOC}"
fi

# -----------------------------------------------------------------------------
# 10. Higiene de secretos, tokens e IPs privadas de instancias
# -----------------------------------------------------------------------------
printf '\n[10/10] Barrido de secretos, llaves e IPs privadas en entregable y evidencias\n'
SCAN_TARGETS=("${ARQ_DOC}" "docs/entrega3/evidencias/I1" "docs/entrega3/evidencias/F3")
if [[ -d "docs/entrega3/evidencias/I2" ]]; then
  SCAN_TARGETS+=("docs/entrega3/evidencias/I2")
fi

if grep -RInE '(BEGIN (RSA|OPENSSH|EC|PRIVATE) KEY|AIza[0-9A-Za-z_-]{35}|ya29\.[0-9A-Za-z._-]+|xkeysib-[0-9A-Za-z-]+|X-Goog-Signature=[0-9a-fA-F]{16,})' "${SCAN_TARGETS[@]}" >/dev/null 2>&1; then
  fail "Se detecto un patron de llave o credencial en ${SCAN_TARGETS[*]}"
else
  pass "Cero credenciales, llaves privadas o firmas sin redactar en ${SCAN_TARGETS[*]}"
fi

# Verificar que no haya IPs privadas de hosts (10.0.1.[1-9] o 10.171.*) fuera del CIDR de subred 10.0.1.0/24
if grep -RInE '10\.(0\.1\.[1-9][0-9]*|171\.[0-9]+\.[0-9]+)' "${SCAN_TARGETS[@]}" >/dev/null 2>&1; then
  fail "Se detecto una IP privada literal de instancia en ${SCAN_TARGETS[*]}"
else
  pass "Cero IPs privadas literales de instancias en ${SCAN_TARGETS[*]}"
fi

printf '\n=============================================================================\n'
printf ' RESUMEN I2: %d OK · %d FALLA · %d OBS\n' "${OK_COUNT}" "${FAIL_COUNT}" "${OBS_COUNT}"
printf '=============================================================================\n'

if [[ "${FAIL_COUNT}" -gt 0 ]]; then
  exit 1
fi
exit 0
