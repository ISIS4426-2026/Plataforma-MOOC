# Diagrama de Arquitectura de Red (Issue B3)

## Diagrama de Red de la Plataforma MOOC

> **Las direcciones privadas de las VMs no son fijas.** La subred las asigna por
> DHCP, así que una VM recreada recibe otra: estas dos ya pasaron de
> `10.0.1.2`/`10.0.1.3` a `10.0.1.4`/`10.0.1.5`. Lo que el diseño fija es la
> **subred** (`10.0.1.0/24`) y las reglas de firewall, que se expresan por
> etiquetas de red y por rango, nunca por IP. Los valores concretos se leen con
> `terraform output`, no se copian de aquí — la de Cloud SQL sí es estable,
> porque la reserva el peering de Private Services Access.

> **El Worker Server sí tiene dirección IPv4 pública** (estática, declarada en
> `compute.tf`), y aun así no es alcanzable desde internet. La protección la da la
> regla de firewall, no la ausencia de dirección: `mooc-allow-web-ingress` aplica
> solo a instancias con la etiqueta `web-server`, y el worker no la lleva. Es la
> decisión de costos de B1 —dos IPv4 externas cuestan 3,65 USD/mes frente a 6,73
> de una IPv4 más Cloud NAT—, y conviene leerla explícita porque «tiene IP
> pública» y «está expuesto» no son lo mismo.
>
> Consecuencia que conviene revisar en I2: **el Cloud NAT quedó aprovisionado y no
> se usa.** Una VM con dirección externa sale por ella, no por el NAT, así que
> `mooc-nat` no procesa tráfico de ninguna de las dos. O se retira, o se retiran
> las IPs externas; tener ambos paga dos veces por la misma salida.


Este diagrama ilustra la topología de red aprovisionada por Terraform (`network.tf`), mostrando la separación entre el acceso público desde Internet y los componentes internos aislados en la VPC privada.

```mermaid
flowchart TD
    subgraph Internet ["🌐 Internet (0.0.0.0/0)"]
        User["Cliente / Navegador"]
        Admin["Administrador / DevOps"]
    end

    subgraph GoogleIAP ["🛡️ Google Cloud IAP (35.235.240.0/20)"]
        IAPProxy["Túnel IAP SSH"]
    end

    subgraph GCP ["☁️ Google Cloud Project: plataforma-mooc-entrega2"]
        subgraph VPC ["🔒 mooc-vpc (10.0.0.0/16)"]
            
            subgraph Subnet ["🖥️ mooc-subnet (us-east1: 10.0.1.0/24)"]
                WebServer["🌐 Web Server VM\nTag: web-server\nTag: allow-iap-ssh\nIP Privada: 10.0.1.4 (asignada por DHCP)\nIP Pública: Externa Estática"]
                WorkerServer["⚙️ Worker Server VM\nTag: worker-server\nTag: allow-iap-ssh\nIP Privada: 10.0.1.5 (asignada por DHCP)\nIP Pública: Externa Estática\nSin regla de ingreso: inalcanzable"]
                RedisQueue[("📦 Cola de Mensajería (Asynq/Redis)\nPuerto: 6379\nEjecutando en Worker Server")]
            end

            CloudNAT["📡 Cloud Router & Cloud NAT\n(Egress Outbound para actualizaciones)"]

            subgraph ServicePeering ["🔐 Service Networking Peering (10.0.2.0/20)"]
                CloudSQL[("🗄️ Cloud SQL PostgreSQL (C1)\nSolo IP Privada: 10.171.240.3\nIPv4 Pública: Deshabilitada")]
            end
        end
    end

    %% Conexiones Públicas
    User -->|HTTP 80 / HTTPS 443| WebServer
    Admin -->|gcloud compute ssh --tunnel-through-iap| IAPProxy
    IAPProxy -->|SSH 22| WebServer
    IAPProxy -->|SSH 22| WorkerServer

    %% Conexiones Internas VPC
    WebServer -->|TCP Interno: 6379| RedisQueue
    WebServer -->|PostgreSQL 5432| CloudSQL
    WorkerServer -->|PostgreSQL 5432| CloudSQL

    %% Salida Egress
    WorkerServer -->|Egress HTTP/HTTPS| CloudNAT
    CloudNAT -->|Salida a repositorios/APIs| Internet

    %% Estilos de Nodos
    style User fill:#e1f5fe,stroke:#0288d1,stroke-width:2px
    style Admin fill:#fff3e0,stroke:#f57c00,stroke-width:2px
    style WebServer fill:#d1c4e9,stroke:#512da8,stroke-width:2px
    style WorkerServer fill:#c8e6c9,stroke:#388e3c,stroke-width:2px
    style RedisQueue fill:#ffcdd2,stroke:#d32f2f,stroke-width:2px
    style CloudSQL fill:#bbdefb,stroke:#1976d2,stroke-width:2px
    style CloudNAT fill:#fff9c4,stroke:#fbc02d,stroke-width:2px
```

---

## Explicación de los Límites de Red y Reglas de Firewall

1. **Web Server (Único Punto Público):**
   * **Regla `mooc-allow-web-ingress`:** Habilita el tráfico entrante desde `0.0.0.0/0` en los puertos TCP `80` (HTTP) y `443` (HTTPS) únicamente a instancias con la etiqueta `web-server`.

2. **Worker Server y Cola de Mensajería (Totalmente Isolados):**
   * El Worker Server no expone ningún puerto hacia internet. Su puerto Redis/Asynq (`6379`) y servicios internos solo son accesibles desde dentro del rango CIDR privado `10.0.1.0/24`.
   * **Salida a Internet (Egress):** Utiliza Cloud Router + Cloud NAT para descargar imágenes Docker y paquetes del sistema (`apt update`), manteniendo cero puertos expuestos a escaneos entrantes externos.

3. **Base de Datos Administrada Cloud SQL (C1):**
   * Configurada mediante **Private Services Access** (`servicenetworking.googleapis.com`) sobre el rango peering `10.0.2.0/20`.
   * No posee dirección IP pública (`ipv4_enabled = false`). Únicamente acepta conexiones PostgreSQL (puerto `5432`) desde las direcciones IP privadas pertenecientes a la subred `10.0.1.0/24`.

4. **Administración Segura vía IAP:**
   * **Regla `mooc-allow-ssh-iap`:** Permite conexiones SSH (puerto `22`) exclusivamente desde el bloque de IPs de Google Identity-Aware Proxy (`35.235.240.0/20`). Las máquinas no tienen el puerto 22 abierto a todo Internet (`0.0.0.0/0`).
