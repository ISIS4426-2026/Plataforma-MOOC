#!/usr/bin/env bash
# Pre-vuelo de la linea base retrospectiva (issue #199, A7).
#
# POR QUE EXISTE
#
# A7 recrea el entorno de dos VMs de la Entrega 2 para medir la linea base
# contra la nube. Eso cuesta dinero y horas, y hay cuatro cosas que, si no estan
# en su sitio, no se descubren hasta estar a mitad de la campana:
#
#   1. que alguien haya aplicado ya Terraform de la Entrega 3 --entonces recrear
#      la Entrega 2 destruiria su trabajo--,
#   2. que las imagenes del codigo que se quiere medir no esten publicadas,
#   3. que el nombre de la instancia de base siga reservado por Google,
#   4. que los perfiles multimedia no sean los declarados, que es exactamente
#      como la Entrega 2 acabo midiendo clips de 8 segundos.
#
# Este guion comprueba las cuatro, **no cambia nada** y estima el coste. Es
# seguro ejecutarlo tantas veces como haga falta.
#
# USO
#
#   bash ./scripts/a7_preparar.sh
#
# Para que el plan de Terraform salga exacto, exporta antes la contrasena de la
# base. Sin ella el guion usa un valor de relleno y lo avisa:
#
#   export TF_VAR_db_password="$(gcloud secrets versions access latest \
#       --secret=db-password --project=plataforma-mooc-entrega2)"
#
# El procedimiento completo esta en docs/entrega3/evidencias/A7/RUNBOOK.md.

set -uo pipefail

PROYECTO="${PROYECTO:-plataforma-mooc-entrega2}"
ZONA="${ZONA:-us-east1-b}"
REGION="${REGION:-us-east1}"
REGISTRO="${REGION}-docker.pkg.dev/${PROYECTO}/mooc"
TAG_BASE="${TAG_BASE:-entrega-2}"
ARBOL="${ARBOL:-/tmp/a7-entrega2}"

RAIZ="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROBLEMAS=0
PENDIENTES=()

titulo() { printf '\n\033[1m== %s\033[0m\n' "$*"; }
ok() { printf '  ok     %s\n' "$*"; }
aviso() { printf '  AVISO  %s\n' "$*"; }
malo() {
    printf '  FALLA  %s\n' "$*"
    PROBLEMAS=$((PROBLEMAS + 1))
}
pendiente() {
    printf '  FALTA  %s\n' "$*"
    PENDIENTES+=("$*")
}

# --- 0. Herramientas -------------------------------------------------------
titulo "0. Herramientas"
for cmd in git terraform gcloud python; do
    if command -v "${cmd}" >/dev/null 2>&1; then
        ok "${cmd}"
    else
        malo "falta ${cmd}"
    fi
done
[[ ${PROBLEMAS} -eq 0 ]] || {
    echo
    echo "Sin las herramientas no se puede seguir."
    exit 1
}

# --- 1. Que codigo se va a medir ------------------------------------------
titulo "1. Las dos corridas y su codigo"
SHA_TAG="$(git -C "${RAIZ}" rev-parse "${TAG_BASE}^{commit}" 2>/dev/null)" || {
    malo "no existe el tag ${TAG_BASE}"
    exit 1
}
SHA_MAIN="$(git -C "${RAIZ}" rev-parse main)"
printf '  corrida 1 (linea base)   %s  tag %s\n' "${SHA_TAG}" "${TAG_BASE}"
printf '  corrida 2 (corregida)    %s  main\n' "${SHA_MAIN}"

# Las correcciones que separan las dos corridas. Es lo que G3 restara.
DIFF_APP="$(git -C "${RAIZ}" diff --shortstat "${SHA_TAG}..${SHA_MAIN}" -- cmd/ internal/ migrations/ 2>/dev/null)"
printf '  diferencia de aplicacion:%s\n' "${DIFF_APP:- ninguna}"
DIFF_INFRA="$(git -C "${RAIZ}" diff --name-only "${SHA_TAG}..${SHA_MAIN}" -- infra/terraform/ | tr '\n' ' ')"
printf '  diferencia de infra     : %s\n' "${DIFF_INFRA:-ninguna}"

# --- 2. Arbol de trabajo en el tag ----------------------------------------
titulo "2. Arbol de trabajo en ${TAG_BASE}"
# Si el directorio desaparecio --/tmp se limpia-- git sigue teniendolo
# registrado y `worktree add` falla con que la ruta ya esta en uso.
git -C "${RAIZ}" worktree prune 2>/dev/null
if [[ -d "${ARBOL}/.git" ]] || [[ -f "${ARBOL}/.git" ]]; then
    ACTUAL="$(git -C "${ARBOL}" rev-parse HEAD 2>/dev/null)"
    if [[ "${ACTUAL}" == "${SHA_TAG}" ]]; then
        ok "${ARBOL} ya esta en el tag"
    else
        malo "${ARBOL} existe pero apunta a ${ACTUAL:0:12}, no al tag"
    fi
