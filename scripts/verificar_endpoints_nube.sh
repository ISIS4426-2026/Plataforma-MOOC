#!/usr/bin/env bash
# Comprueba contra el entorno desplegado los endpoints que **ninguna coleccion
# de Postman toca**, para que el frontend no se los encuentre rotos.
#
# POR QUE HACE FALTA ADEMAS DE NEWMAN
#
# `scripts/test_postman_cloud.sh` corre 206 peticiones repartidas en siete
# colecciones y cubre 44 de las 57 rutas que registra `internal/http/server.go`.
# Las 13 restantes no las toca nadie, y entre ellas estan cosas que un frontend
# usa el primer dia: editar un modulo, renombrar una unidad, borrarla.
#
# Este guion **no sustituye** a newman. Cubre el hueco, y nada mas.
#
# QUE COMPRUEBA, Y QUE NO
#
# Comprueba que cada ruta existe, autoriza a quien debe y devuelve el codigo
# esperado. No valida los cuerpos de respuesta campo por campo: de eso se
# encargan las colecciones y las pruebas de Go. Un 404 aqui significa «esa
# llamada no existe», que es justo lo que el frontend necesita saber antes de
# escribirla.
#
# USO
#
#   BASE_URL="$(cd infra/terraform && terraform output -raw lb_url)" \
#     bash ./scripts/verificar_endpoints_nube.sh
#
# OJO CON EL LIMITADOR
#
# El limite de login son 10 intentos por minuto **y por IP**, no por cuenta. Este
# guion hace tres logins y los reutiliza. No lo lances pegado a una corrida de
# newman o los primeros pasos fallaran con 429 sin que nada este roto.
set -uo pipefail

BASE_URL="${BASE_URL:?Declara el destino, p. ej. BASE_URL=https://34.24.52.111.sslip.io}"
ADMIN="${ADMIN:-admin@plataforma-mooc.test}"
PROFESOR="${PROFESOR:-profesor1@plataforma-mooc.test}"
ESTUDIANTE="${ESTUDIANTE:-estudiante1@plataforma-mooc.test}"
CLAVE="${CLAVE:-Password123!}"

TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

OK=0
FALLOS=0
PENDIENTES=()

verde() { printf '\033[0;32m%s\033[0m\n' "$*"; }
rojo() { printf '\033[0;31m%s\033[0m\n' "$*"; }

# $1=token $2=metodo $3=ruta $4=esperado(lista separada por |) $5=cuerpo
probar() {
    local token="$1" metodo="$2" ruta="$3" esperado="$4" cuerpo="${5:-}"
    local args=(-s -o "${TMP}/cuerpo" -w '%{http_code}' -X "${metodo}"
        -H "Origin: ${BASE_URL}" -H "Content-Type: application/json")
    [ -n "${token}" ] && args+=(-H "Authorization: Bearer ${token}")
    if [ -n "${cuerpo}" ]; then
        args+=(-d "${cuerpo}")
    elif [ "${metodo}" != "GET" ] && [ "${metodo}" != "DELETE" ]; then
        # UN POST SIN CUERPO NECESITA `Content-Length: 0` O NO LLEGA A LA API.
        #
        # El balanceador de Google responde **411 Length Required** a un POST sin
        # Content-Length, antes de tocar nuestro servidor. Afecta a todas las
        # llamadas sin cuerpo: /publish, /unpublish, /enrollments, /attempts,
        # /auth/logout, /auth/verify/resend.
        #
        # Un navegador lo manda solo --`fetch(u, {method:"POST"})` emite
        # Content-Length: 0-- asi que el frontend no lo sufre; curl y varios
        # clientes de servidor, si.
        args+=(-H "Content-Length: 0")
    fi

    local codigo
    codigo="$(curl "${args[@]}" "${BASE_URL}${ruta}")"

    if printf '%s' "${esperado}" | tr '|' '\n' | grep -qx "${codigo}"; then
        verde "  ok     ${metodo} ${ruta} -> ${codigo}"
        OK=$((OK + 1))
    else
        rojo "  FALLO  ${metodo} ${ruta} -> ${codigo} (se esperaba ${esperado})"
        rojo "         $(head -c 200 "${TMP}/cuerpo")"
        FALLOS=$((FALLOS + 1))
        PENDIENTES+=("${metodo} ${ruta} -> ${codigo}, se esperaba ${esperado}")
    fi
}

json() { python -c "import json,sys;d=json.load(sys.stdin);print(d$1)" 2>/dev/null; }

entrar() {
    curl -s -X POST -H "Origin: ${BASE_URL}" -H "Content-Type: application/json" \
        -d "{\"email\":\"$1\",\"password\":\"${CLAVE}\"}" \
        "${BASE_URL}/api/v1/auth/login" | json "['token']"
}

