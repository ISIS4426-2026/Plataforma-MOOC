# Documentación de Administración y Conectividad de Red (Issue B3)

## 1. Mecanismo de Administración Mediante Google Cloud IAP

### Diagnóstico de Seguridad
Para evitar exponer el puerto de administración SSH (`22`) a escaneos y ataques de fuerza bruta desde todo Internet (`0.0.0.0/0`), la VPC `mooc-vpc` implementa el acceso mediante **Google Cloud Identity-Aware Proxy (IAP)**.

### Funcionamiento
Google IAP actúa como un bastion host administrado por Google. La regla de firewall `mooc-allow-ssh-iap` únicamente permite tráfico en el puerto `22` desde el rango de IP asignado a Google IAP (`35.235.240.0/20`). Las instancias no requieren IPs públicas ni la apertura del puerto 22 hacia internet.

### Comandos de Administración

Para autenticarse y conectarse por SSH a cualquiera de las VMs de la plataforma, los integrantes autorizados del proyecto deben ejecutar:

#### Conexión a la VM Web Server:
```bash
gcloud compute ssh mooc-web-server \
  --zone=us-east1-b \
  --project=plataforma-mooc-entrega2 \
  --tunnel-through-iap
```

#### Conexión a la VM Worker Server (sin IP pública):
```bash
gcloud compute ssh mooc-worker-server \
  --zone=us-east1-b \
  --project=plataforma-mooc-entrega2 \
  --tunnel-through-iap
```

#### Port Forwarding Seguro por Túnel IAP (Ejemplo: acceder localmente a un servicio interno):
```bash
gcloud compute start-iap-tunnel mooc-worker-server 6379 \
  --local-host-port=localhost:6379 \
  --zone=us-east1-b \
  --project=plataforma-mooc-entrega2
```

---

## 2. Conectividad Saliente (Egress) para Instalación de Dependencias

### Necesidad Técnica
Tanto el Web Server como el Worker Server requieren conectarse a repositorios externos en tiempo de despliegue y mantenimiento para:
* Actualizar paquetes del sistema operativo (`apt-get update`).
* Descargar imágenes de contenedores Docker (`docker pull`).
* Conectarse a APIs externas y servicios de correo (SMTP STARTTLS/TLS en puertos 587/465).

### Solución Implementada: Cloud Router + Cloud NAT
En `network.tf` se aprovisionan los recursos `google_compute_router.router` y `google_compute_router_nat.nat`:
* **Aislamiento Inbound:** Las instancias privadas no aceptan conexiones desde internet.
* **Acceso Outbound:** Traduce las peticiones salientes originadas en `mooc-subnet` (`10.0.1.0/24`), asignando IPs efímeras administradas por Google únicamente para el tráfico de respuesta.

---

## 3. Matriz de Conectividad y Aislamiento

| Origen | Destino | Puerto | Permiso | Propósito |
| :--- | :--- | :--- | :--- | :--- |
| `0.0.0.0/0` (Internet) | Web Server | `80`, `443` | **Permitido** | Acceso público de usuarios a la plataforma MOOC. |
| `0.0.0.0/0` (Internet) | Worker Server | Todos | **Bloqueado** | Isolamiento completo del Worker. |
| `0.0.0.0/0` (Internet) | Cloud SQL | `5432` | **Bloqueado** | IPv4 pública deshabilitada. |
| `35.235.240.0/20` (IAP) | Web / Worker | `22` | **Permitido** | Administración SSH autenticada vía GCP IAM. |
| `10.0.1.0/24` (VPC) | `10.0.1.0/24` (VPC) | Todos | **Permitido** | Comunicación interna entre Web Server y Worker (Redis 6379). |
| `10.0.1.0/24` (VPC) | `10.0.2.0/20` (Peering) | `5432` | **Permitido** | Conexión de aplicación y worker a PostgreSQL Cloud SQL. |
