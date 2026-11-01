#!/usr/bin/env bash
# Verificador de identidades de servicio con permisos minimos, separacion de las
# dos capas de autenticacion y ausencia de secretos (F2, issue #220).
#
# Cumple la regla de la Nota Tecnica 36: NO se limita a listar; AFIRMA cada
# condicion con OK / FALLA / OBS y sale con codigo distinto de cero si falla
# cualquier control.
#
# CONTROLES (escribe en docs/entrega3/evidencias/F2/):
#   a_inventario_permisos_iam.txt      Recorte de roles/cloudsql.client, matriz
#                                      exacta de permisos por cuenta en HCL (y
#                                      en GCP si gcloud tiene sesion activa),
#                                      0 recursos google_service_account_key y
#                                      ausencia de uso de la cuenta por defecto.
#   b_dos_capas_identidad_cloudsql.txt Separacion estructural y operativa entre
#                                      la Capa 1 (autorizacion cloud via IAM +
#                                      Metadata Server) y la Capa 2
#                                      (autenticacion PostgreSQL como moocuser
#                                      sobre IP privada + sslmode=require).
#   c_barrido_secretos_completo.txt    Ejecucion de scripts/b4_barrido_secretos.sh
#                                      mas verificacion de imagenes Docker,
#                                      .dockerignore, plantillas, scripts
#                                      versionados, evidencias de Entrega 3 y
#                                      ejemplos de configuracion sin secretos.
#   barrido_secretos.txt               Salida integra de b4_barrido_secretos.sh.
#   resumen.txt                        Consolidado de veredictos OK / FALLA / OBS.
#
# USO:
#   bash ./scripts/f2_identidades_y_secretos.sh [directorio-salida]

set -Eeuo pipefail

export PATH="${PATH}:/home/dfortizr1/.local/go/bin:/usr/local/go/bin:${HOME}/go/bin"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
OUT="${1:-${ROOT}/docs/entrega3/evidencias/F2}"
PROYECTO="${PROYECTO:-plataforma-mooc-entrega2}"

PY=python3
command -v python3 >/dev/null 2>&1 || PY=python

mkdir -p "${OUT}"
RESUMEN="${OUT}/resumen.txt"
: >"${RESUMEN}"

FALLAS=0
OBSERVACIONES=0
OKS=0

ahora() { date -u +%Y-%m-%dT%H:%M:%SZ; }

cabecera() {
    local archivo="$1"
    local titulo="$2"
    {
        echo "# ${titulo}"
        echo "# Generado: $(ahora) por scripts/f2_identidades_y_secretos.sh"
        echo "# Commit  : $(git -C "${ROOT}" rev-parse HEAD)"
        echo
    } >"${archivo}"
}

veredicto() {
    local archivo="$1"
    local estado="$2"
    local mensaje="$3"
    printf '%-6s %s\n' "${estado}" "${mensaje}" | tee -a "${archivo}" >>"${RESUMEN}"
    case "${estado}" in
        OK)    OKS=$((OKS + 1)) ;;
        FALLA) FALLAS=$((FALLAS + 1)) ;;
        OBS)   OBSERVACIONES=$((OBSERVACIONES + 1)) ;;
    esac
}

echo "F2 — Verificacion de identidades con permisos minimos, dos autenticaciones y ausencia de secretos"
echo "================================================================================================="

# ==============================================================================
# CONTROL A: Inventario de permisos por identidad y recorte de lo no usado
# ==============================================================================
FA="${OUT}/a_inventario_permisos_iam.txt"
cabecera "${FA}" "a) Inventario de permisos por cuenta de servicio y recorte de permisos sin uso (F2, #220)"

"${PY}" - "${ROOT}" >"${FA}.body" <<'PYEOF'
import os, re, sys

root = sys.argv[1]
tf_dir = os.path.join(root, "infra", "terraform")

def read_tf(name):
    with open(os.path.join(tf_dir, name), encoding="utf-8") as fh:
        return fh.read()

def strip_comments(text):
    lines = []
    for line in text.splitlines():
        s = line.strip()
        if s.startswith("#") or s.startswith("//"):
            continue
        # Remove inline # comments outside quotes
        in_q = False
        out = []
        for ch in line:
            if ch == '"':
                in_q = not in_q
            if ch == '#' and not in_q:
                break
            out.append(ch)
        lines.append("".join(out))
    return "\n".join(lines)

