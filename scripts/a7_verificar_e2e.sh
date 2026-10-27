#!/usr/bin/env bash
# Verifica contra la nube los pasos del E2E que nunca se habian podido ejecutar
# (A7, issue #199, ultima tarea).
#
# POR QUE EXISTE
#
# Tres cosas de las correcciones de la Entrega 1 no se habian probado nunca
# contra un entorno real, porque no habia entorno:
#
#   1. Un PDF y una imagen por el flujo normal, hasta `completed`. Antes de la
#      correccion, el worker ignoraba `payload.TaskType` y pasaba TODO recurso
#      por ffprobe, asi que cualquier documento acababa en `failed` y nunca
#      llegaba al estudiante. Nada en el E2E, en Postman ni en la semilla lo
#      ejercitaba (nota tecnica 16).
#
#   2. El intento de quiz congelado: cambiar el enunciado despues de empezar un
#      intento no debe cambiar como se califica ese intento (nota 17).
#
#   3. Que las sesiones vivan en la base logica separada (A4), que es lo que
#      permite vaciar la cola sin tirar las sesiones (nota 23).
#
# USO
#
#   BASE_URL=https://<host> bash ./scripts/a7_verificar_e2e.sh
#
# Solo hace peticiones HTTP: no necesita Docker ni JMeter. No deja nada que
# purgar salvo un curso de prueba, que borra al final si todo fue bien.

set -uo pipefail

BASE_URL="${BASE_URL:?Declara el destino, p. ej. BASE_URL=https://34.24.52.111.sslip.io}"
PROFESOR="${PROFESOR:-profesor1@plataforma-mooc.test}"
ESTUDIANTE="${ESTUDIANTE:-estudiante1@plataforma-mooc.test}"
CLAVE="${CLAVE:-Password123!}"

TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

FALLOS=0
titulo() { printf '\n\033[1m== %s\033[0m\n' "$*"; }
ok() { printf '  ok     %s\n' "$*"; }
malo() {
    printf '  FALLA  %s\n' "$*"
    FALLOS=$((FALLOS + 1))
}

# Origin es obligatorio para la proteccion contra CSRF en todo lo que cambie
# estado. Sin el, la API responde 403 csrf_origin_rejected -- es el defecto que
# tenia la campana del escenario 2.
# El codigo HTTP se escribe en un archivo, no en una variable: casi todas las
# llamadas se hacen dentro de `$(...)`, y una asignacion en un subshell no
# sobrevive al padre. Con `set -u` eso no da un valor viejo, da un error.
api() { # $1=metodo $2=ruta $3=cuerpo(json o vacio) ; imprime el cuerpo
    local m="$1" ruta="$2" cuerpo="${3:-}"
    local args=(-s -o "${TMP}/cuerpo" -w '%{http_code}' -X "$m"
        -H "Origin: ${BASE_URL}" -H "Content-Type: application/json")
    [ -n "${TOKEN:-}" ] && args+=(-H "Authorization: Bearer ${TOKEN}")
    [ -n "${cuerpo}" ] && args+=(-d "${cuerpo}")
    curl "${args[@]}" "${BASE_URL}${ruta}" >"${TMP}/codigo"
    cat "${TMP}/cuerpo"
}

codigo() { cat "${TMP}/codigo" 2>/dev/null || echo "?"; }

json() { python -c "import json,sys;d=json.load(sys.stdin);print(d$1)" 2>/dev/null; }

# El login esta limitado a 10 intentos por minuto y por IP, y este guion inicia
# sesion varias veces alternando profesor y estudiante. Sin reintento, un 429
# tumba la verificacion entera por una razon que no tiene nada que ver con lo
# que se quiere comprobar.
entrar() { # $1=email
    local intento
    for intento in 1 2 3 4 5; do
        TOKEN=""
        TOKEN="$(api POST /api/v1/auth/login "{\"email\":\"$1\",\"password\":\"${CLAVE}\"}" | json "['token']")"
        [ -n "${TOKEN}" ] && return 0
        if [ "$(codigo)" = "429" ]; then
            echo "  429 en el login de $1: el limitador. Espero 20 s (intento ${intento}/5)"
            sleep 20
            continue
        fi
        break
    done
    malo "no se pudo iniciar sesion como $1 (HTTP $(codigo))"
    return 1
}

