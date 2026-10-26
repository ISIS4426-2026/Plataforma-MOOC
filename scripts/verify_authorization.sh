#!/usr/bin/env bash
# Autorizacion negativa por la peticion subyacente (issues #214, D4): lo que el servidor responde
# cuando un estudiante intenta acciones de profesor y cuando un profesor intenta tocar el curso
# de otro. Demuestra que ocultar un boton en la interfaz NO es el control de acceso: aunque se
# salte la interfaz, el servidor rechaza.
#
# Uso:
#   BASE_URL=https://<origen-del-frontend> CAPACITY_PASSWORD=<contrasena de prueba> \
#     bash scripts/verify_authorization.sh
#   Opcionales: PROF_A, PROF_B, ESTUDIANTE (correos de cuentas de prueba)
#
# Crea UN curso en borrador del profesor A (titulo «[verificacion D4] ...», sin contenido) para
# tener un curso ajeno real contra el que probar. No hay forma de borrar un curso por la API: queda
# como borrador. La contrasena entra por entorno y no se imprime.

set -uo pipefail

BASE_URL="${BASE_URL:?Falta BASE_URL: el origen del frontend, p. ej. http://localhost:8081}"
BASE_URL="${BASE_URL%/}"
PASSWORD="${CAPACITY_PASSWORD:?Falta CAPACITY_PASSWORD (contrasena de prueba de la semilla)}"
PROF_A="${PROF_A:-profesor1@plataforma-mooc.test}"
PROF_B="${PROF_B:-profesor2@plataforma-mooc.test}"
ESTUDIANTE="${ESTUDIANTE:-carga.estudiante0001@plataforma-mooc.test}"

if command -v curl.exe >/dev/null 2>&1; then CURL=curl.exe; else CURL=curl; fi
PY=python; command -v python >/dev/null 2>&1 || PY=python3
fallos=0

JAR_A="$(mktemp)"; JAR_B="$(mktemp)"; JAR_E="$(mktemp)"
trap 'rm -f "$JAR_A" "$JAR_B" "$JAR_E"' EXIT

login() { # $1 jar, $2 correo -> HTTP code
  "$CURL" -s -o /dev/null -c "$1" -w "%{http_code}" --max-time 30 -X POST -H "Origin: ${BASE_URL}" -H "Content-Type: application/json" \
    -d "{\"email\":\"$2\",\"password\":\"${PASSWORD}\"}" "${BASE_URL}/api/v1/auth/login"
}
peticion() { # $1 jar, $2 metodo, $3 ruta, [$4 cuerpo] -> "HTTP codigo"
  local cuerpo="${4:-}"
  local args=(-s --max-time 30 -b "$1" -X "$2" -H "Origin: ${BASE_URL}" -H "Idempotency-Key: $(date +%s%N)-$RANDOM" -w '\n%{http_code}')
  [ -n "$cuerpo" ] && args+=(-H "Content-Type: application/json" -d "$cuerpo")
  local salida; salida="$("$CURL" "${args[@]}" "${BASE_URL}/api/v1$3")"
  local http="${salida##*$'\n'}"; local json="${salida%$'\n'*}"
  local code; code="$(printf '%s' "$json" | "$PY" -c 'import json,sys
try: print(json.load(sys.stdin).get("code",""))
except Exception: print("")')"
  echo "${http} ${code}"
}
esperar() { # $1 descripcion, $2 esperado (codigo HTTP), $3 obtenido ("HTTP codigo")
  local http="${3%% *}"
  if [ "$http" = "$2" ]; then printf '  OK     %-62s -> %s\n' "$1" "$3"; else printf '  FALLA  %-62s -> esperado %s, obtenido %s\n' "$1" "$2" "$3"; fallos=$((fallos + 1)); fi
}

echo "== Origen: ${BASE_URL}  ($(date -u +%Y-%m-%dT%H:%M:%SZ))"
[ "$(login "$JAR_A" "$PROF_A")" = "200" ] || { echo "No se pudo iniciar sesion como profesor A"; exit 2; }
[ "$(login "$JAR_B" "$PROF_B")" = "200" ] || { echo "No se pudo iniciar sesion como profesor B"; exit 2; }
[ "$(login "$JAR_E" "$ESTUDIANTE")" = "200" ] || { echo "No se pudo iniciar sesion como estudiante"; exit 2; }