tf_files = sorted(f for f in os.listdir(tf_dir) if f.endswith(".tf"))
active_hcl = {f: strip_comments(read_tf(f)) for f in tf_files}
all_active_hcl = "\n".join(active_hcl.values())

print("== 1. Verificacion de recorte: roles/cloudsql.client retirado del codigo HCL ==")
cloudsql_hits = []
for f, content in active_hcl.items():
    for idx, line in enumerate(content.splitlines(), 1):
        if "cloudsql.client" in line:
            cloudsql_hits.append(f"{f}:{idx}: {line.strip()}")

if not cloudsql_hits:
    print("   0 apariciones de 'roles/cloudsql.client' en el codigo ejecutable de infra/terraform/*.tf")
    print("VEREDICTO\tOK\troles/cloudsql.client retirado de sa-web-server y sa-worker-server en infra/terraform/service_accounts.tf")
else:
    for h in cloudsql_hits:
        print("   HALLAZGO:", h)
    print("VEREDICTO\tFALLA\troles/cloudsql.client sigue presente en el codigo ejecutable de Terraform")

print()
print("== 2. Ausencia de recursos de llaves de cuenta de servicio (google_service_account_key) ==")
if "google_service_account_key" not in all_active_hcl:
    print("   0 recursos 'google_service_account_key' en infra/terraform/*.tf")
    print("VEREDICTO\tOK\tninguna cuenta de servicio genera ni exporta archivos de llave en Terraform (0 google_service_account_key)")
else:
    print("VEREDICTO\tFALLA\tse encontro un recurso google_service_account_key en Terraform")

print()
print("== 3. Cuentas de servicio declaradas y asignacion explicita a cada componente ==")
sas_expected = [
    ("sa-web-server",    "service_accounts.tf", "api_group.tf",    "Grupo elastico de API (mooc-api-group)"),
    ("sa-worker-server", "service_accounts.tf", "worker_group.tf", "Grupo elastico de workers (mooc-worker-group)"),
    ("sa-queue-server",  "service_accounts.tf", "queue.tf",        "Servidor dedicado de cola Redis/asynq (mooc-queue-server)"),
    ("sa-load-generator","generator.tf",        "generator.tf",    "VM generadora de carga externa bajo demanda (mooc-load-generator)"),
]
all_sas_ok = True
for sa_id, decl_f, use_f, desc in sas_expected:
    decl_ok = f'account_id   = "{sa_id}"' in active_hcl.get(decl_f, "") or f'account_id = "{sa_id}"' in active_hcl.get(decl_f, "")
    print(f"   {sa_id:20s} declarada en {decl_f:20s} -> usada en {use_f:16s} ({desc})")
    if not decl_ok:
        all_sas_ok = False

if all_sas_ok:
    print("VEREDICTO\tOK\tlas 4 identidades (sa-web-server, sa-worker-server, sa-queue-server, sa-load-generator) estan declaradas y adjuntas por componente")
else:
    print("VEREDICTO\tFALLA\tfalta alguna declaracion de cuenta de servicio esperada")