else
    git -C "${RAIZ}" worktree add --detach "${ARBOL}" "${TAG_BASE}" >/dev/null 2>&1 &&
        ok "creado ${ARBOL}" || malo "no se pudo crear el arbol en ${ARBOL}"
fi

# --- 3. El plan de Terraform ----------------------------------------------
#
# La comprobacion que de verdad importa: **cero destrucciones**. Si el plan
# propone destruir algo, es que el estado ya tiene recursos de la Entrega 3 y
# recrear la Entrega 2 se llevaria por delante el trabajo de otro (nota 1).
titulo "3. Plan de Terraform desde el tag (solo lectura)"
RELLENO=0
if [[ -z "${TF_VAR_db_password:-}" ]]; then
    export TF_VAR_db_password="RELLENO-SOLO-PARA-PLAN"
    RELLENO=1
fi

if terraform -chdir="${ARBOL}/infra/terraform" init -input=false >/dev/null 2>&1; then
    ok "backend inicializado"
    PLAN="$(terraform -chdir="${ARBOL}/infra/terraform" plan \
        -lock=false -input=false -no-color 2>&1)"
    RESUMEN="$(grep -E '^Plan:|^No changes' <<<"${PLAN}" | tail -1)"
    printf '  %s\n' "${RESUMEN:-no se pudo leer el resumen del plan}"

    DESTRUIR="$(grep -oE '[0-9]+ to destroy' <<<"${RESUMEN}" | grep -oE '^[0-9]+')"
    if [[ -n "${DESTRUIR}" && "${DESTRUIR}" != "0" ]]; then
        malo "el plan destruiria ${DESTRUIR} recurso(s): el estado ya no es el de la Entrega 2"
        echo "         Para antes de aplicar nada y mira que recursos son:"
        echo "         terraform -chdir=${ARBOL}/infra/terraform plan"
    else
        ok "ninguna destruccion propuesta"
    fi
    grep -E 'will be created' <<<"${PLAN}" | sed 's/^  # /         crea   /'
    [[ ${RELLENO} -eq 1 ]] &&
        aviso "sin TF_VAR_db_password: la linea de google_sql_user del plan es indicativa"
else
    malo "terraform init fallo (credenciales de gcloud o acceso al bucket del estado)"
fi

# --- 4. Imagenes publicadas -----------------------------------------------
#
# prepare_web_env.sh y prepare_worker_env.sh exigen IMAGE_TAG con el SHA
# completo de 40 caracteres, asi que cada corrida necesita SU imagen publicada.
# Una imagen de otro commit mide otro codigo.
titulo "4. Imagenes en Artifact Registry"
for par in "corrida 1 (tag):${SHA_TAG}" "corrida 2 (main):${SHA_MAIN}"; do
    etiqueta="${par%%:*}"
    sha="${par##*:}"
    for img in api worker; do
        if gcloud artifacts docker images describe "${REGISTRO}/${img}:${sha}" \
            --project "${PROYECTO}" >/dev/null 2>&1; then
            ok "${etiqueta} ${img}"
        else
            pendiente "${etiqueta} ${img}: publicar ${img}:${sha:0:12}"
        fi
    done
done

# --- 5. Estado del entorno -------------------------------------------------
titulo "5. Estado actual del entorno"
while read -r nombre estado; do
    [[ -z "${nombre}" ]] && continue
    case "${estado}" in
    RUNNING) aviso "${nombre} esta ENCENDIDA: ya esta costando" ;;
    TERMINATED) ok "${nombre} apagada (hay que encenderla: Terraform no gestiona el encendido)" ;;
    *) aviso "${nombre} en estado ${estado}" ;;
    esac
    # `tr -d` por el retorno de carro: gcloud en Windows escribe CRLF, y sin
    # quitarlo el estado llega como "TERMINATED\r" y no coincide con nada. Es la
    # misma trampa que database.tf documenta para la contrasena de la base.
done < <(gcloud compute instances list --project "${PROYECTO}" \
    --format="value(name,status)" 2>/dev/null | tr -d '\r')

N_SQL="$(gcloud sql instances list --project "${PROYECTO}" --format="value(name)" 2>/dev/null | grep -c . || true)"
if [[ "${N_SQL}" -eq 0 ]]; then
    ok "no hay instancia de base: la crea el apply"
    # El nombre de una instancia borrada sigue reservado una semana. Si lo esta,
    # hay que aplicar con -var db_instance_generation=2, y eso cambia la IP
    # privada y el nombre de conexion que las VMs tienen escritos.
    BORRADO="$(gcloud logging read 'protoPayload.methodName="cloudsql.instances.delete"' \
        --project "${PROYECTO}" --limit=1 --freshness=30d \
        --format="value(timestamp)" 2>/dev/null | head -1)"
    if [[ -n "${BORRADO}" ]]; then
        DIAS="$(python -c "