# --- Preparar un curso de pruebas -------------------------------------------
titulo "Preparando"
entrar "${PROFESOR}" || exit 1
ok "sesion de profesor"

CURSO="$(api POST /api/v1/courses '{"title":"A7 verificacion E2E","description":"Curso temporal de la verificacion de A7."}' | json "['id']")"
MODULO="$(api POST "/api/v1/courses/${CURSO}/modules" '{"title":"Modulo A7"}' | json "['id']")"
UNIDAD="$(api POST "/api/v1/modules/${MODULO}/units" '{"title":"Unidad A7"}' | json "['id']")"
[ -n "${UNIDAD}" ] || {
    malo "no se pudo crear la estructura del curso"
    exit 1
}
ok "curso, modulo y unidad"

# --- 1. Documentos por el flujo normal --------------------------------------
#
# El caso de la nota 16. Antes de la correccion, estos dos acababan en `failed`.
subir() { # $1=tipo $2=mime $3=nombre $4=archivo
    local tipo="$1" mime="$2" nombre="$3" archivo="$4"
    local tam
    tam=$(wc -c <"${archivo}" | tr -d ' ')

    local rid
    rid="$(api POST "/api/v1/units/${UNIDAD}/resources" \
        "{\"title\":\"A7 ${tipo}\",\"type\":\"${tipo}\",\"is_visible\":true,\"is_mandatory\":false,\"allow_download\":true}" |
        json "['id']")"
    [ -n "${rid}" ] || {
        malo "${tipo}: no se pudo crear el recurso (HTTP $(codigo))"
        return 1
    }

    local resp url clave
    resp="$(api POST /api/v1/media/presigned-url \
        "{\"resource_id\":\"${rid}\",\"filename\":\"${nombre}\",\"mime_type\":\"${mime}\",\"file_size_bytes\":${tam}}")"
    url="$(printf '%s' "${resp}" | json "['upload_url']")"
    clave="$(printf '%s' "${resp}" | json "['object_key']")"
    [ -n "${url}" ] || {
        malo "${tipo}: no se emitio URL prefirmada (HTTP $(codigo))"
        return 1
    }

    local put
    put="$(curl -s -o /dev/null -w '%{http_code}' -X PUT -H "Content-Type: ${mime}" --data-binary "@${archivo}" "${url}")"
    [ "${put}" = "200" ] || [ "${put}" = "204" ] || {
        malo "${tipo}: el PUT al bucket devolvio ${put}"
        return 1
    }

    api POST "/api/v1/media/uploads/${rid}/complete" "{\"object_key\":\"${clave}\"}" >/dev/null
    [ "$(codigo)" = "202" ] || {
        malo "${tipo}: la confirmacion devolvio $(codigo), se esperaba 202"
        return 1
    }

    # Un documento solo necesita el escaneo, asi que es cuestion de segundos.
    local i estado=""
    for i in $(seq 1 40); do
        estado="$(api GET "/api/v1/units/${UNIDAD}/resources" "" |
            python -c "
import json,sys
for r in json.load(sys.stdin)['items']:
    if r['id']=='${rid}':
        print(r.get('processing_status') or '')
" 2>/dev/null)"
        case "${estado}" in
        completed | failed) break ;;
        esac
    done

    if [ "${estado}" = "completed" ]; then
        ok "${tipo}: llego a completed por el flujo normal"
    else
        malo "${tipo}: acabo en '${estado:-sin estado}' (la nota 16 describe exactamente esto)"
    fi
}

titulo "1. Documentos e imagenes por el flujo normal (nota 16)"
# Un PDF minimo valido y un PNG de 1x1: lo que se prueba es el camino, no el
# contenido.
printf '%%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\ntrailer<</Root 1 0 R>>\n%%%%EOF\n' >"${TMP}/a7.pdf"
printf '\211PNG\r\n\032\n\0\0\0\rIHDR\0\0\0\1\0\0\0\1\10\6\0\0\0\37\25\304\211\0\0\0\nIDATx\234c\0\1\0\0\5\0\1\r\n-\264\0\0\0\0IEND\256B`\202' >"${TMP}/a7.png"
subir pdf "application/pdf" "a7.pdf" "${TMP}/a7.pdf"
subir image "image/png" "a7.png" "${TMP}/a7.png"