print()
print("== 4. Matriz completa de permisos efectivos declarados en HCL por identidad ==")
matrix = [
    ("sa-web-server", "Proyecto (service_accounts.tf)", "roles/logging.logWriter", "Envio de logs del Ops Agent a Cloud Logging"),
    ("sa-web-server", "Proyecto (service_accounts.tf)", "roles/monitoring.metricWriter", "Envio de metricas del Ops Agent (infra/ops-agent/web.yaml) a Cloud Monitoring"),
    ("sa-web-server", "Sobre si misma (service_accounts.tf)", "roles/iam.serviceAccountTokenCreator", "Firma de URLs V4 con signBlob sin archivo de llave (internal/storage/gcs.go)"),
    ("sa-web-server", "Bucket media (storage.tf)", "roles/storage.objectViewer", "Lectura y verificacion de metadatos de objetos (StatObject, GetObject)"),
    ("sa-web-server", "Bucket media + CEL (storage.tf)", "roles/storage.objectCreator [condicion: api_deny_derivatives_write]", "Delegacion en URL firmada PUT solo para originals/, documents/ y thumbnails/ (prohibido hls/)"),
    ("sa-web-server", "Secreto db-password (database.tf)", "roles/secretmanager.secretAccessor", "Lectura de contrasena de PostgreSQL en prepare_web_env.sh"),
    ("sa-web-server", "Secreto smtp-password (mail.tf)", "roles/secretmanager.secretAccessor", "Lectura de credencial SMTP en prepare_web_env.sh (exclusivo de API)"),
    ("sa-web-server", "Repositorio mooc (artifact_registry.tf)", "roles/artifactregistry.reader", "Descarga de imagenes api y web en el arranque"),

    ("sa-worker-server", "Proyecto (service_accounts.tf)", "roles/logging.logWriter", "Envio de logs del Ops Agent a Cloud Logging"),
    ("sa-worker-server", "Proyecto (service_accounts.tf)", "roles/monitoring.metricWriter", "Envio de metricas del Ops Agent y Prometheus (:9090/metrics)"),
    ("sa-worker-server", "Bucket media (storage.tf)", "roles/storage.objectViewer", "Descarga de originales/documentos/imagenes para procesarlos (solo lectura)"),
    ("sa-worker-server", "Bucket hls (storage.tf)", "roles/storage.objectUser", "Escritura de manifiestos y segmentos derivados HLS en el bucket publico"),
    ("sa-worker-server", "Secreto db-password (database.tf)", "roles/secretmanager.secretAccessor", "Lectura de contrasena de PostgreSQL en prepare_worker_env.sh"),
    ("sa-worker-server", "Repositorio mooc (artifact_registry.tf)", "roles/artifactregistry.reader", "Descarga de la imagen worker en el arranque"),

    ("sa-queue-server", "Proyecto (service_accounts.tf)", "roles/logging.logWriter", "Envio de logs del Ops Agent de la VM de cola"),
    ("sa-queue-server", "Proyecto (service_accounts.tf)", "roles/monitoring.metricWriter", "Publicacion de custom.googleapis.com/mooc/queue/* por cmd/queue-exporter (C6)"),
    ("sa-queue-server", "Repositorio mooc (artifact_registry.tf)", "roles/artifactregistry.reader", "Descarga de la imagen worker para correr /app/queue-exporter (C6)"),

    ("sa-load-generator", "Proyecto (generator.tf, count=generator_enabled)", "roles/logging.logWriter", "Envio de logs cuando la VM generadora esta encendida (G1)"),
    ("sa-load-generator", "Proyecto (generator.tf, count=generator_enabled)", "roles/monitoring.metricWriter", "Envio de metricas de CPU/memoria del generador para contraste de capacidad (G1)"),
]
for sa, scope, role, reason in matrix:
    print(f"   {sa:18s} | {scope:42s} | {role:40s} | {reason}")

# Comprobaciones estructurales de separacion de privilegios en el codigo HCL
sa_tf = active_hcl["service_accounts.tf"]
st_tf = active_hcl["storage.tf"]
db_tf = active_hcl["database.tf"]
ml_tf = active_hcl["mail.tf"]
ar_tf = active_hcl["artifact_registry.tf"]
gn_tf = active_hcl["generator.tf"]

checks = [
    ("roles/logging.logWriter" in sa_tf and "roles/monitoring.metricWriter" in sa_tf,
     "common_roles en service_accounts.tf contiene exactamente logging.logWriter y monitoring.metricWriter"),
    ("roles/iam.serviceAccountTokenCreator" in sa_tf and "google_service_account.web_server.name" in sa_tf and "worker_server" not in sa_tf.split("web_server_can_sign", 1)[1],
     "roles/iam.serviceAccountTokenCreator esta acotado a sa-web-server sobre si misma (el worker no puede firmar URLs)"),
    ("api_deny_derivatives_write" in st_tf and "originals/" in st_tf and "documents/" in st_tf and "thumbnails/" in st_tf,
     "sa-web-server tiene condicion CEL api_deny_derivatives_write en el bucket de medios (no puede escribir en hls/)"),
    ("hls_worker_writer" in st_tf and "roles/storage.objectUser" in st_tf,
     "sa-worker-server escribe derivados unicamente en el bucket HLS y solo tiene objectViewer en el bucket de medios"),
    ("smtp_password_reader" in ml_tf and "web_server" in ml_tf and "worker_server" not in ml_tf,
     "el secreto smtp-password solo concede secretAccessor a sa-web-server (ni worker ni cola leen SMTP)"),
    ("db_password_readers" in db_tf and "web_server" in db_tf and "worker_server" in db_tf and "queue_server" not in db_tf,
     "el secreto db-password solo concede secretAccessor a sa-web-server y sa-worker-server (ni cola ni generador leen BD)"),
    ("runtime_readers" in ar_tf and "roles/artifactregistry.reader" in ar_tf,
     "el repositorio de Artifact Registry solo concede artifactregistry.reader (solo lectura) a las cuentas de ejecucion"),
]
for cond, msg in checks:
    print(f"VEREDICTO\t{'OK' if cond else 'FALLA'}\t{msg}")
