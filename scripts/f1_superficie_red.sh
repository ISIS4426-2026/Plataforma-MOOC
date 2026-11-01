#!/usr/bin/env bash
# Superficie de red y accesos minimos del despliegue elastico (F1, issue #219).
#
# Es la version de la Entrega 3 de scripts/verify_g4.sh, con una diferencia que importa: aquel IMPRIMIA y
# este AFIRMA. Cada control termina en OK o FALLA, y el guion sale con 1 si hay alguna FALLA (nota 36: un
# comprobador que imprime no comprueba).
#
# SOLO LECTURA. No crea, cambia ni borra nada. Retirar reglas sobrantes es otro guion
# (scripts/f1_retirar_reglas_sobrantes.sh), con confirmacion, porque borrar es irreversible.
#
# SE CORRE DESDE EL PORTATIL, con el entorno ENCENDIDO: el barrido tiene que salir de internet, no de
# dentro de la VPC, y con las reglas de reenvio del balanceador creadas (lb_enabled=true). Un barrido del
# entorno aparcado no prueba nada: no habria nada escuchando.
#
# CONTROLES (un archivo por control en docs/entrega3/evidencias/F1/):
#
#   a  barrido externo: que direcciones publicas tiene el proyecto, y que puertos responden en cada una.
#      Solo el balanceador puede tener IP publica, y solo con 80 (redirige) y 443.
#   b  inventario de reglas de cortafuegos: TODAS las del proyecto, en todas las redes, cada una con su
#      razon. Una regla que no esta en el inventario esperado es una sobrante (Entrega 2) y FALLA. Una
#      que esta pero con mas puertos u origenes de los declarados tambien FALLA: se compara el contenido,
#      no el nombre.
#   c  acceso a las instancias de la API: sin IP externa, con las etiquetas y la cuenta esperadas, y las
#      reglas que de verdad las alcanzan son solo balanceador/salud (80) e IAP (22). Base, cola y workers
#      sin superficie publica.
#   d  controles heredados de la Entrega 2, repetidos sobre la arquitectura nueva: HTTP -> HTTPS, TLS
#      minimo, CSRF por origen, Cloud SQL solo privada y cifrada, buckets publico/privado en los dos
#      sentidos.
#
# USO
#
#   bash ./scripts/f1_superficie_red.sh                       # escribe en docs/entrega3/evidencias/F1/
#   bash ./scripts/f1_superficie_red.sh <directorio-salida>
#
# Variables: PROYECTO (plataforma-mooc-entrega2), LB_IP (por omision, terraform output -raw lb_ip),
#            PUERTOS (lista a barrer), MEDIA_BUCKET, HLS_BUCKET
#
# Requiere gcloud con sesion (gcloud auth login), curl y python. nmap es opcional: si esta, el barrido lo
# hace nmap ademas del sondeo con /dev/tcp. No escribe correos de personas: solo conteos.
set -uo pipefail

PROYECTO="${PROYECTO:-plataforma-mooc-entrega2}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
OUT="${1:-${ROOT}/docs/entrega3/evidencias/F1}"
MEDIA_BUCKET="${MEDIA_BUCKET:-${PROYECTO}-media}"
HLS_BUCKET="${HLS_BUCKET:-${PROYECTO}-hls}"
# Puertos de aplicacion, de datos y de administracion que tuvo o podria tener esta plataforma.
PUERTOS="${PUERTOS:-21 22 23 25 80 443 3000 3306 5432 5672 6379 8000 8080 8443 9000 9090 9100 9200 27017}"
# Lo unico que puede responder: el 80 (que redirige) y el 443 del balanceador.
LB_ABIERTOS_ESPERADOS="${LB_ABIERTOS_ESPERADOS:-80 443}"

if command -v curl.exe >/dev/null 2>&1; then CURL=curl.exe; else CURL=curl; fi
PY=python; command -v python >/dev/null 2>&1 && python -c 'import sys; sys.exit(sys.version_info[0] < 3)' 2>/dev/null || PY=python3

if [ -z "${LB_IP:-}" ]; then
    LB_IP="$(cd "${ROOT}/infra/terraform" && terraform output -raw lb_ip 2>/dev/null | tr -d '\r')" || LB_IP=""