CREADO="$("$CURL" -s --max-time 30 -b "$JAR_A" -X POST -H "Origin: ${BASE_URL}" -H "Content-Type: application/json" -H "Idempotency-Key: d4-$(date +%s%N)" \
  -d '{"title":"[verificacion D4] curso del profesor A","description":"Borrador creado por scripts/verify_authorization.sh para probar la autorizacion."}' "${BASE_URL}/api/v1/courses")"
ID="$(printf '%s' "$CREADO" | "$PY" -c 'import json,sys; print(json.load(sys.stdin)["id"])' 2>/dev/null | tr -d '\r')"
[ -n "$ID" ] || { echo "El profesor A no pudo crear su curso de prueba"; exit 2; }
echo "Curso de prueba del profesor A: <curso-A> (borrador)"
echo

echo "-- 1. El dueno SI puede (control positivo)"
esperar "Profesor A edita su propio curso (PUT /courses/<curso-A>)" 200 "$(peticion "$JAR_A" PUT "/courses/${ID}" '{"title":"[verificacion D4] curso del profesor A","description":"editado por su dueno"}')"
esperar "Profesor A agrega un modulo a su curso" 201 "$(peticion "$JAR_A" POST "/courses/${ID}/modules" '{"title":"Modulo del dueno"}')"
esperar "Profesor A lista sus cursos (GET /me/courses)" 200 "$(peticion "$JAR_A" GET "/me/courses")"

echo "-- 2. Un profesor NO puede tocar el curso de otro profesor"
esperar "Profesor B edita el curso de A (PUT)" 403 "$(peticion "$JAR_B" PUT "/courses/${ID}" '{"title":"secuestrado","description":"x"}')"
esperar "Profesor B agrega un modulo al curso de A (POST)" 403 "$(peticion "$JAR_B" POST "/courses/${ID}/modules" '{"title":"intruso"}')"
esperar "Profesor B publica el curso de A (POST publish)" 403 "$(peticion "$JAR_B" POST "/courses/${ID}/publish" '{}')"
esperar "Profesor B despublica el curso de A (POST unpublish)" 403 "$(peticion "$JAR_B" POST "/courses/${ID}/unpublish" '{}')"
esperar "Profesor B lista la estructura del curso de A (GET modules)" 403 "$(peticion "$JAR_B" GET "/courses/${ID}/modules")"
B_LISTA="$("$CURL" -s --max-time 30 -b "$JAR_B" "${BASE_URL}/api/v1/me/courses" | "$PY" -c 'import json,sys; print(sum(1 for c in json.load(sys.stdin)["items"] if c["id"]=="'"${ID}"'"))')"
if [ "${B_LISTA:-1}" = "0" ]; then echo "  OK     El curso de A NO aparece en «mis cursos» del profesor B"; else echo "  FALLA  el curso de A aparece en la lista del profesor B"; fallos=$((fallos + 1)); fi

echo "-- 3. Un estudiante NO puede ejecutar acciones reservadas al profesor"
esperar "Estudiante crea un curso (POST /courses)" 403 "$(peticion "$JAR_E" POST "/courses" '{"title":"x","description":"y"}')"
esperar "Estudiante agrega un modulo al curso de A" 403 "$(peticion "$JAR_E" POST "/courses/${ID}/modules" '{"title":"intruso"}')"
esperar "Estudiante publica el curso de A" 403 "$(peticion "$JAR_E" POST "/courses/${ID}/publish" '{}')"
esperar "Estudiante edita el curso de A (PUT)" 403 "$(peticion "$JAR_E" PUT "/courses/${ID}" '{"title":"x","description":"y"}')"
esperar "Estudiante pide los cursos de autor (GET /me/courses)" 403 "$(peticion "$JAR_E" GET "/me/courses")"

echo "-- 4. Sin sesion"
esperar "Sin sesion: GET /me/courses" 401 "$(peticion /dev/null GET "/me/courses")"
esperar "Sin sesion: POST /courses" 401 "$(peticion /dev/null POST "/courses" '{"title":"x","description":"y"}')"

echo
if [ "$fallos" -eq 0 ]; then echo "RESULTADO: todas las comprobaciones pasaron"; else echo "RESULTADO: ${fallos} comprobacion(es) fallaron"; fi
exit "$fallos"
