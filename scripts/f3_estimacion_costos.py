#!/usr/bin/env python3
"""Estimacion de costos de la arquitectura elastica (F3, issue #221).

Calcula, con precios de lista de us-east1 declarados abajo con su fuente y fecha, el costo por hora y por mes
(24x7) de los dos escenarios que pide el issue -los dos grupos a 1 instancia y a 3 instancias cada uno-, el
costo de una sesion de pruebas con el generador encendido y el costo del entorno APARCADO.

Las cifras salen de aqui y no de una hoja a mano: si cambia un precio o un tamano, se cambia una linea y se
vuelve a generar. Escribe las tablas en Markdown por la salida estandar.

Uso:
  python scripts/f3_estimacion_costos.py                 # tablas
  python scripts/f3_estimacion_costos.py --autoprueba    # comprueba la aritmetica con casos conocidos

No usa la red ni credenciales.
"""
import sys

HORAS_MES = 730  # la misma ventana que la Entrega 2 (CONFIGURACION_Y_COSTOS.md, sec. 4)

# ---------------------------------------------------------------------------------------- precios (USD)
# Precios de lista, us-east1, sin descuentos por uso comprometido. Fecha de consulta: 2026-11-01.
P = {
    # Compute Engine: el mismo valor que la Entrega 2 (e2-highcpu-2 + 30 GiB = 39,11 USD/mes) y C1/G1.
    "e2-highcpu-2_h": 0.04947,
    "e2-small_h": 0.016751,
    "e2-standard-4_h": 0.134012,
    "pd_balanced_gib_mes": 0.10,
    # Cloud SQL Enterprise, cloud.google.com/sql/pricing
    "sql_vcpu_h": 0.0413,
    "sql_gib_ram_h": 0.007,
    "sql_ssd_gib_h": 0.000232877,
    "sql_backup_gib_h": 0.000109589,
    # Cloud Load Balancing, cloud.google.com/load-balancing/pricing
    # Tarifa minima global por el CONJUNTO de hasta 5 reglas, no por regla: lo confirma la factura del 2026-10-29
    # (SKU "Forwarding Rule Minimum Global": 13,23 h x 0,025 = 0,33 USD con las dos reglas activas). Ver nota 61.
    "reglas_reenvio_h": 0.025,
    "lb_datos_gib": 0.008,             # entrada y salida procesadas, por separado
    # Cloud NAT, cloud.google.com/nat/pricing
    "nat_vm_h": 0.0014,                # por VM que usa la pasarela, hasta 32
    "nat_datos_gib": 0.045,
    "nat_ip_h": 0.005,                 # por IP externa de la pasarela; con min_ports_per_vm = 2048 basta una (nota 57)
    # Direcciones IP, cloud.google.com/vpc/network-pricing
    "ip_reservada_sin_uso_h": 0.01,    # la de la regla de reenvio no paga mientras la regla existe
    # Cloud Storage STANDARD us-east1, el mismo valor que la Entrega 2
    "gcs_gib_mes": 0.020,
    # Salida a internet, Premium, us-east1 -> Sudamerica (cloud.google.com/vpc/network-pricing)
    "egreso_sudamerica_gib": 0.19,
    # Cloud Monitoring, cloud.google.com/monitoring/pricing: 8 bytes por punto escalar, 150 MiB/mes gratis
    "monitoring_mib": 0.258,
    "monitoring_mib_gratis": 150,
}

# ---------------------------------------------------------------------------------------- tamanos efectivos
DISCO_GRUPO_GIB = 30      # api_group.tf / worker_group.tf
DISCO_COLA_GIB = 10       # queue.tf
DISCO_GENERADOR_GIB = 30  # generator.tf
SQL_VCPU, SQL_RAM_GIB, SQL_SSD_GIB = 1, 3.75, 10   # db-custom-1-3840, database.tf
SQL_BACKUP_GIB = 1.0      # SUPUESTO: 3 copias de una base de decenas de MiB mas los registros de PITR
GCS_GIB = 3.0             # SUPUESTO: el mismo volumen que la Entrega 2 (tres perfiles + margen)
EGRESO_GIB_MES = 5.0      # SUPUESTO: el mismo que la Entrega 2 (HLS a estudiantes en Sudamerica)
LB_GIB_MES = 5.0          # SUPUESTO: lo que procesa el balanceador, del orden del egreso
NAT_GIB_MES = 2.0         # SUPUESTO: apt, git clone y correo de las instancias nuevas
# Exportador de la cola: 4 series (pending, active, outstanding, oldest_pending_age_seconds) cada 30 s.
SERIES_COLA, PUNTOS_MIN = 4, 2


