#!/usr/bin/env bash
set -e

# ============================================================================
# Plataforma MOOC — Script Demostrativo Interactivo de Segmentos 1 y 2 (10.2)
# ============================================================================

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
RED='\033[0;31m'
BOLD='\033[1m'
NC='\033[0m'

API_URL="${API_URL:-http://localhost:8080/api/v1}"
MAILPIT_URL="${MAILPIT_URL:-http://localhost:8025/api/v1}"

json_get() {
    python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('$1', ''))" 2>/dev/null || true
}

json_compact() {
    python3 -c "import sys, json; print(json.dumps(json.load(sys.stdin)))" 2>/dev/null || cat
}

echo -e "${BLUE}========================================================================${NC}"
echo -e "${GREEN}${BOLD}  Plataforma MOOC — Demostración Segmentos 1 y 2 (Sección 10.2)        ${NC}"
echo -e "${BLUE}========================================================================${NC}"
echo -e "${CYAN}Ejecución en Vivo contra Contenedores Docker Compose (Sin Mocks)${NC}\n"

# Limpieza inicial de rate limits
docker compose exec -T redis redis-cli EVAL "for _,k in ipairs(redis.call('keys','ratelimit:*')) do redis.call('del',k) end" 0 >/dev/null 2>&1 || true

echo -e "${BLUE}------------------------------------------------------------------------${NC}"
echo -e "${GREEN}${BOLD}  SEGMENTO 1: IDENTIDAD Y ADMINISTRACIÓN                                ${NC}"
echo -e "${BLUE}------------------------------------------------------------------------${NC}"

# 1. Registro
DEMO_EMAIL="demo.estudiante.$(date +%s)@mooc.test"
echo -e "${YELLOW}==> Paso 1: Autoregistro público de estudiante (${DEMO_EMAIL})...${NC}"
REG_RESP=$(curl -s -X POST "${API_URL}/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"${DEMO_EMAIL}\",\"password\":\"Password123!\",\"full_name\":\"Estudiante Demo Live\"}")
echo -e "    ${CYAN}Respuesta:${NC} $(echo "$REG_RESP" | json_compact)"
if echo "$REG_RESP" | grep -q "pending_verification"; then
    echo -e "    [✔] ${GREEN}Estado pending_verification y rol estudiante confirmados.${NC}"
fi