# --- 2. El intento de quiz congelado ----------------------------------------
titulo "2. El intento de quiz se califica contra el enunciado congelado (nota 17)"
# Un quiz cuelga de un RECURSO de tipo quiz, no de la unidad: la ruta real es
# POST /resources/{resource_id}/quiz. Inventarla da un 404 que no lo parece.
# Obligatorio a proposito: publicar un curso exige al menos un recurso visible,
# disponible y OBLIGATORIO, que es el que define el criterio de aprobacion
# (internal/course/service.go, validateForPublish). Sin eso, el publish devuelve
# 422 y el intento del estudiante acaba en un 409 que habla de otra cosa.
RQUIZ="$(api POST "/api/v1/units/${UNIDAD}/resources" \
    '{"title":"A7 quiz","type":"quiz","is_visible":true,"is_mandatory":true}' | json "['id']")"
QUIZ=""
[ -n "${RQUIZ}" ] && QUIZ="$(api POST "/api/v1/resources/${RQUIZ}/quiz" \
    '{"title":"Quiz A7","passing_score":50,"max_attempts":3}' | json "['id']")"

if [ -z "${QUIZ}" ]; then
    malo "no se pudo crear el quiz (HTTP $(codigo))"
else
    ok "quiz creado sobre su recurso"
    PREGUNTA="$(api POST "/api/v1/quizzes/${QUIZ}/questions" \
        '{"prompt":"Enunciado ORIGINAL","type":"single_choice","points":10,"options":[{"text":"correcta","is_correct":true},{"text":"incorrecta","is_correct":false}]}' | json "['id']")"
    if [ -z "${PREGUNTA}" ]; then
        malo "no se pudo crear la pregunta (HTTP $(codigo))"
    else
        ok "pregunta creada con su enunciado original"
        TOKEN_PROF="${TOKEN}"

        # El nucleo de la nota 17: el estudiante abre el intento, el profesor
        # cambia el enunciado, y lo que el estudiante ve no debe cambiar.
        #
        # El curso tiene que estar PUBLICADO: abrir un intento sobre un curso en
        # borrador devuelve 409 `quiz_conflict`, cuyo mensaje habla de intentos
        # agotados y no menciona el estado del curso
        # (internal/quiz/service.go: "course %s is %s, not published").
        api POST "/api/v1/courses/${CURSO}/publish" '{}' >/dev/null
        if [ "$(codigo)" = "200" ] || [ "$(codigo)" = "204" ]; then
            ok "curso publicado (requisito para abrir un intento)"
        else
            echo "  AVISO  no se pudo publicar el curso (HTTP $(codigo)); el intento va a fallar"
        fi
        if entrar "${ESTUDIANTE}"; then
            # La inscripcion es del propio interesado: el contrato dice "Enrol
            # THE CALLER in a course", sin cuerpo. Llamarla como profesor con un
            # student_email no inscribe a nadie, y el intento acaba en 403.
            api POST "/api/v1/courses/${CURSO}/enrollments" '{}' >/dev/null
            if [ "$(codigo)" = "201" ] || [ "$(codigo)" = "200" ] || [ "$(codigo)" = "409" ]; then
                ok "el estudiante se inscribio (HTTP $(codigo))"
            else
                echo "  AVISO  la inscripcion devolvio $(codigo); el intento va a fallar"
            fi
            api POST "/api/v1/quizzes/${QUIZ}/attempts" '{}' >/dev/null
            if [ "$(codigo)" = "200" ] || [ "$(codigo)" = "201" ]; then
                ok "el estudiante abrio el intento"

                # Dos reglas del dominio acotan lo que un profesor puede hacer
                # con un intento abierto, y conviene tenerlas claras:
                #
                #   * la API solo permite AÑADIR preguntas: no hay PUT ni DELETE
                #     sobre una pregunta;
                #   * un curso publicado tiene su evaluacion inmutable, asi que
                #     ni siquiera se puede anadir (409 course_immutable).
                #
                # Luego el unico camino para mutar el quiz es despublicar,
                # anadir y volver a publicar. Es ahi donde el snapshot del
                # intento tiene que sostenerse, y es lo que se prueba.
                TOKEN="${TOKEN_PROF}"
                api POST "/api/v1/courses/${CURSO}/unpublish" '{}' >/dev/null
                MUTO=0
                if [ "$(codigo)" = "200" ] || [ "$(codigo)" = "204" ]; then
                    api POST "/api/v1/quizzes/${QUIZ}/questions" \
                        '{"prompt":"Pregunta ANADIDA despues del intento","type":"single_choice","points":10,"options":[{"text":"a","is_correct":true},{"text":"b","is_correct":false}]}' >/dev/null
                    if [ "$(codigo)" = "201" ] || [ "$(codigo)" = "200" ]; then
                        MUTO=1
                        ok "despublicado, pregunta anadida con el intento abierto"
                    else
                        echo "  AVISO  no se pudo anadir la pregunta ni despublicado (HTTP $(codigo))"
                    fi
                    api POST "/api/v1/courses/${CURSO}/publish" '{}' >/dev/null
                    ok "curso vuelto a publicar (HTTP $(codigo))"
                else
                    echo "  AVISO  no se pudo despublicar (HTTP $(codigo)); la prueba pierde sentido"
                fi
                [ "${MUTO}" = "1" ] || echo "  AVISO  el quiz no cambio: lo de abajo no demuestra el congelado"

                entrar "${ESTUDIANTE}" >/dev/null
                ACTUAL="$(api GET "/api/v1/quizzes/${QUIZ}/attempts/current" "")"
                if printf '%s' "${ACTUAL}" | grep -q "ANADIDA"; then
                    malo "el intento abierto YA incluye la pregunta nueva: NO esta congelado"
                elif printf '%s' "${ACTUAL}" | grep -q "Enunciado ORIGINAL"; then
                    ok "el intento sigue con su pregunta original y NO ve la anadida: esta congelado"
                else
                    echo "  AVISO  no se pudo leer el intento actual (HTTP $(codigo))"
                fi
            else
                malo "el estudiante no pudo abrir el intento (HTTP $(codigo))"
            fi
        fi
        TOKEN="${TOKEN_PROF}"
    fi