def vm_h(tipo, disco_gib):
    return P[f"{tipo}_h"] + disco_gib * P["pd_balanced_gib_mes"] / HORAS_MES


def monitoring_mes():
    mib = SERIES_COLA * PUNTOS_MIN * 60 * HORAS_MES * 8 / 2**20
    return mib, max(0.0, mib - P["monitoring_mib_gratis"]) * P["monitoring_mib"]


def escenario(n_api, n_worker, generador=False):
    """Lineas (concepto, USD/h, nota). Lo que factura por volumen se pasa a hora sobre HORAS_MES."""
    filas = [
        (f"Grupo de la API: {n_api} x e2-highcpu-2 + 30 GiB (incluye el frontend)", n_api * vm_h("e2-highcpu-2", DISCO_GRUPO_GIB), "MIG"),
        (f"Grupo de workers: {n_worker} x e2-highcpu-2 + 30 GiB", n_worker * vm_h("e2-highcpu-2", DISCO_GRUPO_GIB), "MIG"),
        ("Cola: mooc-queue-server, e2-small + 10 GiB", vm_h("e2-small", DISCO_COLA_GIB), "NO es de ningun grupo"),
        ("Balanceador: reglas de reenvio (HTTPS y redireccion; tarifa minima de hasta 5)", P["reglas_reenvio_h"], "NO es instancia: factura en reposo"),
        ("Balanceador: datos procesados", 2 * LB_GIB_MES * P["lb_datos_gib"] / HORAS_MES, "supuesto de volumen"),
        ("Cloud SQL: 1 vCPU, 3,75 GiB, zonal", SQL_VCPU * P["sql_vcpu_h"] + SQL_RAM_GIB * P["sql_gib_ram_h"], ""),
        ("Cloud SQL: 10 GiB SSD + copias y PITR", SQL_SSD_GIB * P["sql_ssd_gib_h"] + SQL_BACKUP_GIB * P["sql_backup_gib_h"], ""),
        ("Cloud NAT: pasarela", (n_api + n_worker + 1 + (1 if generador else 0)) * P["nat_vm_h"], "por VM que sale por ella"),
        ("Cloud NAT: 1 IP externa de la pasarela", P["nat_ip_h"], "una IP para ~8 VM a 2 048 puertos cada una (nota 57)"),
        ("Cloud NAT: datos procesados", NAT_GIB_MES * P["nat_datos_gib"] / HORAS_MES, "supuesto de volumen"),
        ("Cloud Storage: 2 buckets STANDARD", GCS_GIB * P["gcs_gib_mes"] / HORAS_MES, "supuesto de volumen"),
        ("Salida a internet: HLS a Sudamerica", EGRESO_GIB_MES * P["egreso_sudamerica_gib"] / HORAS_MES, "supuesto de volumen"),
        ("Monitoring: metrica personalizada de la cola", monitoring_mes()[1] / HORAS_MES, "NO es instancia; dentro de lo gratuito"),
    ]
    if generador:
        filas.append(("Generador de carga: e2-standard-4 + 30 GiB", vm_h("e2-standard-4", DISCO_GENERADOR_GIB), "solo durante las pruebas"))
    return filas


def aparcado(cola_encendida):
    filas = [
        ("Direccion global del balanceador, reservada sin regla", P["ip_reservada_sin_uso_h"], "se conserva a proposito"),
        ("Certificado administrado, mapas, proxies, backend", 0.0, "sin coste por hora"),
        ("Cloud SQL detenida: 10 GiB SSD + copias", SQL_SSD_GIB * P["sql_ssd_gib_h"] + SQL_BACKUP_GIB * P["sql_backup_gib_h"], "el computo deja de facturar"),
        ("Cloud Storage: 2 buckets", GCS_GIB * P["gcs_gib_mes"] / HORAS_MES, ""),
        ("Disco de la cola, 10 GiB", DISCO_COLA_GIB * P["pd_balanced_gib_mes"] / HORAS_MES, "aunque la VM este parada"),
    ]
    if cola_encendida:
        filas.append(("VM de la cola encendida (e2-small)", P["e2-small_h"], "el resize --size=0 no la toca"))
    return filas


