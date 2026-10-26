#!/usr/bin/env bash
# Verifica CORS y CSRF frente al frontend (issue #211, D1): el origen del frontend pasa y un
# origen ajeno es rechazado. Deja el resultado en la salida estandar (se guarda como evidencia).
#
# Uso:
#   BASE_URL=https://<balanceador> CAPACITY_PASSWORD=<contrasena de prueba> \
#     bash scripts/verify_frontend_origin.sh [correo]
#   (contra local: BASE_URL=http://localhost:8081, origen del frontend = BASE_URL)
#
# BASE_URL es el origen del FRONTEND (la API vive en el mismo, bajo /api). El correo por defecto
# es la primera cuenta de carga sembrada (scripts/seeds/capacity_data.sql). La contrasena entra
# por variable de entorno y no se imprime ni se guarda. Solo escribe una sesion y la cierra.

set -uo pipefail

BASE_URL="${BASE_URL:?Falta BASE_URL: el origen del frontend, p. ej. http://localhost:8081}"
BASE_URL="${BASE_URL%/}"
EMAIL="${1:-carga.estudiante0001@plataforma-mooc.test}"
PASSWORD="${CAPACITY_PASSWORD:?Falta CAPACITY_PASSWORD (contrasena de prueba de la semilla)}"
AJENO="https://sitio-ajeno.example"

if command -v curl.exe >/dev/null 2>&1; then CURL=curl.exe; else CURL=curl; fi
fallos=0
check() { # $1 descripcion, $2 esperado, $3 obtenido
  if [ "$2" = "$3" ]; then echo "  OK   $1 -> $3"; else echo "  FALLA $1 -> esperado $2, obtenido $3"; fallos=$((fallos + 1)); fi
}
codigo() { "$CURL" -s -o /dev/null --max-time 20 -w "%{http_code}" "$@"; }
cuerpo() { "$CURL" -s --max-time 20 "$@"; }

JAR="$(mktemp)"; trap 'rm -f "$JAR"' EXIT
echo "== Origen del frontend: ${BASE_URL}"

echo "-- 1. El frontend sirve su pagina y su version"
check "GET /" 200 "$(codigo "${BASE_URL}/")"
echo "     version: $(cuerpo "${BASE_URL}/version.json" | tr -d '\n ' | cut -c1-120)"

echo "-- 2. Login desde el origen del frontend (el navegador manda Origin: ${BASE_URL})"
check "POST /auth/login con Origin propio" 200 "$(codigo -c "$JAR" -X POST -H "Origin: ${BASE_URL}" -H "Content-Type: application/json" \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" "${BASE_URL}/api/v1/auth/login")"
echo "     cookie de sesion recibida: $(grep -c '__Host-mooc_session' "$JAR") (nombre con prefijo __Host-, Secure, HttpOnly)"

echo "-- 3. CSRF: una escritura con la cookie de sesion y un origen AJENO se rechaza"
check "POST /auth/logout con Origin ajeno" 403 "$(codigo -b "$JAR" -X POST -H "Origin: ${AJENO}" "${BASE_URL}/api/v1/auth/logout")"
echo "     respuesta: $(cuerpo -b "$JAR" -X POST -H "Origin: ${AJENO}" "${BASE_URL}/api/v1/auth/logout" | tr -d '\n' | cut -c1-110)"
check "POST /auth/logout SIN Origin (con cookie)" 403 "$(codigo -b "$JAR" -X POST "${BASE_URL}/api/v1/auth/logout")"

echo "-- 4. CORS: el origen ajeno no recibe permiso para leer respuestas"
CABECERAS="$("$CURL" -s -D - -o /dev/null --max-time 20 -X OPTIONS -H "Origin: ${AJENO}" \
  -H "Access-Control-Request-Method: POST" -H "Access-Control-Request-Headers: content-type" "${BASE_URL}/api/v1/auth/login" | tr -d '\r')"
if echo "$CABECERAS" | grep -qi '^access-control-allow-origin:'; then
  echo "  FALLA el preflight desde origen ajeno devolvio Access-Control-Allow-Origin"; fallos=$((fallos + 1))
else
  echo "  OK   preflight desde origen ajeno sin Access-Control-Allow-Origin"
fi
if echo "$CABECERAS" | grep -qi '^access-control-allow-origin: \*'; then echo "  FALLA comodin *"; fallos=$((fallos + 1)); else echo "  OK   nunca devuelve comodin *"; fi

echo "-- 5. La escritura legitima con la cookie y el origen del frontend SI pasa (cierra la sesion)"
check "POST /auth/logout con Origin propio (204 = sin contenido)" 204 "$(codigo -b "$JAR" -X POST -H "Origin: ${BASE_URL}" "${BASE_URL}/api/v1/auth/logout")"

echo
if [ "$fallos" -eq 0 ]; then echo "RESULTADO: todas las comprobaciones pasaron"; else echo "RESULTADO: ${fallos} comprobacion(es) fallaron"; fi
exit "$fallos"