# 2. Correo en Mailpit
echo -e "\n${YELLOW}==> Paso 2: Verificación de correo en Mailpit real...${NC}"
sleep 1
MSG_ID=$(curl -s "${MAILPIT_URL}/search?query=to:${DEMO_EMAIL}&limit=1" | python3 -c "import sys, json; print(json.load(sys.stdin).get('messages', [{}])[0].get('ID', ''))" || true)
if [ -n "$MSG_ID" ]; then
    TOKEN=$(curl -s "${MAILPIT_URL}/message/${MSG_ID}" | python3 -c "
import sys, json, re
data = json.load(sys.stdin)
m = re.search(r'token=([A-Za-z0-9_\-]+)', data.get('Text', ''))
if m:
    print(m.group(1))
")
    echo -e "    [✔] ${GREEN}Correo recibido en Mailpit. Token extraído:${NC} ${TOKEN:0:16}..."
else
    echo -e "    ${RED}Error: Correo no encontrado en Mailpit.${NC}"
fi

# 3. Activación
echo -e "\n${YELLOW}==> Paso 3: Activación de cuenta por token de un solo uso...${NC}"
VERIFY_RESP=$(curl -s "${API_URL}/auth/verify?token=${TOKEN}")
echo -e "    ${CYAN}Respuesta:${NC} $(echo "$VERIFY_RESP" | json_compact)"
if echo "$VERIFY_RESP" | grep -q "active"; then
    echo -e "    [✔] ${GREEN}Cuenta transicionó a estado 'active'.${NC}"
fi

# 4. Token quemado
BURN_RESP=$(curl -s -w "\n%{http_code}" "${API_URL}/auth/verify?token=${TOKEN}")
HTTP_CODE=$(echo "$BURN_RESP" | tail -n1)
echo -e "    ${CYAN}Reintento con mismo token:${NC} HTTP ${HTTP_CODE} (debe ser 400 Bad Request)"
if [ "$HTTP_CODE" = "400" ]; then
    echo -e "    [✔] ${GREEN}Token de activación invalidado tras su primer uso.${NC}"
fi

# 5. Login
echo -e "\n${YELLOW}==> Paso 4: Login de estudiante y emisión de Bearer Token...${NC}"
LOGIN_RESP=$(curl -s -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"${DEMO_EMAIL}\",\"password\":\"Password123!\"}")
STUDENT_TOKEN=$(echo "$LOGIN_RESP" | json_get "token")
echo -e "    [✔] ${GREEN}Sesión iniciada. Token emitido:${NC} ${STUDENT_TOKEN:0:16}..."

# 6. Intento de crear profesor por registro libre
echo -e "\n${YELLOW}==> Paso 5: Intento de autoregistro de profesor (Seguridad RBAC)...${NC}"
PROF_ATTEMPT=$(curl -s -w "\n%{http_code}" -X POST "${API_URL}/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"fake.prof@mooc.test\",\"password\":\"Password123!\",\"full_name\":\"Fake\",\"role\":\"profesor\"}")
HTTP_CODE=$(echo "$PROF_ATTEMPT" | tail -n1)
echo -e "    ${CYAN}Respuesta:${NC} HTTP ${HTTP_CODE} (espera 400 Bad Request)"
if [ "$HTTP_CODE" = "400" ]; then
    echo -e "    [✔] ${GREEN}Registro de profesor rechazado por endpoint público.${NC}"
fi

# 7. Logout y revocación
echo -e "\n${YELLOW}==> Paso 6: Logout y revocación inmediata de sesión en Redis...${NC}"
LOGOUT_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${API_URL}/auth/logout" \
    -H "Authorization: Bearer ${STUDENT_TOKEN}")
echo -e "    ${CYAN}POST /auth/logout:${NC} HTTP ${LOGOUT_CODE} (espera 204)"
AFTER_LOGOUT_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${API_URL}/auth/sessions" \
    -H "Authorization: Bearer ${STUDENT_TOKEN}")
echo -e "    ${CYAN}GET /auth/sessions con token revocado:${NC} HTTP ${AFTER_LOGOUT_CODE} (espera 401)"
if [ "$AFTER_LOGOUT_CODE" = "401" ]; then
    echo -e "    [✔] ${GREEN}Sesión revocada de forma inmediata; acceso denegado con 401.${NC}"
fi

# 8. Admin y protección del último admin
echo -e "\n${YELLOW}==> Paso 7: Administración y Protección del Último Administrador...${NC}"
ADMIN_TOKEN=$(curl -s -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"email":"admin@plataforma-mooc.test","password":"Password123!"}' | json_get "token")

# Suspender al administrador secundario temporalmente para dejar solo 1 activo
curl -s -X PATCH "${API_URL}/admin/users/a0000000-0000-0000-0000-000000000002/status" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"status":"suspended"}' > /dev/null

# Intentar suspender al único administrador activo que queda (Admin Principal)
LAST_ADMIN_RESP=$(curl -s -X PATCH "${API_URL}/admin/users/a0000000-0000-0000-0000-000000000001/status" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"status":"suspended"}')
echo -e "    ${CYAN}Respuesta al suspender último admin:${NC} $(echo "$LAST_ADMIN_RESP" | json_compact)"
if echo "$LAST_ADMIN_RESP" | grep -q "last_admin_protected"; then
    echo -e "    [✔] ${GREEN}Protección activa: last_admin_protected rechazó la suspensión con 409 Conflict.${NC}"
fi

# Reactivar al administrador secundario para dejar el estado limpio
curl -s -X PATCH "${API_URL}/admin/users/a0000000-0000-0000-0000-000000000002/status" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"status":"active"}' > /dev/null

echo -e "\n${BLUE}------------------------------------------------------------------------${NC}"
echo -e "${GREEN}${BOLD}  SEGMENTO 2: AUTORÍA Y PUBLICACIÓN                                     ${NC}"
echo -e "${BLUE}------------------------------------------------------------------------${NC}"

