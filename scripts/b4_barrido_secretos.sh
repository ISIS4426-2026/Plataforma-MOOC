#!/usr/bin/env bash
# Barrido de patrones de secreto en el arbol y en todo el historial (B4, #203).
#
# POR QUE ES UN GUION Y NO UNA ORDEN DE UNA VEZ
#
# El criterio de aceptacion de B4 es «busqueda de patrones de secreto en el arbol
# y en todo el historial sin coincidencias», y eso no se comprueba una vez: hay
# que repetirlo antes de etiquetar la entrega (I5) y al cerrar (I6). Un guion
# versionado tambien deja constancia de **que** se busco, que es la mitad del
# valor de la comprobacion: un barrido sin patrones documentados no distingue
# «no hay secretos» de «no busque bien».
#
# QUE BUSCA, Y POR QUE ESOS PATRONES
#
#   * llaves privadas en PEM y JSON de cuenta de servicio, que son el hallazgo
#     que de verdad compromete un proyecto de GCP;
#   * claves de API con prefijo reconocible: Google (AIza), Brevo (xkeysib-),
#     AWS (AKIA), GitHub (ghp_, github_pat_);
#   * JWT, que es como se filtra una sesion en un volcado de registro;
#   * contrasenas asignadas con valor literal, que es el caso que ningun prefijo
#     detecta;
#   * contrasenas dentro de una URI de conexion, que es la forma concreta en la
#     que este proyecto podria filtrar la de la base.
#
# USO
#
#   bash ./scripts/b4_barrido_secretos.sh                 # a la salida estandar
#   bash ./scripts/b4_barrido_secretos.sh <fichero>       # y tambien a un fichero
#
# Sale con 1 si encuentra algo, para que una canalizacion se pare.
set -Eeuo pipefail

SALIDA="${1:-}"
HALLAZGOS=0

# Los patrones, uno por linea, con su nombre. Se mantienen aqui y no repartidos
# por el guion para que la evidencia pueda listarlos tal cual.
declare -a NOMBRES=(
    "llave privada en PEM"
    "JSON de cuenta de servicio"
    "clave de API de Google"
    "clave de API de Brevo"
    "clave de acceso de AWS"
    "token de GitHub"
    "JWT"
)
declare -a PATRONES=(
    '-----BEGIN [A-Z ]*PRIVATE KEY'
    # Una cuenta de servicio real lleva la llave en PEM como valor; exigirlo evita
    # que cuente como hallazgo la documentacion que nombra el campo (F2, #220).
    '"private_key"[[:space:]]*:[[:space:]]*"-----BEGIN'
    'AIza[0-9A-Za-z_-]{35}'
    'xkeysib-[0-9a-f]{20}'
    'AKIA[0-9A-Z]{16}'
    '(ghp_[0-9A-Za-z]{36}|github_pat_[0-9A-Za-z_]{20})'
    'eyJ[A-Za-z0-9_-]{30,}\.[A-Za-z0-9_-]{20,}'
)

# Ficheros que no deben estar versionados nunca.
declare -a PROHIBIDOS=(
    ".env" ".env.local" ".env.production"
    "terraform.tfstate" "terraform.tfstate.backup"
    "application_default_credentials.json" "sa-key.json" "credentials.json"
)

emitir() {
    if [[ -n "${SALIDA}" ]]; then
        echo "$*" | tee -a "${SALIDA}"
    else
        echo "$*"
    fi
}

[[ -z "${SALIDA}" ]] || : >"${SALIDA}"

emitir "Barrido de patrones de secreto (B4, issue #203)"
emitir "=============================================="
emitir "Fecha      : $(date -u +%Y-%m-%dT%H:%M:%SZ)"
emitir "Commit      : $(git rev-parse HEAD)"
emitir "Commits     : $(git rev-list --all | wc -l) en todas las ramas"
emitir "Objetos     : $(git rev-list --all --objects | wc -l)"
emitir ""