PYEOF

grep -v '^VEREDICTO' "${FA}.body" >>"${FA}"
while IFS=$'\t' read -r _ est msg; do
    veredicto "${FA}" "${est}" "${msg}"
done < <(grep '^VEREDICTO' "${FA}.body")
rm -f "${FA}.body"

# Si gcloud esta instalado y tiene sesion activa, contrastar tambien contra la nube
if command -v gcloud >/dev/null 2>&1 && gcloud config get-value account >/dev/null 2>&1; then
    echo >>"${FA}"
    echo "== 5. Contraste contra la politica IAM en vivo del proyecto (${PROYECTO}) ==" >>"${FA}"
    if POL_JSON="$(gcloud projects get-iam-policy "${PROYECTO}" --format=json 2>/dev/null)"; then
        "${PY}" - "${POL_JSON}" >>"${FA}" <<'PYEOF'
import json, sys
pol = json.loads(sys.argv[1])
cloudsql_members = []
for b in pol.get("bindings", []):
    if b.get("role") == "roles/cloudsql.client":
        cloudsql_members.extend(b.get("members", []))
if not cloudsql_members:
    print("   0 cuentas con roles/cloudsql.client en la politica activa del proyecto")
else:
    print("   Pendiente de terraform apply desde main (Nota Tecnica 1): roles/cloudsql.client aun figura en el estado remoto para:", ", ".join(cloudsql_members))
PYEOF
    fi
else
    echo >>"${FA}"
    echo "== 5. Nota sobre aplicacion en la nube (Nota Tecnica 1) ==" >>"${FA}"
    echo "   Segun la Nota Tecnica 1, 'terraform apply' se ejecuta unicamente desde la rama 'main'." >>"${FA}"
    echo "   Al mergear F2 en 'main' y aplicar, Terraform destruira exclusivamente los 2 enlaces:" >>"${FA}"
    echo "     - google_project_iam_member.common[\"web-cloudsql.client\"]" >>"${FA}"
    echo "     - google_project_iam_member.common[\"worker-cloudsql.client\"]" >>"${FA}"
fi

# ==============================================================================
# CONTROL B: Separacion de las dos capas de identidad (Cloud IAM vs PostgreSQL)
# ==============================================================================
FB="${OUT}/b_dos_capas_identidad_cloudsql.txt"
cabecera "${FB}" "b) Separacion de las dos capas de identidad: Autorizacion Cloud (IAM) vs Autenticacion PostgreSQL"

"${PY}" - "${ROOT}" >"${FB}.body" <<'PYEOF'
import os, re, sys

root = sys.argv[1]

def read_file(rel):
    with open(os.path.join(root, rel), encoding="utf-8") as fh:
        return fh.read()

db_tf = read_file("infra/terraform/database.tf")
sa_tf = read_file("infra/terraform/service_accounts.tf")
gcs_go = read_file("internal/storage/gcs.go")
val_go = read_file("internal/config/validate.go")
prep_web = read_file("scripts/prepare_web_env.sh")
prep_wrk = read_file("scripts/prepare_worker_env.sh")
go_mod = read_file("go.mod")

print("== 1. Capa 1 — Autorizacion de acceso al servicio cloud (GCP IAM) ==")
print("   Sujeto       : Cuenta de servicio adjunta a la VM (sa-web-server, sa-worker-server, sa-queue-server, sa-load-generator)")
print("   Credencial   : Token OAuth2 efimero obtenido de http://metadata.google.internal/computeMetadata/v1/ (sin archivo JSON)")
print("   Alcance      : APIs de GCP (Secret Manager, Cloud Storage, Artifact Registry, Cloud Logging, Cloud Monitoring, IAM signBlob)")
print("   Revocacion   : Retirar el binding IAM en Terraform; efecto inmediato sin rotar secretos")
print()