echo "Endpoints sin cobertura de Postman, contra ${BASE_URL}"
echo "======================================================"
echo

# ------------------------------------------------------- 1. sin autenticacion
echo "1. Sondas e infraestructura (sin token)"
probar "" GET /api/v1/health 200
probar "" GET /api/v1/live 200
probar "" GET /api/v1/ready 200
# 403 es lo CORRECTO: nginx.mig.conf deniega esta ruta a proposito --servirla
# publicaria el inventario de rutas, los codigos de respuesta y el volumen de
# trafico--. El agente de observabilidad raspa el 8080 del contenedor y no pasa
# por el proxy. Un 200 aqui seria el fallo.
probar "" GET /api/v1/metrics 403
probar "" GET /api/docs "200|301|302"
echo

# ------------------------------------------------------------- 2. las sesiones
echo "2. Sesiones"
T_ADMIN="$(entrar "${ADMIN}")"
T_PROF="$(entrar "${PROFESOR}")"
T_EST="$(entrar "${ESTUDIANTE}")"
for par in "admin:${T_ADMIN}" "profesor:${T_PROF}" "estudiante:${T_EST}"; do
    if [ -n "${par#*:}" ]; then
        verde "  ok     sesion de ${par%%:*}"
        OK=$((OK + 1))
    else
        rojo "  FALLO  no se pudo iniciar sesion como ${par%%:*}"
        rojo "         sin esto el resto no prueba nada; revisa la siembra y el limitador"
        FALLOS=$((FALLOS + 1))
        exit 1
    fi
done
echo

# --------------------------------------------------------------- 3. el admin
echo "3. Administracion"
probar "${T_ADMIN}" GET /api/v1/admin/audit-logs 200
# Esta ruta NUNCA existio en el servidor, aunque el contrato la declaraba. Se
# borro de api/openapi.yaml el 29 de octubre de 2026 junto con su esquema.
#
# La comprobacion se queda como guardia: si alguien la reintroduce en el
# contrato sin implementarla, aqui vuelve a verse. Un 404 es el resultado
# correcto; lo que daria que pensar es un 201.
probar "${T_ADMIN}" POST /users/professors 404 \
    '{"email":"nuevo.profesor@plataforma-mooc.test","full_name":"Nuevo","password":"Password123!"}'
echo

# ------------------------------------------------- 4. edicion de la estructura
echo "4. Edicion de la estructura del curso (lo que mas usa un frontend)"
CURSO="$(curl -s -X POST -H "Origin: ${BASE_URL}" -H "Authorization: Bearer ${T_PROF}" \
    -H "Content-Type: application/json" \
    -d '{"title":"Verificacion de endpoints","description":"Curso temporal para comprobar las rutas sin cobertura."}' \
    "${BASE_URL}/api/v1/courses" | json "['id']")"
MODULO="$(curl -s -X POST -H "Origin: ${BASE_URL}" -H "Authorization: Bearer ${T_PROF}" \
    -H "Content-Type: application/json" -d '{"title":"Modulo original"}' \
    "${BASE_URL}/api/v1/courses/${CURSO}/modules" | json "['id']")"
UNIDAD="$(curl -s -X POST -H "Origin: ${BASE_URL}" -H "Authorization: Bearer ${T_PROF}" \
    -H "Content-Type: application/json" -d '{"title":"Unidad original"}' \
    "${BASE_URL}/api/v1/modules/${MODULO}/units" | json "['id']")"
RECURSO="$(curl -s -X POST -H "Origin: ${BASE_URL}" -H "Authorization: Bearer ${T_PROF}" \
    -H "Content-Type: application/json" \
    -d '{"title":"Recurso original","type":"pdf","is_visible":true,"is_mandatory":false,"allow_download":false}' \
    "${BASE_URL}/api/v1/units/${UNIDAD}/resources" | json "['id']")"

if [ -z "${CURSO}" ] || [ -z "${MODULO}" ] || [ -z "${UNIDAD}" ] || [ -z "${RECURSO}" ]; then
    rojo "  FALLO  no se pudo construir el curso de prueba"
    rojo "         curso=${CURSO:-vacio} modulo=${MODULO:-vacio} unidad=${UNIDAD:-vacio} recurso=${RECURSO:-vacio}"
    exit 1
fi
verde "  ok     curso de prueba ${CURSO}"
OK=$((OK + 1))

probar "${T_PROF}" PUT "/api/v1/modules/${MODULO}" 200 '{"title":"Modulo renombrado"}'
probar "${T_PROF}" PUT "/api/v1/units/${UNIDAD}" 200 '{"title":"Unidad renombrada"}'
probar "${T_PROF}" PUT "/api/v1/resources/${RECURSO}" 200 \
    '{"title":"Recurso renombrado","is_visible":true,"is_mandatory":false,"allow_download":false}'