def usd(x, decimales=2):
    """Coma decimal y espacio de millares, como el resto de la documentacion."""
    return f"{x:,.{decimales}f}".replace(",", " ").replace(".", ",")


def usd_h(x):
    return usd(x, 4)


def tabla(titulo, filas, horas_extra=None):
    total_h = sum(f[1] for f in filas)
    out = [f"### {titulo}", "", "| Concepto | USD/h | USD/mes (730 h) | Nota |", "| :--- | ---: | ---: | :--- |"]
    for c, h, n in filas:
        out.append(f"| {c} | {usd_h(h)} | {usd(h * HORAS_MES)} | {n} |")
    out.append(f"| **Total** | **{usd_h(total_h)}** | **{usd(total_h * HORAS_MES)}** | |")
    if horas_extra:
        for etiqueta, horas in horas_extra:
            out.append(f"| {etiqueta} | | **{usd(total_h * horas)}** | {horas} h |")
    return "\n".join(out) + "\n", total_h


def main():
    t1, h1 = tabla("Escenario 1: los dos grupos a 1 instancia", escenario(1, 1))
    t3, h3 = tabla("Escenario 3: los dos grupos a 3 instancias", escenario(3, 3))
    ts, hs = tabla("Sesion de pruebas: escenario 1 mas el generador", escenario(1, 1, generador=True),
                   [("Una sesion de 2 h", 2), ("Una sesion de 3 h", 3)])
    tp, hp = tabla("Entorno aparcado, con la cola parada", aparcado(False), [("Por dia", 24)])
    tq, hq = tabla("Entorno aparcado, con la cola encendida", aparcado(True), [("Por dia", 24)])
    mib, _ = monitoring_mes()
    print(t1)
    print(t3)
    print(f"Pasar de 1 a 3 instancias por grupo cuesta **{usd_h(h3 - h1)} USD/h** mas "
          f"(+{100 * (h3 - h1) / h1:.0f} %): solo crecen las instancias y su paso por Cloud NAT; todo lo demas es fijo.\n")
    print(ts)
    print(tp)
    print(tq)
    print(f"La metrica personalizada son ~{usd(mib, 1)} MiB/mes ({SERIES_COLA} series, un punto cada 30 s, 8 bytes por punto), "
          f"frente a {P['monitoring_mib_gratis']} MiB gratuitos por cuenta de facturacion.\n")


def autoprueba():
    fallos = []
    # La Entrega 2 cotizo e2-highcpu-2 + 30 GiB en 39,11 USD/mes: el precio de aqui tiene que reproducirlo.
    if abs(vm_h("e2-highcpu-2", 30) * HORAS_MES - 39.11) > 0.02:
        fallos.append("e2-highcpu-2 + 30 GiB no da 39,11 USD/mes")
    # Y Cloud SQL 1 vCPU / 3,75 GiB en 49,31 USD/mes.
    if abs((P["sql_vcpu_h"] + 3.75 * P["sql_gib_ram_h"]) * HORAS_MES - 49.31) > 0.02:
        fallos.append("Cloud SQL no da 49,31 USD/mes")
    # Las reglas de reenvio: lo facturado el 2026-10-29 fueron 13,23 h y 0,33 USD con las dos activas.
    if abs(round(13.23 * P["reglas_reenvio_h"], 2) - 0.33) > 1e-9:
        fallos.append("las reglas de reenvio no reproducen la factura del 2026-10-29")
    # Tres instancias por grupo cuestan mas que una, y la diferencia son solo instancias + NAT.
    d = sum(f[1] for f in escenario(3, 3)) - sum(f[1] for f in escenario(1, 1))
    esperado = 4 * vm_h("e2-highcpu-2", 30) + 4 * P["nat_vm_h"]
    if abs(d - esperado) > 1e-9:
        fallos.append("la diferencia 3 vs 1 no es solo instancias + NAT")
    mib, coste = monitoring_mes()
    if not (0 < mib < 150 and coste == 0):
        fallos.append("la metrica de la cola deberia caber en lo gratuito")
    for f in fallos:
        print("AUTOPRUEBA FALLA:", f)
    print("autoprueba:", "OK" if not fallos else f"{len(fallos)} fallo(s)")
    return not fallos


if __name__ == "__main__":
    if "--autoprueba" in sys.argv:
        sys.exit(0 if autoprueba() else 1)
    main()