import datetime as d, sys
t = '${BORRADO}'[:19]
print((d.datetime.utcnow() - d.datetime.fromisoformat(t)).days)
" 2>/dev/null)"
        if [[ -n "${DIAS}" && "${DIAS}" -lt 8 ]]; then
            malo "la base se borro hace ${DIAS} dia(s): el nombre sigue reservado"
            echo "         Aplica con -var db_instance_generation=<siguiente>"
        else
            ok "la base se borro hace ${DIAS:-?} dias: el nombre ya esta libre"
        fi
    fi
else
    aviso "ya hay ${N_SQL} instancia(s) de base: ya esta costando"
fi

gcloud secrets describe db-password --project "${PROYECTO}" >/dev/null 2>&1 &&
    ok "el secreto db-password existe (no se lee su valor aqui)" ||
    malo "falta el secreto db-password: sin el no hay contrasena que dar a Terraform"

# --- 6. Lo que exige A6 antes de medir ------------------------------------
titulo "6. Comprobaciones de A6 y A5"
if python "${RAIZ}/scripts/capacity_criterios.py" --autoprueba >/dev/null 2>&1; then
    ok "las reglas de medicion se cumplen (capacity_criterios.py)"
else
    malo "la autoprueba de los criterios de medicion falla: no se mide nada hasta arreglarla"
fi

if bash "${RAIZ}/scripts/generar_media_prueba.sh" --verificar >/dev/null 2>&1; then
    ok "los tres perfiles multimedia son los declarados"
else
    pendiente "los perfiles: bash ./scripts/generar_media_prueba.sh"
fi

# --- 7. Coste de la campana -----------------------------------------------
#
# Derivado de la tabla de I6 y de la nota tecnica 24. El transcodificado es lo
# que marca la duracion: con perfiles reales cuesta 1,2x la duracion del video,
# y el escenario 2 mueve 162 minutos de video.
titulo "7. Coste y duracion estimados de la campana"
python - <<'PY'
TARIFA_HORA = 0.169   # todo encendido, tabla de I6
EGRESO_GB = 0.12

fases = [
    ("Provisionar, encender, desplegar y sembrar", 1.5),
    ("Escenario 1, 6 niveles, codigo del tag", 1.3),
    ("Escenario 2, 5 niveles con perfiles reales", 2.2),
    ("Desplegar la imagen corregida y migrar", 0.5),
    ("Escenario 1, 6 niveles, codigo corregido", 1.3),
    ("Verificar el E2E pendiente y destruir", 0.7),
]
total = sum(h for _, h in fases)
print(f"  {'fase':<46} {'horas':>6}")
for nombre, h in fases:
    print(f"  {nombre:<46} {h:>6.1f}")
print(f"  {'TOTAL con el entorno encendido':<46} {total:>6.1f}")
print()
print(f"  Computo + base: {total:.1f} h x {TARIFA_HORA} USD/h = {total*TARIFA_HORA:.2f} USD")
print(f"  Egreso HLS acotado a 10 segmentos por estudiante: 3,55 GB = {3.55*EGRESO_GB:.2f} USD")
print(f"  Egreso HLS sin acotar (video entero)            : 22,1 GB = {22.1*EGRESO_GB:.2f} USD")
print()
print(f"  CAMPANA ACOTADA : {total*TARIFA_HORA + 3.55*EGRESO_GB:.2f} USD")
print(f"  CAMPANA COMPLETA: {total*TARIFA_HORA + 22.1*EGRESO_GB:.2f} USD  (el 6% del credito de 50)")
print()
print("  El escenario 2 son 162 minutos de video a 1,2x en 2 vCPU: 97 minutos")
print("  de transcodificado que no se pueden acelerar (nota tecnica 24).")
print("  Usa -max-segmentos 10 salvo que quieras medir el egreso completo.")
PY

# --- Resumen ---------------------------------------------------------------
titulo "Resumen"
if [[ ${#PENDIENTES[@]} -gt 0 ]]; then
    echo "  Antes de aplicar hay que resolver:"
    for p in "${PENDIENTES[@]}"; do echo "    - ${p}"; done
    echo
fi
if [[ ${PROBLEMAS} -gt 0 ]]; then
    echo "  ${PROBLEMAS} comprobacion(es) en rojo. No apliques nada todavia."
    exit 1
fi
if [[ ${#PENDIENTES[@]} -gt 0 ]]; then
    echo "  Nada en rojo, pero faltan los pendientes de arriba."
    exit 2
fi
echo "  Todo listo. Sigue docs/entrega3/evidencias/A7/RUNBOOK.md desde el paso 2."
