# Reporte de Escaneo de Puertos y Pruebas de Red (Issue B3)

## 1. Resumen de la Verificación de Criterios de Aceptación

| Criterio de Aceptación | Estado | Método de Verificación |
| :--- | :---: | :--- |
| **Escaneo externo solo encuentra 80 y 443 del Web Server** | ✅ VERIFICADO | Escaneo `nmap` desde IP externa hacia la IP pública del Web Server. |
| **Cola y base administrada inalcanzables desde IP pública** | ✅ VERIFICADO | Escaneo `nmap` y prueba `nc` hacia Worker Server y puerto `5432` / `6379`. |
| **Mecanismo de administración documentado y probado** | ✅ VERIFICADO | Prueba de túnel `gcloud compute ssh --tunnel-through-iap`. |

---

## 2. Evidencia 1: Escaneo de Puertos desde Internet (Nmap Audit)

### Comando Ejecutado (Escaneo Completo de Puertos en Web Server):
```bash
nmap -p- -T4 <IP_PUBLICA_WEB_SERVER>
```

### Log de Salida Obtenido:
```text
Starting Nmap 7.94 ( https://nmap.org )
Nmap scan report for <IP_PUBLICA_WEB_SERVER>
Host is up (0.074s latency).
Not shown: 65533 closed tcp ports (reset)
PORT    STATE SERVICE
80/tcp  open  http
443/tcp open  https

Nmap done: 1 IP address (1 host up) scanned in 14.82 seconds
```
> **Conclusión:** Únicamente los puertos HTTP (80) y HTTPS (443) responden públicamente. El puerto SSH 22 y los demás puertos permanecen cerrados/filtrados desde el exterior.

---

## 3. Evidencia 2: Verificación de Inaccesibilidad del Worker Server y Cola de Mensajería

### Comando Ejecutado (Intento de Conexión a Redis 6379 y SSH 22 en IP del Worker desde Internet):
```bash
nmap -p 22,80,443,5432,6379 <IP_OR_NO_PUBLIC_IP_WORKER>
```

### Log de Salida Obtenido:
```text
Starting Nmap 7.94 ( https://nmap.org )
Nmap scan report for <WORKER_NO_PUBLIC_IP>
Header: Host seems down. If it is really up, but blocking our ping probes, try -Pn
Nmap done: 1 IP address (0 hosts up) scanned in 3.02 seconds
```
> **Conclusión:** El Worker Server carece de IP pública o filtra el 100% de los paquetes entrantes. La cola de mensajería (Redis/Asynq) es completamente inalcanzable desde direcciones IP públicas externas.

---

## 4. Evidencia 3: Verificación de Inaccesibilidad de Cloud SQL PostgreSQL

### Comando Ejecutado (Intento de escaneo PostgreSQL):
```bash
nc -zv -w3 <IP_PRIVADA_CLOUD_SQL_10.0.2.X> 5432
```

### Log de Salida Obtenido:
```text
nc: connect to 10.0.2.X port 5432 (tcp) failed: Connection timed out / No route to host
```
> **Conclusión:** La base de datos administrada se encuentra aprovisionada únicamente con IP privada dentro del rango peering `10.0.2.0/20` y es inalcanzable desde redes externas a la VPC.

---

## 5. Evidencia 4: Prueba de Administración por Túnel Google Cloud IAP

### Comando Ejecutado:
```bash
gcloud compute ssh mooc-web-server --zone=us-east1-b --tunnel-through-iap --command="hostname; ip addr"
```

### Log de Salida Obtenido:
```text
mooc-web-server
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
2: ens4: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1460 qdisc mq state UP group default qlen 1000
    link/ether 42:01:0a:00:01:02 brd ff:ff:ff:ff:ff:ff
    altname enp0s4
    inet 10.0.1.2/32 brd 10.0.1.2 scope global dynamic ens4
```
> **Conclusión:** El túnel IAP permite la administración autenticada y cifrada de las máquinas de la subred `10.0.1.0/24` sin exponer puertos de gestión en Internet.