# --------------------------------------------------------------- 1. el arbol
emitir "1) Patrones de secreto en el arbol de trabajo"
emitir ""
for i in "${!PATRONES[@]}"; do
    # `git grep -I` salta binarios; se busca solo en lo versionado, que es lo que
    # se publica.
    if encontrado="$(git grep -nIE -e "${PATRONES[$i]}" -- . 2>/dev/null)"; then
        emitir "   HALLAZGO  ${NOMBRES[$i]}"
        emitir "${encontrado}"
        HALLAZGOS=$((HALLAZGOS + 1))
    else
        emitir "   0 coincidencias  ${NOMBRES[$i]}"
    fi
done
emitir ""

# ---------------------------------------------------- 2. contrasenas con valor
# Aparte de los prefijos, porque una contrasena no tiene forma reconocible. Se
# excluyen las referencias a variables, las plantillas de ejemplo y la prosa.
emitir "2) Contrasenas o claves asignadas con un valor literal"
emitir ""
if asignadas="$(git grep -nIE -e '(PASSWORD|SECRET_KEY|ACCESS_KEY|API_KEY)[[:space:]]*=[[:space:]]*["\x27]?[A-Za-z0-9/+=_@.-]{8,}' -- . 2>/dev/null |
    grep -viE 'example|\.md:|_test|getenv|os\.environ|\$\{|<[A-Z_]+>|=[[:space:]]*$|variable|description|trimspace|RELLENO')"; then
    emitir "   HALLAZGO"
    emitir "${asignadas}"
    HALLAZGOS=$((HALLAZGOS + 1))
else
    emitir "   0 coincidencias"
fi
emitir ""

emitir "3) Contrasenas dentro de una URI de conexion"
emitir ""
# Aqui NO vale listar coincidencias: el repositorio tiene a proposito
# credenciales de desarrollo en `docker-compose.yml`, y un barrido que las
# imprime cada vez no distingue «no hay secretos» de «hay ruido que nadie lee».
#
# Asi que esto **afirma** algo: toda contrasena que aparezca en una URI de
# conexion tiene que ser una de las de desarrollo declaradas abajo. Si aparece
# cualquier otra, es un hallazgo.
#
# Lo que hace inofensivas a las declaradas no es esta lista, es que
# `internal/config/validate.go` **rechaza el arranque** con APP_ENV=production si
# sobreviven a un despliegue. La seccion 6 lo comprueba.
declare -a FIJAS_DESARROLLO=(
    "moocpassword"   # docker-compose.yml, constante localDBPassword de validate.go
    "postgres"       # valor por omision de config.go para arrancar en local
    "UnaContrasena"  # fixture de validate_test.go, con sus variantes
    "a?b&c=d"        # fixture de TestValidateNoSeConfundeConUnaContrasenaConSimbolos
)

patron_uri='(postgres|postgresql|redis|rediss|amqp|mongodb)://[A-Za-z0-9_.-]+:[^@/[:space:]]{4,}@'

# Se excluyen las referencias a variable --${VAR} y $VAR-- y los marcadores:
# un hueco a rellenar en mayusculas (CONTRASENA, <PASSWORD>) no es una
# contrasena, y confundirlo con una convierte el barrido en ruido.
desconocidas="$(git grep -hoIE -e "${patron_uri}" -- . 2>/dev/null |
    sed -E 's#^[a-z]+://[A-Za-z0-9_.-]+:##; s#@$##' |
    grep -vE '^\$' |
    grep -vE '^\*+$' |
    grep -vE '^<.*>$' |
    grep -vE '^[A-Z_]{4,}$' |
    sort -u || true)"

restantes=""
while IFS= read -r clave; do
    [[ -n "${clave}" ]] || continue
    conocida=0
    for fija in "${FIJAS_DESARROLLO[@]}"; do
        [[ "${clave}" == "${fija}"* ]] && conocida=1 && break
    done
    [[ "${conocida}" -eq 1 ]] || restantes+="${clave}"$'\n'
done <<<"${desconocidas}"

emitir "   contrasenas distintas encontradas en URIs: $(printf '%s' "${desconocidas}" | grep -c . || true)"
for fija in "${FIJAS_DESARROLLO[@]}"; do
    emitir "   declarada de desarrollo: ${fija}"
done