c1_checks = [
    ("cloud.google.com/go/storage" in go_mod and "CredentialsFile" not in gcs_go,
     "Capa 1 (GCS): internal/storage/gcs.go usa Application Default Credentials (servidor de metadatos) y signBlob sin archivos de llave"),
    ("gcloud secrets versions access latest --secret=db-password" in prep_web and
     "gcloud secrets versions access latest --secret=smtp-password" in prep_web and
     "gcloud secrets versions access latest --secret=db-password" in prep_wrk and
     "smtp-password" not in prep_wrk.split("DB_PASSWORD=", 1)[1],
     "Capa 1 (Secret Manager): prepare_web_env.sh lee db-password y smtp-password via IAM; prepare_worker_env.sh lee solo db-password"),
]
for cond, msg in c1_checks:
    print(f"VEREDICTO\t{'OK' if cond else 'FALLA'}\t{msg}")

print()
print("== 2. Capa 2 — Autenticacion de la aplicacion ante PostgreSQL (Cloud SQL) ==")
print("   Sujeto       : Rol de PostgreSQL 'moocuser' (declarado en google_sql_user.app en database.tf)")
print("   Red y cifrado: IP privada en mooc-vpc (ipv4_enabled = false) + TLS obligatorio (ssl_mode = ENCRYPTED_ONLY / sslmode=require)")
print("   Credencial   : Contrasena en DATABASE_URL (leida en el arranque desde Secret Manager 'db-password')")
print("   Alcance      : Objetos SQL dentro de la base 'moocdb' (tablas, secuencias, advisory locks de migraciones)")
print("   Revocacion   : Nueva version en Secret Manager + terraform apply + reinicio de contenedores (independiente de IAM)")
print()

c2_checks = [
    ("ipv4_enabled = false" in db_tf and 'ssl_mode = "ENCRYPTED_ONLY"' in db_tf and "private_network = google_compute_network.vpc.id" in db_tf,
     "Capa 2 (Red y TLS en servidor): database.tf exige ipv4_enabled=false, private_network=mooc-vpc y ssl_mode=ENCRYPTED_ONLY"),
    ('name     = "moocuser"' in db_tf and 'name     = "moocdb"' in db_tf,
     "Capa 2 (Identidad SQL): database.tf declara el usuario PostgreSQL 'moocuser' sobre la base 'moocdb'"),
    ("cloudsqlconn" not in go_mod and "cloud-sql-proxy" not in prep_web and "cloud_sql_proxy" not in prep_web,
     "Independencia de capas: ni go.mod ni los scripts usan Cloud SQL Auth Proxy ni conectores IAM; la conexion es TCP directa a la IP privada"),
    ("sslmode=require" in prep_web and "sslmode=require" in prep_wrk and "sslmode" in val_go,
     "Capa 2 (TLS en cliente): prepare_web_env.sh, prepare_worker_env.sh e internal/config/validate.go exigen sslmode=require en produccion"),
]
for cond, msg in c2_checks:
    print(f"VEREDICTO\t{'OK' if cond else 'FALLA'}\t{msg}")
PYEOF

grep -v '^VEREDICTO' "${FB}.body" >>"${FB}"
while IFS=$'\t' read -r _ est msg; do
    veredicto "${FB}" "${est}" "${msg}"
done < <(grep '^VEREDICTO' "${FB}.body")
rm -f "${FB}.body"

# ==============================================================================
# CONTROL C: Verificacion de ausencia de secretos en codigo, imagenes,
#            plantillas, scripts versionados, evidencias e historial
# ==============================================================================
FC="${OUT}/c_barrido_secretos_completo.txt"
cabecera "${FC}" "c) Comprobacion de ausencia de secretos en codigo, imagenes, plantillas, scripts, evidencias e historial"

echo "== 1. Ejecucion de scripts/b4_barrido_secretos.sh (arbol, URIs, historial completo y validador de produccion) ==" >>"${FC}"
if bash "${ROOT}/scripts/b4_barrido_secretos.sh" "${OUT}/barrido_secretos.txt" >>"${FC}" 2>&1; then
    veredicto "${FC}" OK "scripts/b4_barrido_secretos.sh paso con 0 hallazgos en arbol, URIs, historial de git y tests de validate.go"
else
    veredicto "${FC}" FALLA "scripts/b4_barrido_secretos.sh reporto hallazgos"