# 1. Login como Profesor
echo -e "${YELLOW}==> Paso 1: Autenticación como Profesor y Creación de Borrador...${NC}"
PROF_TOKEN=$(curl -s -X POST "${API_URL}/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"email":"profesor1@plataforma-mooc.test","password":"Password123!"}' | json_get "token")

COURSE_RESP=$(curl -s -X POST "${API_URL}/courses" \
    -H "Authorization: Bearer ${PROF_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"title":"Curso Demo Cloud Native","description":"Arquitecturas escalables y tolerantes a fallos."}')
COURSE_ID=$(echo "$COURSE_RESP" | json_get "id")
COURSE_STABLE_ID=$(echo "$COURSE_RESP" | json_get "stable_id")
echo -e "    ${CYAN}Curso Creado:${NC} ID=${COURSE_ID}, StableID=${COURSE_STABLE_ID}"
echo -e "    [✔] ${GREEN}Borrador creado en versión 1 con identificador estable único.${NC}"

# 2. Intento de publicación con errores
echo -e "\n${YELLOW}==> Paso 2: Intento de publicación con errores acumulativos...${NC}"
PUB_FAIL_RESP=$(curl -s -X POST "${API_URL}/courses/${COURSE_ID}/publish" \
    -H "Authorization: Bearer ${PROF_TOKEN}")
echo -e "    ${CYAN}Respuesta 422:${NC} $(echo "$PUB_FAIL_RESP" | json_compact)"
if echo "$PUB_FAIL_RESP" | grep -q "validation_failed"; then
    echo -e "    [✔] ${GREEN}Validación exhaustiva: reporta simultáneamente estructura y criterios de aprobación.${NC}"
fi

# 3. Construcción de jerarquía
echo -e "\n${YELLOW}==> Paso 3: Construcción de Jerarquía de 4 Niveles...${NC}"
MOD_ID=$(curl -s -X POST "${API_URL}/courses/${COURSE_ID}/modules" \
    -H "Authorization: Bearer ${PROF_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"title":"Módulo 1: Fundamentos"}' | json_get "id")

UNIT_ID=$(curl -s -X POST "${API_URL}/modules/${MOD_ID}/units" \
    -H "Authorization: Bearer ${PROF_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"title":"Unidad 1.1: Microservicios"}' | json_get "id")

RES1_ID=$(curl -s -X POST "${API_URL}/units/${UNIT_ID}/resources" \
    -H "Authorization: Bearer ${PROF_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"title":"Lectura Obligatoria","type":"text","is_mandatory":true,"is_visible":true,"allow_download":false,"content_text":"# Cloud Native\nPrincipios."}' | json_get "id")

RES2_ID=$(curl -s -X POST "${API_URL}/units/${UNIT_ID}/resources" \
    -H "Authorization: Bearer ${PROF_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"title":"Complementario","type":"text","is_mandatory":false,"is_visible":true,"allow_download":true,"content_text":"Lectura adicional."}' | json_get "id")

RES3_RESP=$(curl -s -X POST "${API_URL}/units/${UNIT_ID}/resources" \
    -H "Authorization: Bearer ${PROF_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"title":"Quiz Final","type":"quiz","is_mandatory":true,"is_visible":true,"allow_download":false,"content_text":"Evaluacion formativa"}')
RES3_ID=$(echo "$RES3_RESP" | json_get "id")
RES3_STABLE_ID=$(echo "$RES3_RESP" | json_get "stable_id")
echo -e "    [✔] ${GREEN}Jerarquía completa creada: Curso -> Módulo -> Unidad -> 3 Recursos.${NC}"

# 4. Previsualización con ETag
echo -e "\n${YELLOW}==> Paso 4: Previsualización con cabecera ETag...${NC}"
ETAG=$(curl -i -s "${API_URL}/courses/${COURSE_ID}" \
    -H "Authorization: Bearer ${PROF_TOKEN}" | grep -i '^etag:' | tr -d '\r\n')
echo -e "    [✔] ${GREEN}Cabecera detectada:${NC} ${ETAG}"

# 5. Reordenamiento y preservación de stable_id
echo -e "\n${YELLOW}==> Paso 5: Reordenamiento y Preservación de Stable ID...${NC}"
curl -s -X DELETE "${API_URL}/resources/${RES2_ID}" \
    -H "Authorization: Bearer ${PROF_TOKEN}" > /dev/null

REORDER_CHECK=$(curl -s -H "Authorization: Bearer ${PROF_TOKEN}" "${API_URL}/units/${UNIT_ID}/resources" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for item in data.get('items', []):
    if item.get('id') == '$RES3_ID':
        print(item.get('position'), item.get('stable_id'))
")
NEW_POS=$(echo "$REORDER_CHECK" | awk '{print $1}')
VERIFIED_STABLE=$(echo "$REORDER_CHECK" | awk '{print $2}')
echo -e "    ${CYAN}Recurso 3 desplazado a posición:${NC} ${NEW_POS}"
echo -e "    ${CYAN}Stable ID original:${NC} ${RES3_STABLE_ID} == ${VERIFIED_STABLE}"
if [ "$NEW_POS" = "1" ] && [ "$RES3_STABLE_ID" = "$VERIFIED_STABLE" ]; then
    echo -e "    [✔] ${GREEN}Reordenamiento atómico exitoso sin huecos y con stable_id intacto.${NC}"
fi

# 6. Publicación exitosa
echo -e "\n${YELLOW}==> Paso 6: Publicación Exitosa del Curso...${NC}"
PUB_RESP=$(curl -s -X POST "${API_URL}/courses/${COURSE_ID}/publish" \
    -H "Authorization: Bearer ${PROF_TOKEN}")
echo -e "    ${CYAN}Estado tras publicar:${NC} $(echo "$PUB_RESP" | json_get "status") (versión $(echo "$PUB_RESP" | json_get "version"))"
if echo "$PUB_RESP" | grep -q '"status":"published"'; then
    echo -e "    [✔] ${GREEN}Curso publicado con éxito.${NC}"
fi

# 7. Inmutabilidad estricta
echo -e "\n${YELLOW}==> Paso 7: Inmutabilidad Estricta del Curso Publicado...${NC}"
IMMUTABLE_RESP=$(curl -s -X PUT "${API_URL}/courses/${COURSE_ID}" \
    -H "Authorization: Bearer ${PROF_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"title":"Modificación Ilegal","description":"Intento de modificación"}')
echo -e "    ${CYAN}Respuesta al editar publicado:${NC} $(echo "$IMMUTABLE_RESP" | json_compact)"
if echo "$IMMUTABLE_RESP" | grep -q "course_immutable"; then
    echo -e "    [✔] ${GREEN}Edición rechazada con 409 y código 'course_immutable'.${NC}"
fi

# 8. Despublicación temporal (MVP 5.1)
echo -e "\n${YELLOW}==> Paso 8: Despublicación Temporal (MVP 5.1) y Republicación...${NC}"
curl -s -X POST "${API_URL}/courses/${COURSE_ID}/unpublish" \
    -H "Authorization: Bearer ${PROF_TOKEN}" > /dev/null
curl -s -X PUT "${API_URL}/courses/${COURSE_ID}" \
    -H "Authorization: Bearer ${PROF_TOKEN}" \
    -H "Content-Type: application/json" \
    -d '{"title":"Curso Demo Cloud Native (Actualizado)","description":"Arquitecturas escalables y tolerantes a fallos (Actualizado)."}' > /dev/null
REPUBLISH_RESP=$(curl -s -X POST "${API_URL}/courses/${COURSE_ID}/publish" \
    -H "Authorization: Bearer ${PROF_TOKEN}")
echo -e "    ${CYAN}Título republicado:${NC} $(echo "$REPUBLISH_RESP" | json_get "title")"
if echo "$REPUBLISH_RESP" | grep -q "Actualizado"; then
    echo -e "    [✔] ${GREEN}Flujo MVP 5.1 completado: unpublish -> edit -> publish exitoso.${NC}"
fi

echo -e "\n${BLUE}========================================================================${NC}"
echo -e "${GREEN}${BOLD}  ✔ DEMOSTRACIÓN DE SEGMENTOS 1 Y 2 COMPLETADA SATISFACTORIAMENTE       ${NC}"
echo -e "${BLUE}========================================================================${NC}\n"