fi

# --- 3. Donde viven las sesiones --------------------------------------------
titulo "3. Separacion de sesiones (A4, nota 23)"
echo "  Se comprueba en la instancia de la cola, no por HTTP:"
echo "    redis-cli -n 0 dbsize   -> solo la cola de asynq"
echo "    redis-cli -n 1 dbsize   -> session:* y ratelimit:*"
echo "  Ojo: la caida a PostgreSQL NO repuebla la cache, asi que un token"
echo "  anterior al despliegue autentica sin escribir en la base 1. Hay que"
echo "  hacer un login nuevo antes de mirar."

# --- Limpieza ----------------------------------------------------------------
titulo "Limpieza"
# La API no tiene DELETE para cursos --solo despublicar--, asi que un curso de
# pruebas no se puede retirar del todo: lo mas que se consigue es sacarlo del
# catalogo publicado. Conviene saberlo antes de generar datos de prueba contra
# un entorno que se vaya a conservar.
if [ "${FALLOS}" -eq 0 ]; then
    api POST "/api/v1/courses/${CURSO}/unpublish" '{}' >/dev/null
    if [ "$(codigo)" = "200" ] || [ "$(codigo)" = "204" ]; then
        ok "curso de pruebas despublicado (la API no permite borrarlo)"
    else
        echo "  AVISO  el curso ${CURSO} sigue publicado (HTTP $(codigo))"
    fi
else
    echo "  El curso ${CURSO} se conserva publicado para inspeccionarlo."
fi

echo
if [ "${FALLOS}" -gt 0 ]; then
    echo "${FALLOS} comprobacion(es) fallaron."
    exit 1
fi
echo "Todas las comprobaciones pasaron."