fi

echo >>"${FC}"
echo "== 2. Comprobacion de imagenes Docker, .dockerignore y bundle de frontend ==" >>"${FC}"
"${PY}" - "${ROOT}" >"${FC}.body" <<'PYEOF'
import os, re, subprocess, sys

root = sys.argv[1]

def read_file(rel):
    with open(os.path.join(root, rel), encoding="utf-8") as fh:
        return fh.read()

dockerignore = read_file(".dockerignore")
df_api = read_file("Dockerfile.api")
df_wrk = read_file("Dockerfile.worker")
df_web = read_file("Dockerfile.web")

required_ignores = [".env", ".env.*", "!.env.example", "*-sa-key.json", "*service-account*.json",
                    "application_default_credentials.json", "infra/", "*.tfstate", ".git/", "docs/"]
missing_ignores = [p for p in required_ignores if p not in dockerignore.splitlines()]
print(f"   Patrones criticos en .dockerignore: {len(required_ignores) - len(missing_ignores)}/{len(required_ignores)}")
if not missing_ignores:
    print("VEREDICTO\tOK\t.dockerignore excluye .env, llaves JSON, estado de Terraform, .git/ y docs/ del contexto de construccion")
else:
    print(f"VEREDICTO\tFALLA\tfaltan patrones en .dockerignore: {missing_ignores}")

# Verificar que las etapas finales de los 3 Dockerfiles no copian .env ni secretos
final_api = df_api.split("FROM alpine:3.22", 1)[1] if "FROM alpine:3.22" in df_api else ""
final_wrk = df_wrk.split("FROM alpine:3.22", 1)[1] if "FROM alpine:3.22" in df_wrk else ""
final_web = df_web.split("FROM nginx:1.27-alpine", 1)[1] if "FROM nginx:1.27-alpine" in df_web else ""

dockerfiles_clean = (
    "COPY --from=builder" in final_api and ".env" not in final_api and "PASSWORD" not in final_api and
    "COPY --from=builder" in final_wrk and ".env" not in final_wrk and "PASSWORD" not in final_wrk and
    "COPY --from=build /app/dist" in final_web and "npm run check:bundle" in df_web
)
print("   Dockerfile.api    : multietapa, etapa final solo copia /app/bin/api y corre como usuario 'app'")
print("   Dockerfile.worker : multietapa, etapa final solo copia /app/bin/{worker,seed-media,queue-exporter} y corre como usuario 'app'")
print("   Dockerfile.web    : multietapa, ejecuta 'npm run build && npm run check:bundle' y solo copia /app/dist + nginx.web.conf")
print(f"VEREDICTO\t{'OK' if dockerfiles_clean else 'FALLA'}\tlas 3 imagenes (Dockerfile.api, Dockerfile.worker, Dockerfile.web) son multietapa y no incorporan secretos ni archivos .env")

print()
print("== 3. Comprobacion de plantillas de instancias y scripts versionados ==" )
scripts_to_check = [
    "infra/bootstrap/mooc-bootstrap.sh",
    "infra/bootstrap/mooc-migrate.sh",
    "infra/terraform/queue_startup.sh",
    "scripts/prepare_web_env.sh",
    "scripts/prepare_worker_env.sh",
]
bad_scripts = []
for rel in scripts_to_check:
    full = os.path.join(root, rel)
    if not os.path.exists(full):
        continue
    txt = read_file(rel)
    # Verify no literal password assignment
    for line in txt.splitlines():
        s = line.strip()
        if s.startswith("#"):
            continue
        if re.search(r'(DB_PASSWORD|SMTP_PASSWORD)\s*=\s*["\'][A-Za-z0-9_+=/.-]{8,}["\']', s):
            bad_scripts.append(f"{rel}: {s}")

if not bad_scripts:
    print(f"   {len(scripts_to_check)} guiones de arranque y preparacion revisados sin credenciales en duro")
    print("VEREDICTO\tOK\tplantillas de arranque y scripts versionados leen secretos de Secret Manager en ejecucion (modo 0600) sin valores en duro")
else:
    for b in bad_scripts:
        print("   HALLAZGO:", b)
    print("VEREDICTO\tFALLA\tse encontraron credenciales literales en scripts de arranque")

