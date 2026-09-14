#!/usr/bin/env bash
# ==============================================================================
# Script: generate_architecture_pdf.sh
# Propósito: Genera automáticamente el PDF formal del Informe de Arquitectura
#            de Software (docs/INFORME_ARQUITECTURA.pdf) renderizando diagramas
#            Mermaid a alta resolución y aplicando estilos ejecutivos con WeasyPrint.
# ==============================================================================

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCS_DIR="${REPO_ROOT}/docs"
INPUT_MD="${DOCS_DIR}/INFORME_ARQUITECTURA.md"
OUTPUT_PDF="${DOCS_DIR}/INFORME_ARQUITECTURA.pdf"
BUILD_DIR="$(mktemp -d -t mooc_pdf_build_XXXXXX)"

trap 'rm -rf "${BUILD_DIR}"' EXIT

echo "==> [1/5] Preparando entorno de compilación de PDF..."
if [ ! -f "${INPUT_MD}" ]; then
    echo "ERROR: No se encontró el archivo ${INPUT_MD}" >&2
    exit 1
fi

cp "${INPUT_MD}" "${BUILD_DIR}/input.md"

echo "==> [2/5] Renderizando diagramas Mermaid a imágenes PNG de alta resolución..."
docker run --rm \
    -u "$(id -u):$(id -g)" \
    -v "${BUILD_DIR}:/data" \
    minlag/mermaid-cli:latest \
    -i /data/input.md \
    -o /data/processed.md \
    -e png \
    -s 2 >/dev/null 2>&1

echo "==> [3/5] Transformando Markdown a HTML ejecutivo con portada institucional..."
python3 - << PYEOF
import re
import subprocess
import os

build_dir = "${BUILD_DIR}"
processed_md = os.path.join(build_dir, "processed.md")

with open(processed_md, "r", encoding="utf-8") as f:
    content = f.read()

# Marcadores de bloques especiales de GitHub
content = re.sub(r'>\s*\[!CAUTION\]', '> **⚠️ ADVERTENCIA CRÍTICA / GUARDRAIL:**', content)
content = re.sub(r'>\s*\[!IMPORTANT\]', '> **📌 NOTA IMPORTANTE:**', content)
content = re.sub(r'>\s*\[!TIP\]', '> **💡 RECOMENDACIÓN TÉCNICA:**', content)
content = re.sub(r'>\s*\[!NOTE\]', '> **ℹ️ INFORMACIÓN:**', content)

# Títulos de figuras numeradas
figure_titles = [
    "Figura 1: Modelo Conceptual de Roles Globales y Dominios de Responsabilidad",
    "Figura 2: Árbol de Requerimientos de Atributos de Calidad (ASRs)",
    "Figura 3: Vista de Contexto del Sistema — C4 Nivel 1",
    "Figura 4: Vista de Contenedores y Topología de Red en Docker — C4 Nivel 2",
    "Figura 5: Vista de Componentes del Backend — Arquitectura Hexagonal C4 Nivel 3",
    "Figura 6: Vista de Datos — Modelo Entidad-Relación (ERD) en PostgreSQL 16",
    "Figura 7: Vista de Despliegue Físico y Topología de Runtime en Docker Compose",
    "Figura 8: Diagrama de Secuencia — Flujo A: Registro, Verificación SMTP y Sesión Revocable",
    "Figura 9: Diagrama de Secuencia — Flujo B: Autoría de Cursos, Ordenamiento Atómico y Publicación",
    "Figura 10: Diagrama de Secuencia — Flujo C: Procesamiento Asíncrono, Idempotencia, Reintentos y DLQ",
    "Figura 11: Modelo de Seguridad Defensiva en Profundidad (5 Capas)",
    "Figura 12: Estrategia de Resiliencia, Recuperación y Cumplimiento de RPO/RTO",
    "Figura 13: Flujo de Telemetría y Correlación Transversal (OTel, Prometheus, slog)",
    "Figura 14: Hoja de Ruta (Roadmap) y Fases de Evolución Arquitectónica"
]

for idx, title in enumerate(figure_titles, start=1):
    content = content.replace(f'![diagram](./processed-{idx}.png)', f'![{title}](./processed-{idx}.png)')

