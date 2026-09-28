#!/usr/bin/env bash
# Verificacion de red y seguridad del despliegue en la nube (issue #130, G4).
#
# Es de SOLO LECTURA: no crea, cambia ni borra nada en el proyecto. Deja un
# archivo de evidencia por control en docs/entrega2/evidencias/G4/.
#
# Uso:
#   bash ./scripts/verify_g4.sh
#
# Requiere: gcloud con sesion iniciada (`gcloud auth login`), curl y python.
# No usa ni imprime credenciales; los correos personales de los integrantes se
# reemplazan por su conteo para no publicarlos en el repositorio.

set -uo pipefail

PROJECT="${PROJECT_ID:-plataforma-mooc-entrega2}"
WEB_IP="${WEB_IP:-34.24.52.111}"
WORKER_IP="${WORKER_IP:-35.237.6.244}"
BASE="https://${WEB_IP}.sslip.io"
MEDIA_BUCKET="${PROJECT}-media"
HLS_BUCKET="${PROJECT}-hls"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="$(cd "${SCRIPT_DIR}/.." && pwd)/docs/entrega2/evidencias/G4"
mkdir -p "${OUT}"

# curl.exe en Windows (Git Bash), curl en el resto.
if command -v curl.exe >/dev/null 2>&1; then CURL=curl.exe; else CURL=curl; fi
PY=python; command -v python >/dev/null 2>&1 || PY=python3

head_of() { # $1 = archivo, $2 = titulo
  {
    echo "# $2"
    echo "# Generado: $(date -u +%Y-%m-%dT%H:%M:%SZ) por scripts/verify_g4.sh (solo lectura)"
    echo
  } > "$1"
}
code() { "$CURL" -s -o /dev/null --max-time 20 -w "HTTP %{http_code}" "$@"; }

# ---------------------------------------------------------------------------
# a) Puertos vistos desde internet
# ---------------------------------------------------------------------------
f="${OUT}/a_escaneo_puertos_externo.txt"
head_of "$f" "a) Escaneo de puertos desde internet (esta maquina no esta dentro de la VPC)"
for target in "web ${WEB_IP}" "worker ${WORKER_IP}"; do
  set -- $target
  echo "== $1 ($2) ==" >> "$f"
  for p in 22 25 80 443 3000 3306 5432 6379 8080 9090 9100; do
    if timeout 4 bash -c "echo > /dev/tcp/$2/$p" 2>/dev/null; then
      echo "puerto $p: ABIERTO" >> "$f"
    else
      echo "puerto $p: cerrado o filtrado" >> "$f"
    fi
  done
  echo >> "$f"
done

# ---------------------------------------------------------------------------
# b) Base de datos, cola y worker no expuestos
# ---------------------------------------------------------------------------
f="${OUT}/b_base_de_datos_cola_worker.txt"
head_of "$f" "b) Base de datos administrada, cola (Redis) y Worker Server"
{
  echo "== Cloud SQL: direcciones y configuracion de red =="
  gcloud sql instances describe mooc-db-1 --project="$PROJECT" \
    --format="json(state,databaseVersion,ipAddresses,settings.ipConfiguration.ipv4Enabled,settings.ipConfiguration.privateNetwork,settings.ipConfiguration.sslMode,settings.ipConfiguration.authorizedNetworks)"
  echo
  echo "== VMs y direcciones (la IP externa del worker es solo por costo; ver reglas de firewall) =="
  gcloud compute instances list --project="$PROJECT" \
    --format="table(name,networkInterfaces[0].networkIP:label=IP_INTERNA,networkInterfaces[0].accessConfigs[0].natIP:label=IP_EXTERNA,tags.items.list():label=ETIQUETAS)"
  echo
  echo "== Reglas de firewall de ENTRADA =="
  gcloud compute firewall-rules list --project="$PROJECT" --filter="direction=INGRESS" \
    --format="table(name,network.basename(),sourceRanges.list():label=ORIGEN,allowed[].map().firewall_rule().list():label=PERMITE,targetTags.list():label=ETIQUETA_DESTINO)"
  echo
  echo "== Reglas de entrada que alcanzan al worker (etiqueta worker-server) =="
  gcloud compute firewall-rules list --project="$PROJECT" --filter="direction=INGRESS" --format=json | "$PY" -c "
import json, sys
rules = json.load(sys.stdin)
hit = []
for r in rules:
    tags = r.get('targetTags')
    if r['network'].endswith('/mooc-vpc') and (tags is None or 'worker-server' in tags):
        hit.append(r)
for r in hit:
    print(r['name'], '| origen:', ','.join(r.get('sourceRanges', [])), '| permite:', [a['IPProtocol'] + ':' + ','.join(a.get('ports', ['todos'])) for a in r.get('allowed', [])])
print()
world = [r['name'] for r in hit if '0.0.0.0/0' in r.get('sourceRanges', [])]
print('Reglas que abren el worker a internet (0.0.0.0/0):', world if world else 'NINGUNA')
"
} >> "$f" 2>&1