fi
[ -n "${LB_IP}" ] || { echo "ERROR: no se pudo leer la IP del balanceador. Pasa LB_IP=..." >&2; exit 2; }
BASE="https://${LB_IP}.sslip.io"

mkdir -p "${OUT}"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

RESUMEN="${OUT}/resumen.txt"
FALLAS=0
OBSERVACIONES=0
: >"${RESUMEN}"

ahora() { date -u +%Y-%m-%dT%H:%M:%SZ; }
cabecera() { # archivo titulo
    {
        echo "# $2"
        echo "# Generado: $(ahora) por scripts/f1_superficie_red.sh (solo lectura), desde $(uname -s) fuera de la VPC"
        echo "# Proyecto: ${PROYECTO}   Balanceador: ${BASE}"
        echo
    } >"$1"
}
# veredicto archivo OK|FALLA|OBS texto
veredicto() {
    local f="$1" v="$2" t="$3"
    printf '%-6s %s\n' "$v" "$t" | tee -a "$f" >>"${RESUMEN}"
    case "$v" in FALLA) FALLAS=$((FALLAS + 1)) ;; OBS) OBSERVACIONES=$((OBSERVACIONES + 1)) ;; esac
    return 0
}

json() { # nombre comando...  -> ${TMP}/nombre.json; falla ruidosamente si gcloud falla
    local n="$1"; shift
    if ! "$@" --project="${PROYECTO}" --format=json >"${TMP}/$n.json" 2>"${TMP}/$n.err"; then
        echo "ERROR: fallo: $* --project=${PROYECTO}" >&2
        sed 's/^/    /' "${TMP}/$n.err" >&2
        exit 2
    fi
}

echo "F1 - superficie de red y accesos minimos (${PROYECTO}, ${BASE})"
gcloud config get-value account >/dev/null 2>&1 || { echo "ERROR: sin sesion de gcloud" >&2; exit 2; }

# Todo lo que se consulta, de una vez. Si una consulta falla, se para: un control sin datos no es un OK.
json instancias    gcloud compute instances list
json reglas        gcloud compute firewall-rules list
json redes         gcloud compute networks list
json direcciones   gcloud compute addresses list
json reenvio       gcloud compute forwarding-rules list
json sql           gcloud sql instances list
json iam           gcloud projects get-iam-policy "${PROYECTO}"

# ============================================================================ a) barrido externo
f="${OUT}/a_barrido_externo.txt"
cabecera "$f" "a) Direcciones publicas del proyecto y barrido de puertos desde internet"

"$PY" - "${TMP}" "${LB_IP}" >"${TMP}/publicas.txt" <<'PYEOF'
import json, sys, os
tmp, lb = sys.argv[1], sys.argv[2]
cargar = lambda n: json.load(open(os.path.join(tmp, n + ".json")))
publicas = []
for i in cargar("instancias"):
    for ni in i.get("networkInterfaces", []):
        for ac in ni.get("accessConfigs", []) or []:
            if ac.get("natIP"):
                publicas.append(("instancia", i["name"], ac["natIP"]))
for d in cargar("direcciones"):
    if d.get("addressType", "EXTERNAL") == "EXTERNAL":
        publicas.append(("direccion reservada", d["name"], d.get("address", "")))
for r in cargar("reenvio"):
    if r.get("loadBalancingScheme", "").startswith("EXTERNAL"):
        publicas.append(("regla de reenvio " + r.get("portRange", ""), r["name"], r.get("IPAddress", "")))
for s in cargar("sql"):
    for ip in s.get("ipAddresses", []):
        if ip.get("type") == "PRIMARY":
            publicas.append(("cloud sql publica", s["name"], ip.get("ipAddress", "")))
for t, n, ip in publicas:
    print(f"{ip}\t{t}\t{n}\t{'balanceador' if ip == lb else 'OTRA'}")
PYEOF

{
    echo "== Direcciones publicas encontradas en el proyecto (instancias, reservadas, reglas de reenvio, Cloud SQL) =="
    printf 'IP\tQUE\tNOMBRE\tES\n'
    cat "${TMP}/publicas.txt"
    echo
} >>"$f"