# Portada formal corporativa / académica
cover_html = """
<div class="cover-page" id="informe-formal-de-arquitectura-de-software--plataforma-mooc">
  <div class="cover-header">
    <div class="cover-badge">LÍNEA BASE v1.0.0 — BASELINE DE ARQUITECTURA</div>
    <div class="cover-title">Informe Formal de<br/>Arquitectura de Software</div>
    <div class="cover-subtitle">Plataforma Web de Cursos Masivos Abiertos en Línea (MOOC)<br/>Monolito Modular en Go, Workers Asíncronos, Persistencia Relacional y Observabilidad</div>
  </div>
  
  <div class="cover-meta-grid">
    <div class="cover-meta-item"><span class="cover-meta-label">Proyecto:</span><span class="cover-meta-value">Plataforma MOOC (Massive Open Online Courses)</span></div>
    <div class="cover-meta-item"><span class="cover-meta-label">Programa / Asignatura:</span><span class="cover-meta-value">ISIS4426 — Arquitectura Cloud / Soluciones Software</span></div>
    <div class="cover-meta-item"><span class="cover-meta-label">Organización Operadora:</span><span class="cover-meta-value">Institución Operadora de Plataforma MOOC</span></div>
    <div class="cover-meta-item"><span class="cover-meta-label">Versión de Entrega:</span><span class="cover-meta-value">1.0.0 (MVP — Etapa Actual / Issues #9 a #30)</span></div>
    <div class="cover-meta-item"><span class="cover-meta-label">Fecha de Emisión:</span><span class="cover-meta-value">Septiembre 2026</span></div>
    <div class="cover-meta-item"><span class="cover-meta-label">Estado del Documento:</span><span class="cover-meta-value">Aprobado / Línea Base Técnica y Operativa</span></div>
    <div class="cover-meta-item"><span class="cover-meta-label">Equipo Responsable:</span><span class="cover-meta-value">Equipo de Arquitectura e Ingeniería MOOC</span></div>
  </div>

  <div class="cover-footer">
    <div>Confidencial — Uso Académico e Institucional</div>
    <div>Universidad de los Andes • Departamento de Ingeniería de Sistemas y Computación</div>
  </div>
</div>
"""

body_md = content
first_h1_match = re.search(r'^#\s+Informe Formal de Arquitectura de Software.*?---', content, re.DOTALL | re.MULTILINE)
if first_h1_match:
    body_md = content[first_h1_match.end():].strip()

temp_md = os.path.join(build_dir, "body.md")
with open(temp_md, "w", encoding="utf-8") as f:
    f.write(body_md)

temp_html = os.path.join(build_dir, "body.html")
subprocess.run(["pandoc", temp_md, "-f", "markdown", "-t", "html", "-o", temp_html], check=True)

with open(temp_html, "r", encoding="utf-8") as f:
    body_html = f.read()

body_html = re.sub(r'<blockquote>\s*<p><strong>⚠️ ADVERTENCIA CRÍTICA', '<blockquote class="caution-box"><p><strong>⚠️ ADVERTENCIA CRÍTICA', body_html)
body_html = re.sub(r'<blockquote>\s*<p><strong>📌 NOTA IMPORTANTE', '<blockquote class="important-box"><p><strong>📌 NOTA IMPORTANTE', body_html)

# Contenedor para la tabla de contenido
body_html = re.sub(
    r'(<h2 id="tabla-de-contenido">Tabla de Contenido</h2>\s*<ul>.*?</ul>)',
    r'<div class="toc-container">\1</div>',
    body_html,
    flags=re.DOTALL
)