# Un estudiante no puede editar la estructura. Que la ruta exista no basta: si
# autorizara de mas, el frontend no lo notaria hasta que alguien lo probara.
probar "${T_EST}" PUT "/api/v1/modules/${MODULO}" "403|404" '{"title":"No deberia"}'
probar "${T_PROF}" DELETE "/api/v1/units/${UNIDAD}" "200|204"
probar "${T_PROF}" DELETE "/api/v1/modules/${MODULO}" "200|204"
echo

# ---------------------------------------------------------- 5. los intentos
echo "5. Intentos de quiz"
M2="$(curl -s -X POST -H "Origin: ${BASE_URL}" -H "Authorization: Bearer ${T_PROF}" \
    -H "Content-Type: application/json" -d '{"title":"Modulo del quiz"}' \
    "${BASE_URL}/api/v1/courses/${CURSO}/modules" | json "['id']")"
U2="$(curl -s -X POST -H "Origin: ${BASE_URL}" -H "Authorization: Bearer ${T_PROF}" \
    -H "Content-Type: application/json" -d '{"title":"Unidad del quiz"}' \
    "${BASE_URL}/api/v1/modules/${M2}/units" | json "['id']")"
R2="$(curl -s -X POST -H "Origin: ${BASE_URL}" -H "Authorization: Bearer ${T_PROF}" \
    -H "Content-Type: application/json" \
    -d '{"title":"Recurso del quiz","type":"quiz","is_visible":true,"is_mandatory":true,"allow_download":false}' \
    "${BASE_URL}/api/v1/units/${U2}/resources" | json "['id']")"
# `is_mandatory: true` no es decoracion: publicar un curso exige **al menos un
# recurso obligatorio y visible**, y sin publicar no hay inscripcion ni intentos.
QUIZ="$(curl -s -X POST -H "Origin: ${BASE_URL}" -H "Authorization: Bearer ${T_PROF}" \
    -H "Content-Type: application/json" \
    -d '{"title":"Quiz de verificacion","passing_score":50,"max_attempts":3}' \
    "${BASE_URL}/api/v1/resources/${R2}/quiz" | json "['id']")"

if [ -z "${QUIZ}" ]; then
    rojo "  FALLO  no se pudo crear el quiz; los intentos no se pueden probar"
    FALLOS=$((FALLOS + 1))
    PENDIENTES+=("no se pudo crear el quiz de prueba")
else
    curl -s -o /dev/null -X POST -H "Origin: ${BASE_URL}" -H "Authorization: Bearer ${T_PROF}" \
        -H "Content-Type: application/json" \
        -d '{"statement":"¿Dos mas dos?","type":"single_choice","points":1,"options":[{"text":"4","is_correct":true},{"text":"5","is_correct":false}]}' \
        "${BASE_URL}/api/v1/quizzes/${QUIZ}/questions"
    curl -s -o "${TMP}/publicar" -X POST -H "Origin: ${BASE_URL}" -H "Authorization: Bearer ${T_PROF}" \
        -H "Content-Length: 0" "${BASE_URL}/api/v1/courses/${CURSO}/publish"
    curl -s -o /dev/null -X POST -H "Origin: ${BASE_URL}" -H "Authorization: Bearer ${T_EST}" \
        -H "Content-Length: 0" "${BASE_URL}/api/v1/courses/${CURSO}/enrollments"

    if [ -s "${TMP}/publicar" ] && grep -q '"code"' "${TMP}/publicar"; then
        rojo "  AVISO  la publicacion del curso devolvio: $(head -c 200 "${TMP}/publicar")"
        rojo "         sin curso publicado el estudiante no se inscribe y los intentos fallan"
    fi

    probar "${T_EST}" POST "/api/v1/quizzes/${QUIZ}/attempts" "200|201"
    probar "${T_EST}" GET "/api/v1/quizzes/${QUIZ}/attempts/current" 200
    probar "${T_EST}" PUT "/api/v1/quizzes/${QUIZ}/attempts/current" 200 '{"answers":[]}'
fi
echo

# ------------------------------------------------------------------ resumen
echo "======================================================"
echo "Comprobaciones correctas: ${OK}"
echo "Fallos                 : ${FALLOS}"
if [ "${FALLOS}" -gt 0 ]; then
    echo
    echo "Lo que no respondio como se esperaba:"
    for p in "${PENDIENTES[@]}"; do echo "  - ${p}"; done
fi
echo
echo "El curso ${CURSO} se queda publicado: la API no permite borrar cursos."
[ "${FALLOS}" -eq 0 ]