OTRAS="$(awk -F'\t' '$4=="OTRA"{print $1}' "${TMP}/publicas.txt" | sort -u)"
if [ -z "${OTRAS}" ]; then
    veredicto "$f" OK "la unica direccion publica del proyecto es la del balanceador (${LB_IP})"
else
    veredicto "$f" FALLA "hay direcciones publicas que no son el balanceador: $(echo ${OTRAS})"
fi
REENVIO_LB="$(awk -F'\t' '$4=="balanceador" && $2 ~ /^regla de reenvio/' "${TMP}/publicas.txt" | wc -l | tr -d ' ')"
if [ "${REENVIO_LB}" -ge 1 ]; then
    veredicto "$f" OK "el balanceador tiene ${REENVIO_LB} regla(s) de reenvio: el entorno esta encendido y el barrido mide algo"
else
    veredicto "$f" FALLA "el balanceador no tiene reglas de reenvio: entorno aparcado, el barrido no prueba nada (lb_enabled=true)"
fi

barrer() { # ip -> lineas "puerto abierto|cerrado"
    local ip="$1" p
    for p in ${PUERTOS}; do
        if timeout 4 bash -c "echo > /dev/tcp/${ip}/${p}" 2>/dev/null; then echo "$p abierto"; else echo "$p cerrado"; fi
    done
}

for ip in ${LB_IP} ${OTRAS}; do
    {
        echo "== Barrido TCP de ${ip} desde este equipo (fuera de la VPC), $(ahora) =="
        echo "   puertos: ${PUERTOS}"
    } >>"$f"
    barrer "${ip}" >"${TMP}/barrido_${ip}.txt"
    sed 's/^/   puerto /' "${TMP}/barrido_${ip}.txt" >>"$f"
    if command -v nmap >/dev/null 2>&1; then
        echo "   -- nmap -Pn -sT, los mismos puertos --" >>"$f"
        nmap -Pn -sT -p "$(echo ${PUERTOS} | tr ' ' ',')" "${ip}" 2>&1 | grep -E "^[0-9]+/tcp|Nmap done" | sed 's/^/   /' >>"$f"
    fi
    echo >>"$f"
    ABIERTOS="$(awk '$2=="abierto"{print $1}' "${TMP}/barrido_${ip}.txt" | tr '\n' ' ' | sed 's/ $//')"
    if [ "${ip}" = "${LB_IP}" ]; then
        if [ "${ABIERTOS}" = "${LB_ABIERTOS_ESPERADOS}" ]; then
            veredicto "$f" OK "balanceador ${ip}: abiertos solo [${ABIERTOS}] de ${PUERTOS// /,}"
        else
            veredicto "$f" FALLA "balanceador ${ip}: abiertos [${ABIERTOS}], se esperaba exactamente [${LB_ABIERTOS_ESPERADOS}]"
        fi
    else
        if [ -z "${ABIERTOS}" ]; then
            veredicto "$f" OK "${ip}: ningun puerto abierto"
        else
            veredicto "$f" FALLA "${ip}: puertos abiertos desde internet [${ABIERTOS}]"
        fi
    fi
done

# ============================================================================ b) inventario de reglas
f="${OUT}/b_reglas_firewall.txt"
cabecera "$f" "b) Inventario de reglas de cortafuegos efectivas, con su razon"