# Corrección de enlaces internos del índice
anchor_fixes = {
    'href="#1-resumen-ejecutivo"': 'href="#resumen-ejecutivo"',
    'href="#2-contexto-de-negocio-objetivos-y-alcance"': 'href="#contexto-de-negocio-objetivos-y-alcance"',
    'href="#21-propósito-y-visión-del-sistema"': 'href="#propósito-y-visión-del-sistema"',
    'href="#22-roles-globales-y-modelo-de-actores"': 'href="#roles-globales-y-modelo-de-actores"',
    'href="#23-jerarquía-académica-y-tipos-de-recursos"': 'href="#jerarquía-académica-y-tipos-de-recursos"',
    'href="#24-guardrails-y-principios-inquebrantables-de-arquitectura"': 'href="#guardrails-y-principios-inquebrantables-de-arquitectura"',
    'href="#25-alcance-funcional-implementado-vs-proyectado"': 'href="#alcance-funcional-implementado-vs.-proyectado"',
    'href="#3-atributos-de-calidad-y-requerimientos-no-funcionales-asrs"': 'href="#atributos-de-calidad-y-requerimientos-no-funcionales-asrs"',
    'href="#31-escalabilidad-y-concurrencia"': 'href="#escalabilidad-y-concurrencia"',
    'href="#32-disponibilidad-y-resiliencia"': 'href="#disponibilidad-y-resiliencia"',
    'href="#33-seguridad-e-integridad-transaccional"': 'href="#seguridad-e-integridad-transaccional"',
    'href="#34-rendimiento-y-latencia"': 'href="#rendimiento-y-latencia"',
    'href="#35-observabilidad-y-auditabilidad"': 'href="#observabilidad-y-auditabilidad"',
    'href="#36-mantenibilidad-y-desacoplamiento"': 'href="#mantenibilidad-y-desacoplamiento"',
    'href="#4-registro-de-decisiones-de-arquitectura-adrs"': 'href="#registro-de-decisiones-de-arquitectura-adrs"',
    'href="#5-vistas-de-arquitectura-modelo-c4--isoiec-42010"': 'href="#vistas-de-arquitectura-modelo-c4-isoiec-42010"',
    'href="#51-vista-de-contexto-del-sistema-c4--nivel-1"': 'href="#vista-de-contexto-del-sistema-c4-nivel-1"',
    'href="#52-vista-de-contenedores-y-topología-c4--nivel-2"': 'href="#vista-de-contenedores-y-topología-c4-nivel-2"',
    'href="#53-vista-de-componentes-y-modularidad-del-backend-c4--nivel-3"': 'href="#vista-de-componentes-y-modularidad-del-backend-c4-nivel-3"',
    'href="#54-vista-de-información-y-datos-modelo-relacional-e-invariantes"': 'href="#vista-de-información-y-datos-modelo-relacional-e-invariantes"',
    'href="#55-vista-de-despliegue-e-infraestructura-física--runtime"': 'href="#vista-de-despliegue-e-infraestructura-física-runtime"',
    'href="#56-vista-dinámica-flujos-críticos-de-ejecución-secuencia"': 'href="#vista-dinámica-flujos-críticos-de-ejecución-secuencia"',
    'href="#6-estrategia-de-seguridad-gobierno-y-cumplimiento"': 'href="#estrategia-de-seguridad-gobierno-y-cumplimiento"',
    'href="#7-estrategia-de-resiliencia-concurrencia-y-recuperación-ante-desastres-drp"': 'href="#estrategia-de-resiliencia-concurrencia-y-recuperación-ante-desastres-drp"',
    'href="#8-estrategia-de-observabilidad-telemetría-y-monitoreo"': 'href="#estrategia-de-observabilidad-telemetría-y-monitoreo"',
    'href="#81-correlación-transversal-en-logs"': 'href="#correlación-transversal-en-logs"',
    'href="#82-métricas-prometheus-expuestas"': 'href="#métricas-prometheus-expuestas"',
    'href="#9-estado-actual-de-la-implementación-y-métricas-de-calidad"': 'href="#estado-actual-de-la-implementación-y-métricas-de-calidad"',
    'href="#91-matriz-de-trazabilidad-y-cumplimiento-de-especificación"': 'href="#matriz-de-trazabilidad-y-cumplimiento-de-especificación"',
    'href="#92-métricas-de-verificación-automatizada-aseguramiento-de-calidad"': 'href="#métricas-de-verificación-automatizada-aseguramiento-de-calidad"',
    'href="#10-plan-de-evolución-arquitectónica-y-roadmap-técnico"': 'href="#plan-de-evolución-arquitectónica-y-roadmap-técnico"',
    'href="#11-glosario-de-términos"': 'href="#glosario-de-términos"'
}

for k, v in anchor_fixes.items():
    body_html = body_html.replace(k, v)