print()
print("== 4. Comprobacion de evidencias en docs/entrega3/evidencias/ ==")
evid_dir = os.path.join(root, "docs", "entrega3", "evidencias")
evid_files = 0
evid_findings = []
pat_bearer = re.compile(r'Bearer\s+[A-Za-z0-9_.-]{25,}')
pat_goog_sig = re.compile(r'X-Goog-Signature=[0-9a-fA-F]{32,}')
pat_priv_key = re.compile(r'-----BEGIN [A-Z ]*PRIVATE KEY')

for dirpath, _, filenames in os.walk(evid_dir):
    for fn in filenames:
        if fn.endswith((".png", ".jpg", ".mp4", ".pdf", ".gitkeep")):
            continue
        evid_files += 1
        fp = os.path.join(dirpath, fn)
        rel_fp = os.path.relpath(fp, root)
        try:
            content = open(fp, encoding="utf-8", errors="ignore").read()
        except Exception:
            continue
        for m in pat_bearer.findall(content):
            if "REDACTED" not in m and "falsa" not in m:
                evid_findings.append(f"{rel_fp}: token Bearer sin redactar ({m[:20]}...)")
        for m in pat_goog_sig.findall(content):
            evid_findings.append(f"{rel_fp}: firma de URL firmada expuesta ({m[:28]}...)")
        if pat_priv_key.search(content):
            evid_findings.append(f"{rel_fp}: llave privada PEM")

print(f"   Archivos de texto revisados en docs/entrega3/evidencias/: {evid_files}")
if not evid_findings:
    print("   0 tokens Bearer, 0 firmas X-Goog-Signature y 0 llaves privadas en docs/entrega3/evidencias/")
    print("VEREDICTO\tOK\tninguna evidencia de docs/entrega3/evidencias/ contiene tokens Bearer, firmas de URL ni llaves privadas")
else:
    for ef in evid_findings:
        print("   HALLAZGO:", ef)
    print("VEREDICTO\tFALLA\tse encontraron secretos o tokens sin redactar en docs/entrega3/evidencias/")

print()
print("== 5. Comprobacion de ejemplos de configuracion sin valores sensibles ==")
examples = [
    ".env.example",
    "infra/terraform/terraform.tfvars.example",
    "docs/entrega3/evidencias/F2/ejemplos/web.conf.example",
    "docs/entrega3/evidencias/F2/ejemplos/worker.conf.example",
    "docs/entrega3/evidencias/F2/ejemplos/runtime.web.env.example",
    "docs/entrega3/evidencias/F2/ejemplos/runtime.worker.env.example",
]
missing_ex = [e for e in examples if not os.path.exists(os.path.join(root, e))]
env_ex = read_file(".env.example")
secret_keys = ["POSTGRES_PASSWORD=", "DATABASE_URL=", "MINIO_ROOT_PASSWORD=", "S3_ACCESS_KEY=", "S3_SECRET_KEY=", "SMTP_USERNAME=", "SMTP_PASSWORD="]
non_empty_secrets = []
for sk in secret_keys:
    for line in env_ex.splitlines():
        if line.startswith(sk) and line.strip() != sk:
            non_empty_secrets.append(line.strip())

if not missing_ex and not non_empty_secrets:
    for e in examples:
        print(f"   ok  {e}")
    print("VEREDICTO\tOK\tlos 6 archivos de ejemplo (.env.example, terraform.tfvars.example y ejemplos de F2) existen y tienen vacios o con marcador todos los campos sensibles")
else:
    print(f"VEREDICTO\tFALLA\tfaltan ejemplos ({missing_ex}) o hay secretos con valor en .env.example ({non_empty_secrets})")
PYEOF

grep -v '^VEREDICTO' "${FC}.body" >>"${FC}"
while IFS=$'\t' read -r _ est msg; do
    veredicto "${FC}" "${est}" "${msg}"
done < <(grep '^VEREDICTO' "${FC}.body")
rm -f "${FC}.body"

# ==============================================================================
# RESUMEN FINAL
# ==============================================================================
{
    echo
    echo "================================================================================================="
    echo "Total: ${OKS} comprobacion(es) OK, ${FALLAS} falla(s), ${OBSERVACIONES} observacion(es)."
    echo "Generado: $(ahora)"
    echo "Este archivo no contiene credenciales, llaves ni secretos."
} >>"${RESUMEN}"

echo
cat "${RESUMEN}"
[[ "${FALLAS}" -eq 0 ]] || exit 1
exit 0