# El inventario esperado ES el codigo de infraestructura: network.tf, queue.tf y worker_group.tf.
# Cualquier cambio legitimo en esas reglas tiene que venir con su cambio aqui, y viceversa.
"$PY" - "${TMP}" >"${TMP}/reglas.txt" <<'PYEOF'
import json, sys, os
tmp = sys.argv[1]
reglas = json.load(open(os.path.join(tmp, "reglas.json")))
redes = json.load(open(os.path.join(tmp, "redes.json")))
LB = ["130.211.0.0/22", "35.191.0.0/16"]
ESPERADAS = {
    # nombre: (red, direccion, permite, origen_rangos, origen_cuentas, destino_etiquetas, destino_cuentas, razon)
    "mooc-allow-ssh-iap": ("mooc-vpc", "INGRESS", ["tcp:22"], ["35.235.240.0/20"], [], ["allow-iap-ssh"], [],
        "administracion: SSH solo por el tunel de IAP de Google; no hay SSH desde internet (B2)"),
    "mooc-allow-lb-health": ("mooc-vpc", "INGRESS", ["tcp:80"], LB, [], ["mooc-api-node"], [],
        "balanceador externo global y sus verificaciones de salud hacia la API; unica entrada a la API (B2, C3)"),
    "mooc-allow-worker-health": ("mooc-vpc", "INGRESS", ["tcp:9090"], LB, [], ["mooc-worker-node"], [],
        "verificacion de salud del autohealing de workers (/readyz); los rangos son los de Google, no internet (C5)"),
    "mooc-allow-queue": ("mooc-vpc", "INGRESS", ["tcp:6379"], [], ["sa-web-server@", "sa-worker-server@"], [], ["sa-queue-server@"],
        "cola compartida: solo las cuentas de la API y de los workers hacia la cuenta de la cola (B1, B2)"),
    "mooc-allow-internal-icmp": ("mooc-vpc", "INGRESS", ["icmp"], ["10.0.1.0/24"], [], [], [],
        "diagnostico dentro de la subred; no abre ningun puerto de servicio (B2)"),
}
def permite(r):
    out = []
    for a in r.get("allowed", []) or []:
        ps = a.get("ports")
        out += [f"{a['IPProtocol']}:{p}" for p in ps] if ps else [a["IPProtocol"]]
    for a in r.get("denied", []) or []:
        out.append("DENY " + a["IPProtocol"] + ":" + ",".join(a.get("ports", ["todos"])))
    return sorted(out)
def prefijos(cuentas, esperadas):
    """Las cuentas llevan el proyecto en el dominio; se comparan por su prefijo (sa-xxx@)."""
    return len(cuentas) == len(esperadas) and all(any(c.startswith(e) for c in cuentas) for e in esperadas)
lineas = []
nombres = set()
for r in sorted(reglas, key=lambda r: (r["network"].rsplit("/", 1)[-1], r["name"])):
    n = r["name"]; nombres.add(n)
    red = r["network"].rsplit("/", 1)[-1]
    real = (red, r.get("direction"), permite(r), sorted(r.get("sourceRanges", []) or []),
            sorted(r.get("sourceServiceAccounts", []) or []), sorted(r.get("targetTags", []) or []),
            sorted(r.get("targetServiceAccounts", []) or []))
    desc = (f"{n} | red={red} {real[1]} permite={','.join(real[2])} origen={','.join(real[3] + real[4]) or '-'} "
            f"destino={','.join(real[5] + real[6]) or 'TODA LA RED'}{' DESACTIVADA' if r.get('disabled') else ''}")
    if n not in ESPERADAS:
        lineas.append(("SOBRANTE", n, red, desc, "no esta en el codigo de infraestructura de la Entrega 3"))
        continue
    e = ESPERADAS[n]
    ok = (real[0] == e[0] and real[1] == e[1] and real[2] == sorted(e[2]) and real[3] == sorted(e[3])
          and (prefijos(real[4], e[4]) if e[4] else not real[4]) and real[5] == sorted(e[5])
          and (prefijos(real[6], e[6]) if e[6] else not real[6]) and not r.get("disabled"))
    lineas.append(("ESPERADA" if ok else "DISTINTA", n, red, desc, e[7] if ok else "existe pero NO coincide con el codigo: " + e[7]))
for n in ESPERADAS:
    if n not in nombres:
        lineas.append(("FALTA", n, ESPERADAS[n][0], n, "declarada en el codigo y ausente en el proyecto"))
for estado, n, red, desc, razon in lineas:
    print(f"{estado}\t{n}\t{red}\t{desc}\t{razon}")
for red in redes:
    if red["name"] != "mooc-vpc":
        print(f"RED_SOBRANTE\t{red['name']}\t{red['name']}\tred {red['name']} ({len(red.get('subnetworks', []))} subredes)\t"
              "red que no usa el despliegue; hallazgo 2 de la Entrega 2")
PYEOF