# CSS formal de alta calidad para WeasyPrint
css_content = """
@page {
    size: A4;
    margin: 20mm 18mm 20mm 18mm;
    @top-left {
        content: "Plataforma MOOC — Informe Formal de Arquitectura de Software";
        font-family: "DejaVu Sans", "Noto Sans", sans-serif;
        font-size: 7pt;
        color: #64748b;
        font-weight: 500;
        border-bottom: 0.5pt solid #cbd5e1;
        padding-bottom: 2.5mm;
    }
    @top-right {
        content: "Línea Base v1.0.0 (MVP)";
        font-family: "DejaVu Sans", "Noto Sans", sans-serif;
        font-size: 7pt;
        color: #64748b;
        font-weight: 500;
        border-bottom: 0.5pt solid #cbd5e1;
        padding-bottom: 2.5mm;
    }
    @bottom-left {
        content: "ISIS4426 — Arquitectura Cloud / Desarrollo de Software";
        font-family: "DejaVu Sans", "Noto Sans", sans-serif;
        font-size: 7pt;
        color: #64748b;
        border-top: 0.5pt solid #cbd5e1;
        padding-top: 2.5mm;
    }
    @bottom-right {
        content: "Página " counter(page) " de " counter(pages);
        font-family: "DejaVu Sans", "Noto Sans", sans-serif;
        font-size: 7pt;
        color: #1e3a8a;
        font-weight: 700;
        border-top: 0.5pt solid #cbd5e1;
        padding-top: 2.5mm;
    }
}

@page:first {
    margin: 0;
    @top-left { content: normal; border: none; }
    @top-right { content: normal; border: none; }
    @bottom-left { content: normal; border: none; }
    @bottom-right { content: normal; border: none; }
}

body {
    font-family: "DejaVu Sans", "Noto Sans", "Helvetica Neue", sans-serif;
    font-size: 9pt;
    line-height: 1.5;
    color: #1e293b;
    margin: 0;
    padding: 0;
}

.cover-page {
    height: 255mm;
    background: linear-gradient(135deg, #0f172a 0%, #1e3a8a 60%, #1d4ed8 100%);
    color: #ffffff;
    padding: 25mm 20mm 20mm 20mm;
    box-sizing: border-box;
    page-break-after: always;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
}

.cover-header {
    border-bottom: 2px solid #60a5fa;
    padding-bottom: 16px;
}

.cover-badge {
    display: inline-block;
    background-color: #2563eb;
    color: #ffffff;
    padding: 4px 12px;
    border-radius: 16px;
    font-size: 8pt;
    font-weight: 700;
    letter-spacing: 0.8px;
    text-transform: uppercase;
    margin-bottom: 16px;
    border: 1px solid #93c5fd;
}

.cover-title {
    font-size: 24pt;
    font-weight: 800;
    line-height: 1.18;
    margin: 8px 0 12px 0;
    color: #ffffff;
}

.cover-subtitle {
    font-size: 12pt;
    font-weight: 400;
    color: #bfdbfe;
    line-height: 1.35;
    margin: 0;
}

.cover-meta-grid {
    background: rgba(255, 255, 255, 0.08);
    border: 1px solid rgba(255, 255, 255, 0.2);
    border-radius: 8px;
    padding: 16px 20px;
    margin-top: 25px;
}

.cover-meta-item {
    margin-bottom: 10px;
    font-size: 8.8pt;
}

.cover-meta-item:last-child {
    margin-bottom: 0;
}

.cover-meta-label {
    color: #93c5fd;
    font-weight: 600;
    display: inline-block;
    width: 160px;
}

.cover-meta-value {
    color: #ffffff;
    font-weight: 400;
}

.cover-footer {
    border-top: 1px solid rgba(255, 255, 255, 0.2);
    padding-top: 12px;
    font-size: 8pt;
    color: #94a3b8;
    display: flex;
    justify-content: space-between;
}

h1 {
    color: #0f172a;
    font-size: 15pt;
    font-weight: 800;
    border-bottom: 2pt solid #1e3a8a;
    padding-bottom: 4pt;
    margin-top: 22pt;
    margin-bottom: 10pt;
    page-break-before: always;
}

h2 {
    color: #1e3a8a;
    font-size: 12pt;
    font-weight: 700;
    border-bottom: 0.75pt solid #cbd5e1;
    padding-bottom: 3pt;
    margin-top: 15pt;
    margin-bottom: 7pt;
    page-break-after: avoid;
}

h3 {
    color: #1e40af;
    font-size: 10pt;
    font-weight: 700;
    margin-top: 12pt;
    margin-bottom: 5pt;
    page-break-after: avoid;
}

h4 {
    color: #334155;
    font-size: 9.2pt;
    font-weight: 600;
    margin-top: 10pt;
    margin-bottom: 4pt;
    page-break-after: avoid;
}

p {
    margin-top: 0;
    margin-bottom: 6pt;
    text-align: justify;
}

ul, ol {
    margin-top: 0;
    margin-bottom: 7pt;
    padding-left: 18px;
}

li {
    margin-bottom: 2.5pt;
}

table {
    width: 100%;
    border-collapse: collapse;
    margin: 10pt 0;
    font-size: 7.8pt;
    page-break-inside: avoid;
}

th, td {
    padding: 5pt 6.5pt;
    border: 0.5pt solid #cbd5e1;
    vertical-align: top;
}

th {
    background-color: #1e3a8a;
    color: #ffffff;
    font-weight: 700;
    text-align: left;
}

tr:nth-child(even) {
    background-color: #f8fafc;
}

pre {
    background-color: #f8fafc;
    border: 0.5pt solid #cbd5e1;
    border-radius: 4pt;
    padding: 7pt 9pt;
    font-family: "DejaVu Sans Mono", monospace;
    font-size: 7.2pt;
    line-height: 1.35;
    white-space: pre-wrap;
    word-break: break-all;
    page-break-inside: avoid;
    margin: 7pt 0;
}

code {
    font-family: "DejaVu Sans Mono", monospace;
    font-size: 7.6pt;
    background-color: #f1f5f9;
    color: #0f172a;
    padding: 1pt 3pt;
    border-radius: 2pt;
    border: 0.5pt solid #e2e8f0;
}

pre code {
    background: none;
    border: none;
    padding: 0;
    color: inherit;
}

blockquote {
    margin: 9pt 0;
    padding: 6pt 10pt;
    background-color: #f8fafc;
    border-left: 3pt solid #2563eb;
    border-radius: 0 4pt 4pt 0;
    font-size: 8.5pt;
    page-break-inside: avoid;
}

blockquote p {
    margin: 2pt 0;
}

.caution-box {
    border-left-color: #dc2626 !important;
    background-color: #fef2f2 !important;
}

.important-box {
    border-left-color: #d97706 !important;
    background-color: #fffbeb !important;
}

figure {
    margin: 12pt auto;
    text-align: center;
    page-break-inside: avoid;
}

figure img {
    display: block;
    max-width: 95%;
    max-height: 440px;
    height: auto;
    margin: 0 auto;
    border: 0.5pt solid #cbd5e1;
    border-radius: 4pt;
    padding: 5pt;
    background-color: #ffffff;
}

figcaption {
    text-align: center;
    font-size: 7.8pt;
    font-style: italic;
    color: #475569;
    margin-top: 4pt;
}

.toc-container {
    background-color: #f8fafc;
    border: 0.5pt solid #cbd5e1;
    border-radius: 6pt;
    padding: 12pt 16pt;
    margin: 14pt 0;
    page-break-after: always;
}

.toc-container h2 {
    margin-top: 0;
    border-bottom: 1.5pt solid #1e3a8a;
    color: #1e3a8a;
}

.toc-container a {
    color: #1e3a8a;
    text-decoration: none;
}

.toc-container a:hover {
    text-decoration: underline;
}

hr {
    border: none;
    border-top: 0.5pt solid #cbd5e1;
    margin: 12pt 0;
}
"""