if [[ -n "${restantes//[[:space:]]/}" ]]; then
    emitir "   HALLAZGO: contrasenas que NO estan declaradas como de desarrollo"
    while IFS= read -r r; do
        [[ -n "${r}" ]] && emitir "      ${r:0:4}... ($(printf '%s' "${r}" | wc -c) caracteres)"
    done <<<"${restantes}"
    HALLAZGOS=$((HALLAZGOS + 1))
else
    emitir "   0 contrasenas sin declarar"
fi
emitir ""

# ------------------------------------------------------------ 4. el historial
emitir "4) Los mismos patrones en TODO el historial"
emitir ""
emitir "   Metodo: el contenido de cada diff de \`git log -p --all --full-history\`,"
emitir "   que cubre lo que alguna vez se anadio aunque despues se borrara."
emitir ""
HISTORIAL="$(git log -p --all --full-history 2>/dev/null || true)"
for i in "${!PATRONES[@]}"; do
    n="$(printf '%s' "${HISTORIAL}" | grep -acE -e "${PATRONES[$i]}" || true)"
    if [[ "${n}" -gt 0 ]]; then
        emitir "   HALLAZGO (${n})  ${NOMBRES[$i]}"
        printf '%s' "${HISTORIAL}" | grep -aoE -e "${PATRONES[$i]}" | sort -u | head -n 5 |
            while read -r m; do emitir "      ${m:0:24}..."; done
        HALLAZGOS=$((HALLAZGOS + 1))
    else
        emitir "   0 coincidencias  ${NOMBRES[$i]}"
    fi
done
emitir ""

# ------------------------------------------------- 5. ficheros nunca versionados
emitir "5) Ficheros sensibles: alguna vez versionados?"
emitir ""
for f in "${PROHIBIDOS[@]}"; do
    n="$(git log --all --oneline --full-history -- "${f}" 2>/dev/null | wc -l)"
    if [[ "${n}" -eq 0 ]]; then
        emitir "$(printf '   %-42s nunca versionado' "${f}")"
    else
        emitir "$(printf '   %-42s APARECE en %s commit(s)' "${f}" "${n}")"
        HALLAZGOS=$((HALLAZGOS + 1))
    fi
done
emitir ""

# --------------------------------------------------------- 6. el validador
# La garantia de verdad no es que las credenciales de desarrollo no esten en el
# repositorio --estan, a proposito-- sino que **el arranque en produccion las
# rechaza**. Eso no se puede olvidar, a diferencia de una convencion.
emitir "6) El arranque en produccion rechaza las credenciales de desarrollo"
emitir ""
faltan_constantes=0
for c in localDBPassword localS3Secret localMailHost; do
    if grep -q "${c}" internal/config/validate.go 2>/dev/null; then
        emitir "   ok     internal/config/validate.go declara ${c}"
    else
        emitir "   HALLAZGO  internal/config/validate.go ya no declara ${c}"
        faltan_constantes=1
    fi
done
[[ "${faltan_constantes}" -eq 0 ]] || HALLAZGOS=$((HALLAZGOS + 1))

if command -v go >/dev/null 2>&1; then
    if salida="$(go test ./internal/config/ -run 'TestValidateRechazaValoresDeDesarrolloEnProduccion|TestValidateNoEstorbaEnDesarrollo' -count=1 2>&1)"; then
        emitir "   ok     el test de rechazo pasa"
        emitir "$(printf '%s' "${salida}" | tail -n 2 | sed 's/^/          /')"
    else
        emitir "   HALLAZGO  el test de rechazo NO pasa"
        emitir "$(printf '%s' "${salida}" | tail -n 15 | sed 's/^/          /')"
        HALLAZGOS=$((HALLAZGOS + 1))
    fi
else
    emitir "   n/d    go no esta disponible aqui; ejecutar el test antes de publicar"
fi
emitir ""

emitir "=============================================="
if [[ "${HALLAZGOS}" -eq 0 ]]; then
    emitir "RESULTADO: 0 hallazgos."
    exit 0
fi
emitir "RESULTADO: ${HALLAZGOS} grupo(s) de hallazgos. Revisar antes de publicar."
exit 1