{
    echo "== Inventario: todas las reglas del proyecto, en todas las redes =="
    echo
    awk -F'\t' '{printf "%-12s %s\n             razon: %s\n\n", $1, $4, $5}' "${TMP}/reglas.txt"
    echo "== Reglas con origen 0.0.0.0/0 en cualquier red =="
    "$PY" -c "
import json,sys
r=json.load(open(sys.argv[1]))
m=[x['name']+' ('+x['network'].rsplit('/',1)[-1]+')' for x in r if '0.0.0.0/0' in (x.get('sourceRanges') or []) and x.get('direction')=='INGRESS']
print('\n'.join(m) if m else 'ninguna')" "${TMP}/reglas.json"
    echo
} >>"$f"

N_ESP="$(awk -F'\t' '$1=="ESPERADA"' "${TMP}/reglas.txt" | wc -l | tr -d ' ')"
while IFS=$'\t' read -r estado n red _ razon; do
    case "$estado" in
        ESPERADA) ;;
        SOBRANTE) veredicto "$f" FALLA "regla sobrante ${n} (red ${red}): ${razon}" ;;
        DISTINTA) veredicto "$f" FALLA "regla ${n}: ${razon}" ;;
        FALTA) veredicto "$f" FALLA "regla ${n}: ${razon}" ;;
        RED_SOBRANTE) veredicto "$f" FALLA "red sobrante ${n}: ${razon}" ;;
    esac
done <"${TMP}/reglas.txt"
veredicto "$f" OK "${N_ESP} de 5 reglas esperadas existen y coinciden con el codigo, cada una con su razon"
if awk -F'\t' '$1=="SOBRANTE"||$1=="RED_SOBRANTE"' "${TMP}/reglas.txt" | grep -q .; then
    {
        echo
        echo "== Para retirarlas (revisar con el equipo antes; es irreversible) =="
        echo "   bash ./scripts/f1_retirar_reglas_sobrantes.sh"
    } >>"$f"
fi

# ============================================================================ c) acceso a la API y al resto
f="${OUT}/c_acceso_minimo.txt"
cabecera "$f" "c) Quien puede llegar a cada componente"

"$PY" - "${TMP}" >"${TMP}/acceso.txt" <<'PYEOF'
import json, sys, os
tmp = sys.argv[1]
cargar = lambda n: json.load(open(os.path.join(tmp, n + ".json")))
inst = cargar("instancias"); reglas = cargar("reglas"); sql = cargar("sql"); iam = cargar("iam")
def rol(i):
    n = i["name"]
    return "api" if n.startswith("mooc-api-") else "worker" if n.startswith("mooc-worker-") else \
           "cola" if n.startswith("mooc-queue") else "generador" if n.startswith("mooc-load-generator") else "otra"
CUENTA = {"api": "sa-web-server@", "worker": "sa-worker-server@", "cola": "sa-queue-server@", "generador": "sa-load-generator@"}
ETIQ = {"api": "mooc-api-node", "worker": "mooc-worker-node", "cola": "queue-server", "generador": "load-generator"}
def alcanza(regla, i):
    if regla.get("direction") != "INGRESS" or regla.get("disabled"):
        return False
    if not any(ni["network"].rsplit("/", 1)[-1] == regla["network"].rsplit("/", 1)[-1] for ni in i.get("networkInterfaces", [])):
        return False
    tt, ts = regla.get("targetTags"), regla.get("targetServiceAccounts")
    cuentas = [s["email"] for s in i.get("serviceAccounts", [])]
    if tt:
        return any(t in (i.get("tags", {}).get("items") or []) for t in tt)
    if ts:
        return any(c in cuentas for c in ts)
    return True
for i in sorted(inst, key=lambda x: x["name"]):
    r = rol(i)
    ext = [ac.get("natIP") for ni in i.get("networkInterfaces", []) for ac in (ni.get("accessConfigs") or []) if ac.get("natIP")]
    cuentas = [s["email"] for s in i.get("serviceAccounts", [])]
    tags = i.get("tags", {}).get("items") or []
    entran = sorted(x["name"] for x in reglas if alcanza(x, i))
    print("\t".join([i["name"], r, i.get("status", ""), ",".join(ext) or "-", ",".join(cuentas) or "-",
                     ",".join(tags) or "-", ",".join(entran) or "-",
                     "1" if r in CUENTA and any(c.startswith(CUENTA[r]) for c in cuentas) else "0",
                     "1" if r in ETIQ and ETIQ[r] in tags else "0"]))