with open(os.path.join(build_dir, "style.css"), "w", encoding="utf-8") as f:
    f.write(css_content)

full_html = f"""<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="utf-8"/>
<title>Informe Formal de Arquitectura de Software — Plataforma MOOC</title>
<link rel="stylesheet" href="style.css"/>
</head>
<body>
{cover_html}
<div class="content-wrapper">
{body_html}
</div>
</body>
</html>
"""

with open(os.path.join(build_dir, "report.html"), "w", encoding="utf-8") as f:
    f.write(full_html)
PYEOF

echo "==> [4/5] Compilando PDF final con WeasyPrint en contenedor Docker..."
# Asegurar imagen local de WeasyPrint
if ! docker image inspect weasyprint-tool >/dev/null 2>&1; then
    echo "    (Construyendo imagen local weasyprint-tool...)"
    docker build -t weasyprint-tool - << 'DOCKEREOF' >/dev/null 2>&1
FROM alpine:latest
RUN apk add --no-cache weasyprint font-noto font-dejavu
WORKDIR /data
ENTRYPOINT ["weasyprint"]
DOCKEREOF
fi

docker run --rm \
    -u "$(id -u):$(id -g)" \
    -e XDG_CACHE_HOME=/tmp \
    -v "${BUILD_DIR}:/data" \
    weasyprint-tool \
    /data/report.html /data/output.pdf >/dev/null 2>&1

echo "==> [5/5] Copiando documento compilado a ${OUTPUT_PDF}..."
cp "${BUILD_DIR}/output.pdf" "${OUTPUT_PDF}"

PDF_PAGES=$(pdfinfo "${OUTPUT_PDF}" 2>/dev/null | awk '/^Pages:/ {print $2}' || echo "N/A")
PDF_SIZE=$(du -h "${OUTPUT_PDF}" | cut -f1)

echo "=============================================================================="
echo "✅ PDF Generado con éxito!"
echo "   Archivo: docs/INFORME_ARQUITECTURA.pdf"
echo "   Páginas: ${PDF_PAGES}"
echo "   Tamaño:  ${PDF_SIZE}"
echo "=============================================================================="