# ---------------------------------------------------------------------------
# c) IAM diferenciado por componente
# ---------------------------------------------------------------------------
f="${OUT}/c_iam_por_componente.txt"
head_of "$f" "c) Cuentas de servicio por componente, sus permisos y sus llaves"
{
  echo "== Cuenta de servicio asignada a cada VM =="
  gcloud compute instances list --project="$PROJECT" \
    --format="table(name,serviceAccounts[0].email:label=CUENTA_DE_SERVICIO)"
  echo
  echo "== Roles a nivel de proyecto por cuenta de servicio de la aplicacion =="
  gcloud projects get-iam-policy "$PROJECT" --format=json | "$PY" -c "
import json, sys
d = json.load(sys.stdin)
for b in sorted(d.get('bindings', []), key=lambda x: x['role']):
    sas = [m for m in b['members'] if m.startswith('serviceAccount:')]
    for m in sas:
        print(f\"{m.split(':',1)[1]:75s} {b['role']}\")
"
  echo
  echo "== Roles humanos a nivel de proyecto (solo conteo, sin correos) =="
  gcloud projects get-iam-policy "$PROJECT" --format=json | "$PY" -c "
import json, sys
d = json.load(sys.stdin)
for b in sorted(d.get('bindings', []), key=lambda x: x['role']):
    n = len([m for m in b['members'] if m.startswith('user:')])
    if n: print(f\"{n} persona(s)  {b['role']}\")
"
  echo
  for b in "$MEDIA_BUCKET" "$HLS_BUCKET"; do
    echo "== IAM del bucket gs://$b =="
    gcloud storage buckets get-iam-policy "gs://$b" --project="$PROJECT" --format=json | "$PY" -c "
import json, sys
d = json.load(sys.stdin)
for b in d.get('bindings', []):
    cond = ' [condicion: %s]' % b['condition']['title'] if 'condition' in b else ''
    print('  ', b['role'], '->', ', '.join(b['members']), cond)
"
    echo
  done
  echo "== Llaves de cuenta de servicio creadas por personas (debe ser 0 en todas) =="
  for sa in $(gcloud iam service-accounts list --project="$PROJECT" --format="value(email)"); do
    sa="${sa//[[:space:]]/}"
    out=""
    for intento in 1 2 3; do
      out="$(gcloud iam service-accounts keys list --iam-account="$sa" --managed-by=user              --project="$PROJECT" --format="value(name)" 2>/dev/null)" && break
      out="ERROR"; sleep 3
    done
    if [ "$out" = "ERROR" ]; then
      echo "$sa : NO SE PUDO CONSULTAR (reintentar)"
    else
      echo "$sa : $(printf '%s' "$out" | grep -c .) llave(s) de usuario"
    fi
  done
  echo
  echo "== La cuenta de computo por defecto sigue con rol Editor a nivel proyecto? =="
  gcloud projects get-iam-policy "$PROJECT" --format=json | "$PY" -c "
import json, sys
d = json.load(sys.stdin)
for b in d['bindings']:
    for m in b['members']:
        if m.endswith('-compute@developer.gserviceaccount.com'):
            print(m.split(':',1)[1], '->', b['role'])
"
} >> "$f" 2>&1

# ---------------------------------------------------------------------------
# d) Secretos
# ---------------------------------------------------------------------------
f="${OUT}/d_secretos.txt"
head_of "$f" "d) Busqueda de secretos en repositorio, historial y evidencias"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
{
  cd "$ROOT" || exit 1
  echo "== 1. Patrones de llaves privadas/tokens en archivos versionados =="
  git ls-files | grep -vE "\.(pdf|png|jpg|mp4|ts|jtl)$" | xargs grep -nIE \
    "BEGIN (RSA |EC |OPENSSH |)PRIVATE KEY|AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z_-]{35}|\"private_key\"|ghp_[A-Za-z0-9]{30,}|xox[baprs]-[A-Za-z0-9-]{10,}" \
    2>/dev/null | sed 's/^/HALLAZGO: /'
  echo "(archivos revisados: $(git ls-files | wc -l); si no hay lineas HALLAZGO arriba, no hay coincidencias)"
  echo
  echo "== 2. Archivos sensibles versionados (.env, llaves, estado y variables de Terraform) =="
  git ls-files | grep -iE "(^|/)\.env($|\.)|\.pem$|\.key$|\.p12$|credentials.*\.json|\.tfstate|\.tfvars$" \
    | sed 's/^/versionado: /'
  echo "(solo debe aparecer .env.example, que lleva los secretos vacios)"
  echo
  echo "== 3. Lo mismo en TODO el historial de git =="
  git log --all --diff-filter=A --name-only --pretty=format: | sort -u \
    | grep -iE "(^|/)\.env($|\.)|\.pem$|\.key$|\.p12$|credentials.*\.json|\.tfstate|\.tfvars$" \
    | sed 's/^/alguna vez subido: /'
  echo
  echo "== 4. Llaves de cuenta de servicio en cualquier commit =="
  n=$(git log --all --oneline -S"private_key_id" | wc -l)
  echo "commits que mencionan private_key_id: $n"
  echo
  echo "== 5. Variables secretas en .env.example (deben ir vacias) =="
  grep -nE "^(POSTGRES_PASSWORD|MINIO_ROOT_PASSWORD|S3_SECRET_KEY|S3_ACCESS_KEY)=" .env.example
  echo
  echo "== 6. Las imagenes no llevan .env: .dockerignore y etapa final =="
  grep -nE "^(\.env|\.env\.\*|\*-sa-key\.json|\.git|infra|docs)" .dockerignore
  grep -nE "^COPY" Dockerfile.api Dockerfile.worker
  echo
  echo "== 7. Tokens Bearer sin redactar dentro de docs/ =="
  grep -rlIE "Bearer [A-Za-z0-9_.-]{25,}" docs | sed 's/^/HALLAZGO: /'
} >> "$f" 2>&1

# ---------------------------------------------------------------------------
# e) HTTPS, cookies seguras y CSRF en el entorno desplegado
# ---------------------------------------------------------------------------
f="${OUT}/e_https_cookies_csrf.txt"
head_of "$f" "e) HTTPS, cookies seguras y CSRF en el despliegue real (${BASE})"
{
  echo "== HTTP en el puerto 80 redirige a HTTPS? =="
  "$CURL" -s -I --max-time 20 "http://${WEB_IP}.sslip.io/" | grep -iE "^HTTP|^Location"
  echo
  echo "== Cabeceras de respuesta HTTPS =="
  "$CURL" -s -I --max-time 20 "${BASE}/api/v1/health" | grep -iE "^HTTP|^Strict-Transport|^X-Content|^X-Frame|^Content-Security"
  echo "(si no aparece Strict-Transport-Security, el servidor no manda HSTS)"
  echo
  echo "== Version minima de TLS: un cliente limitado a TLS 1.1 debe fallar =="
  "$CURL" -s --max-time 15 --tls-max 1.1 -o /dev/null -w "resultado: HTTP %{http_code}, codigo curl %{exitcode}\n" "${BASE}/api/v1/health"
  echo
  echo "== CSRF: peticion que cambia datos, con cookie de sesion, segun el origen =="
  echo -n "Origin de otro sitio (https://evil.example) : "
  "$CURL" -s --max-time 20 -X POST -H "Origin: https://evil.example" -H "Cookie: __Host-mooc_session=falsa" -w " [HTTP %{http_code}]\n" "${BASE}/api/v1/auth/logout"
  echo -n "Sin Origin                                  : "
  "$CURL" -s --max-time 20 -X POST -H "Cookie: __Host-mooc_session=falsa" -w " [HTTP %{http_code}]\n" "${BASE}/api/v1/auth/logout"
  echo -n "Origin propio (la app lo deja pasar)        : "
  "$CURL" -s --max-time 20 -X POST -H "Origin: ${BASE}" -H "Cookie: __Host-mooc_session=falsa" -w " [HTTP %{http_code}]\n" "${BASE}/api/v1/auth/logout"
  echo "(403 csrf_origin_rejected = bloqueado por CSRF; 401 = paso el filtro CSRF y fallo porque la cookie es falsa)"
  echo
  echo "== Atributos de la cookie de sesion =="
  echo "Set-Cookie real capturado en D3: docs/entrega2/evidencias/D3/login_https_cookie_secure.txt"
  echo "Definicion en el codigo (internal/http/handler/auth.go):"
  grep -nE "SessionCookieName =|HttpOnly:|Secure:|SameSite:" "${ROOT}/internal/http/handler/auth.go" | head -6
} >> "$f" 2>&1

# ---------------------------------------------------------------------------
# f) Simetria de los buckets: derivados publicos, originales privados
# ---------------------------------------------------------------------------
f="${OUT}/f_buckets_publico_privado.txt"
head_of "$f" "f) Bucket de derivados HLS (publico) frente al de originales (privado), sin firmar"
{
  HLS_OBJ="$(gcloud storage ls "gs://${HLS_BUCKET}/hls/**/*.m3u8" --project="$PROJECT" 2>/dev/null | head -1)"
  ORIG_OBJ="$(gcloud storage ls "gs://${MEDIA_BUCKET}/originals/**" --project="$PROJECT" 2>/dev/null | grep -v '/$' | head -1)"
  echo "Objeto HLS de muestra : ${HLS_OBJ:-(no hay)}"
  echo "Original de muestra   : ${ORIG_OBJ:-(no hay)}"
  echo
  if [ -n "$HLS_OBJ" ]; then
    echo -n "1) Leer manifiesto HLS sin firmar (esperado 200)           : "
    code "https://storage.googleapis.com/${HLS_OBJ#gs://}"; echo
  fi
  if [ -n "$ORIG_OBJ" ]; then
    echo -n "2) Leer ORIGINAL sin firmar (esperado 403)                 : "
    code "https://storage.googleapis.com/${ORIG_OBJ#gs://}"; echo
  fi
  echo -n "3) Escribir en el bucket HLS sin firmar (esperado 403)      : "
  code -X PUT --data "x" "https://storage.googleapis.com/${HLS_BUCKET}/hls/prueba-anonima.txt"; echo
  echo -n "4) Escribir en el bucket privado sin firmar (esperado 403)  : "
  code -X PUT --data "x" "https://storage.googleapis.com/${MEDIA_BUCKET}/originals/prueba-anonima.txt"; echo
  echo -n "5) Listar el bucket privado sin firmar (esperado 401/403)   : "
  code "https://storage.googleapis.com/storage/v1/b/${MEDIA_BUCKET}/o"; echo
  echo -n "6) Listar el bucket HLS sin firmar                          : "
  code "https://storage.googleapis.com/storage/v1/b/${HLS_BUCKET}/o"; echo
  echo "   (200 significa que cualquiera puede ENUMERAR los nombres de los derivados; ver hallazgo en README)"
} >> "$f" 2>&1

echo "Listo. Evidencia en: ${OUT}"
ls -1 "${OUT}"