for s in sql:
    ipc = s.get("settings", {}).get("ipConfiguration", {})
    print("\t".join(["SQL", s["name"], str(ipc.get("ipv4Enabled")), (ipc.get("privateNetwork") or "-").rsplit("/", 1)[-1],
                     str(ipc.get("sslMode") or ipc.get("requireSsl")), str(len(ipc.get("authorizedNetworks", []) or []))]))
iap = sum(1 for b in iam.get("bindings", []) if b["role"] == "roles/iap.tunnelResourceAccessor"
          for m in b["members"] if m.startswith(("user:", "group:")))
print(f"IAP\t{iap}")
PYEOF

{
    echo "== Instancias: IP externa, cuenta de servicio, etiquetas y reglas que las alcanzan =="
    printf '%-24s %-9s %-11s %-10s %s\n' NOMBRE ROL ESTADO IP_EXT "REGLAS DE ENTRADA QUE LA ALCANZAN"
    grep -vE "^(SQL|IAP)" "${TMP}/acceso.txt" | awk -F'\t' '{printf "%-24s %-9s %-11s %-10s %s\n", $1, $2, $3, $4, $7}'
    echo
    echo "== Cuenta de servicio y etiquetas por instancia =="
    grep -vE "^(SQL|IAP)" "${TMP}/acceso.txt" | awk -F'\t' '{printf "%-24s cuenta=%s  etiquetas=%s\n", $1, $5, $6}'
    echo
    echo "== Cloud SQL =="
    grep "^SQL" "${TMP}/acceso.txt" | awk -F'\t' '{printf "%s  ipv4_publica=%s  red_privada=%s  ssl=%s  redes_autorizadas=%s\n", $2, $3, $4, $5, $6}'
    echo
    echo "== Administracion autorizada: personas o grupos con roles/iap.tunnelResourceAccessor (solo conteo) =="
    grep "^IAP" "${TMP}/acceso.txt" | cut -f2
    echo
} >>"$f"

N_API=0
while IFS=$'\t' read -r nombre rol estado ext cuentas tags entran cuenta_ok etiq_ok; do
    case "${nombre}" in SQL | IAP) continue ;; esac
    [ "${ext}" = "-" ] && veredicto "$f" OK "${nombre} (${rol}): sin IP externa" ||
        veredicto "$f" FALLA "${nombre} (${rol}): tiene IP externa ${ext}"
    case "${rol}" in
        api)
            N_API=$((N_API + 1))
            [ "${entran}" = "mooc-allow-internal-icmp,mooc-allow-lb-health,mooc-allow-ssh-iap" ] &&
                veredicto "$f" OK "${nombre}: solo la alcanzan balanceador/salud (80), IAP (22) e ICMP interno" ||
                veredicto "$f" FALLA "${nombre}: la alcanzan [${entran}]; se esperaba lb-health, ssh-iap e icmp interno"
            ;;
        worker)
            [ "${entran}" = "mooc-allow-internal-icmp,mooc-allow-ssh-iap,mooc-allow-worker-health" ] &&
                veredicto "$f" OK "${nombre}: solo la alcanzan la salud del autohealing (9090), IAP (22) e ICMP interno" ||
                veredicto "$f" FALLA "${nombre}: la alcanzan [${entran}]"
            ;;
        cola)
            [ "${entran}" = "mooc-allow-internal-icmp,mooc-allow-queue,mooc-allow-ssh-iap" ] &&
                veredicto "$f" OK "${nombre}: el 6379 solo desde las cuentas de API y workers; IAP (22) e ICMP interno" ||
                veredicto "$f" FALLA "${nombre}: la alcanzan [${entran}]"
            ;;
        generador)
            [ "${entran}" = "mooc-allow-internal-icmp,mooc-allow-ssh-iap" ] &&
                veredicto "$f" OK "${nombre}: no abre puertos; solo IAP (22) e ICMP interno (nota 49)" ||
                veredicto "$f" FALLA "${nombre}: la alcanzan [${entran}]"
            ;;
        *) veredicto "$f" FALLA "${nombre}: instancia que no pertenece a ningun componente del despliegue" ;;
    esac
    if [ "${rol}" != "otra" ]; then
        [ "${cuenta_ok}" = "1" ] && [ "${etiq_ok}" = "1" ] ||
            veredicto "$f" FALLA "${nombre}: cuenta (${cuentas}) o etiquetas (${tags}) no son las de su componente"
    fi
done <"${TMP}/acceso.txt"
[ "${N_API}" -ge 1 ] || veredicto "$f" FALLA "no hay instancias de API en marcha: el control de acceso a la API no se pudo comprobar"

while IFS=$'\t' read -r _ nombre ipv4 red ssl redes_aut; do
    if [ "${ipv4}" = "False" ] && [ "${red}" = "mooc-vpc" ] && [ "${redes_aut}" = "0" ]; then
        veredicto "$f" OK "Cloud SQL ${nombre}: sin IPv4 publica, solo por la red privada mooc-vpc, sin redes autorizadas"
    else
        veredicto "$f" FALLA "Cloud SQL ${nombre}: ipv4_publica=${ipv4} red=${red} redes_autorizadas=${redes_aut}"
    fi
    [ "${ssl}" = "ENCRYPTED_ONLY" ] || [ "${ssl}" = "TRUSTED_CLIENT_CERTIFICATE_REQUIRED" ] &&
        veredicto "$f" OK "Cloud SQL ${nombre}: conexion cifrada obligatoria (${ssl})" ||
        veredicto "$f" FALLA "Cloud SQL ${nombre}: modo SSL ${ssl}, se esperaba ENCRYPTED_ONLY"
done < <(grep "^SQL" "${TMP}/acceso.txt")

# ============================================================================ d) controles heredados
f="${OUT}/d_controles_entrega2.txt"
cabecera "$f" "d) Controles de la Entrega 2 repetidos sobre la arquitectura elastica"

codigo() { "${CURL}" -s -o /dev/null --max-time 20 -w '%{http_code}' "$@" 2>/dev/null; }

REDIR="$("${CURL}" -s -I --max-time 20 "http://${LB_IP}.sslip.io/" 2>/dev/null | tr -d '\r')"
echo "== HTTP en el 80 ==" >>"$f"
printf '%s\n' "${REDIR}" | grep -iE "^HTTP|^Location" | sed 's/^/   /' >>"$f"
if printf '%s' "${REDIR}" | grep -qE "^HTTP/[0-9.]+ 30[18]" && printf '%s' "${REDIR}" | grep -qi "^Location: https://"; then
    veredicto "$f" OK "el 80 redirige a HTTPS"
else
    veredicto "$f" FALLA "el 80 no redirige a HTTPS"
fi

echo "== HTTPS ==" >>"$f"
CAB="$("${CURL}" -s -I --max-time 20 "${BASE}/api/v1/ready" 2>/dev/null | tr -d '\r')"
printf '%s\n' "${CAB}" | grep -iE "^HTTP|^Strict-Transport|^X-Content-Type|^X-Frame|^Content-Security" | sed 's/^/   /' >>"$f"
printf '%s' "${CAB}" | grep -qE "^HTTP/[0-9.]+ 200" &&
    veredicto "$f" OK "el 443 sirve con un certificado valido (curl verifica la cadena)" ||
    veredicto "$f" FALLA "el 443 no responde 200 con certificado valido (certificado gestionado ACTIVE?)"
printf '%s' "${CAB}" | grep -qi "^Strict-Transport-Security" &&
    veredicto "$f" OK "HSTS presente" ||
    veredicto "$f" OBS "sin Strict-Transport-Security (observacion heredada de la Entrega 2, no es falla)"

TLS11="$("${CURL}" -s --max-time 15 --tls-max 1.1 -o /dev/null -w '%{http_code}' "${BASE}/api/v1/ready" 2>/dev/null)"
echo "   cliente limitado a TLS 1.1: HTTP ${TLS11:-000}" >>"$f"
[ "${TLS11:-000}" = "000" ] &&
    veredicto "$f" OK "TLS 1.1 rechazado" ||
    veredicto "$f" FALLA "un cliente TLS 1.1 obtuvo HTTP ${TLS11}"

echo "== CSRF: POST con cookie de sesion segun el origen ==" >>"$f"
C_AJENO="$(codigo -X POST -H "Origin: https://evil.example" -H "Cookie: __Host-mooc_session=falsa" "${BASE}/api/v1/auth/logout")"
C_SIN="$(codigo -X POST -H "Cookie: __Host-mooc_session=falsa" "${BASE}/api/v1/auth/logout")"
C_PROPIO="$(codigo -X POST -H "Origin: ${BASE}" -H "Cookie: __Host-mooc_session=falsa" "${BASE}/api/v1/auth/logout")"
printf '   origen ajeno: %s   sin origen: %s   origen propio: %s\n' "${C_AJENO}" "${C_SIN}" "${C_PROPIO}" >>"$f"
echo "   (403 = rechazado por CSRF; el propio pasa el filtro y falla despues porque la cookie es falsa)" >>"$f"
[ "${C_AJENO}" = "403" ] && [ "${C_SIN}" = "403" ] && [ "${C_PROPIO}" != "403" ] &&
    veredicto "$f" OK "CSRF: origen ajeno y ausente rechazados (403), el propio pasa el filtro (${C_PROPIO})" ||
    veredicto "$f" FALLA "CSRF: ajeno=${C_AJENO} sin=${C_SIN} propio=${C_PROPIO}"

echo "== Buckets: derivados publicos, originales privados, sin firmar ==" >>"$f"
HLS_OBJ="$(gcloud storage ls "gs://${HLS_BUCKET}/hls/**/*.m3u8" --project="${PROYECTO}" 2>/dev/null | head -1 | tr -d '\r')"
ORIG_OBJ="$(gcloud storage ls "gs://${MEDIA_BUCKET}/originals/**" --project="${PROYECTO}" 2>/dev/null | grep -v '/$' | head -1 | tr -d '\r')"
if [ -n "${HLS_OBJ}" ]; then
    c="$(codigo "https://storage.googleapis.com/${HLS_OBJ#gs://}")"
    echo "   leer manifiesto HLS sin firmar: ${c}" >>"$f"
    [ "$c" = "200" ] && veredicto "$f" OK "derivado HLS legible sin firmar (por diseno)" ||
        veredicto "$f" FALLA "derivado HLS sin firmar devolvio ${c}"
else
    veredicto "$f" OBS "no hay ningun manifiesto HLS en gs://${HLS_BUCKET}: lectura publica no comprobada"
fi
if [ -n "${ORIG_OBJ}" ]; then
    c="$(codigo "https://storage.googleapis.com/${ORIG_OBJ#gs://}")"
    echo "   leer original sin firmar: ${c}" >>"$f"
    [ "$c" = "403" ] || [ "$c" = "401" ] && veredicto "$f" OK "original NO legible sin firmar (${c})" ||
        veredicto "$f" FALLA "original sin firmar devolvio ${c}"
else
    veredicto "$f" OBS "no hay originales en gs://${MEDIA_BUCKET}/originals/: lectura privada no comprobada"
fi
for b in "${HLS_BUCKET}" "${MEDIA_BUCKET}"; do
    c="$(codigo -X PUT --data "x" "https://storage.googleapis.com/${b}/f1-prueba-anonima.txt")"
    echo "   escritura anonima en ${b}: ${c}" >>"$f"
    [ "$c" = "403" ] || [ "$c" = "401" ] && veredicto "$f" OK "escritura anonima rechazada en ${b} (${c})" ||
        veredicto "$f" FALLA "escritura anonima en ${b} devolvio ${c}"
done
c="$(codigo "https://storage.googleapis.com/storage/v1/b/${MEDIA_BUCKET}/o")"
echo "   listar ${MEDIA_BUCKET} sin firmar: ${c}" >>"$f"
[ "$c" = "403" ] || [ "$c" = "401" ] && veredicto "$f" OK "el bucket privado no se puede listar sin firmar (${c})" ||
    veredicto "$f" FALLA "listar el bucket privado sin firmar devolvio ${c}"

# ============================================================================ resumen
{
    echo
    echo "Total: ${FALLAS} falla(s), ${OBSERVACIONES} observacion(es). Generado $(ahora)."
    echo "Este archivo no contiene credenciales, llaves ni secretos."
} >>"${RESUMEN}"
echo
cat "${RESUMEN}"
echo
echo "Evidencia en: ${OUT}"
[ "${FALLAS}" -eq 0 ] || exit 1
exit 0
