# Ruta para trabajar la infraestructura en paralelo

Cómo repartir el bloque de despliegue entre tres personas sin que nadie espere a
otro y sin que se pisen sobre el estado compartido de Terraform.

Parte de **B1 y B2 ya cerrados**: el proyecto, la región, el presupuesto, el
estado remoto, las cuentas de servicio y las APIs están en su sitio.

---

## El grafo real

```
B2 ✅
├── B3 ──┐
├── B4 ──┴──→ C1 ──→ C2
└── C3 ─────→ C4
                      D1  (declara depender de A5 ❌)
C1 + C4 + D1 ──→ D2 ──→ D3, F1
C4 + D1 ───────→ E1
```

Tres cosas se leen ahí:

**Solo B3, B4 y C3 están desbloqueados hoy.** Todo lo demás espera a alguno de
esos tres.

**D1 declara depender de A5, que no está construido**, y D1 alimenta D2 y E1. Tal
cual, toda la cadena de despliegue está detenida detrás de los quizzes.

**B3 es el mayor desbloqueador:** alimenta C1, D2 y E1. Si hay que priorizar
algo, es eso.

---

## La decisión que hay que tomar primero: A5

Construir una imagen de contenedor no exige que la aplicación esté terminada;
las imágenes se reconstruyen en cada cambio, es su naturaleza. **La propuesta es
relajar la dependencia de D1 sobre A5**: que D1 entregue el pipeline de
construcción con lo que hay en `main` —A3 y A6 ya están— y que las imágenes
finales de la entrega se rebuildeen cuando A5 aterrice.

Con eso A5 deja de bloquear el despliegue. Sigue siendo obligatorio, pero su
plazo real es **antes de H2 y H3**, porque el escenario 1 incluye «presentan
quizzes». Encaja bien con que A5, H2 y H3 son de la misma persona.

Si se prefiere mantener la dependencia estricta, **A5 pasa a ser lo más urgente
de la entrega**, por encima de cualquier issue de infraestructura.

---

## El reparto

Cada ronda son tres frentes que **no dependen entre sí**. Nadie espera a nadie
dentro de una ronda.

| Ronda | Diego | Fredy | Tania |
| :--- | :--- | :--- | :--- |
| **1** | **B3** VPC, subredes, firewall | **B4** secretos | **C3** bucket, IAM, CORS |
| **2** | **D1** imágenes Docker | **C1** Cloud SQL | **C4** apuntar al bucket real |
| **3** | **D2** VM web y proxy | **E1** VM worker | **G1** datos sintéticos |
| **4** | **D3** HTTPS y cookies | **C2** respaldo y recreación | **A5** quizzes |
| **5** | **G4** verificación de red | **G3** E2E en cloud | **G2** Postman en cloud · **F1** SMTP |

### Los dos cambios respecto a la asignación actual

**C3 pasa de Diego a Tania.** Diego tenía dos de los tres issues desbloqueados,
así que el paralelismo de la primera ronda dependía de que una sola persona
hiciera dos cosas a la vez.

**E1 pasa de Diego a Fredy.** Diego acumulaba ocho issues, casi todo el camino
crítico; E1 y D2 caían en la misma ronda sobre la misma persona.

### Por qué cada issue cae donde cae

**Ronda 1** son exactamente los tres que solo dependen de B2. B3 primero porque
desbloquea tres issues; B4 es el más ligero —Secret Manager ya está habilitado y
el secreto `db-password` ya existe—, y por eso se combina bien con revisar los
avances de los otros dos.

**Ronda 2** cada uno continúa por su rama natural: quien hizo B3 y B4 tiene el
contexto para C1; quien hizo C3 tiene el del bucket para C4.

**Ronda 3** es donde aparecen las VMs, que es el primer gasto serio de crédito.
G1 entra aquí porque necesita C4 para sembrar el bucket.

**A5 en la ronda 4** es la consecuencia de relajar D1. Si se mantiene la
dependencia estricta, sube a la ronda 1 y desplaza todo lo demás.

---

## Cómo no pisarse sobre el estado compartido

Este es el punto delicado, y el que no resuelve ninguna asignación de tareas: las
tres personas escriben Terraform **contra un mismo estado**.

### El riesgo que importa

El bloqueo del backend impide que dos `apply` corran a la vez, pero **no** impide
algo peor: si aplicas desde una rama que no tiene el código de los demás,
Terraform verá en el estado recursos que tu código no declara y **propondrá
destruirlos**. Un `apply` desde una rama desactualizada puede borrar el trabajo
de otro.

### Las tres reglas que lo evitan

**1. Un archivo por componente.** Así tres personas tocan tres archivos
distintos y `git` no tiene nada que fusionar:

| Archivo | Issue |
| :--- | :--- |
| `network.tf` | B3 |
| `secrets.tf` | B4 |
| `storage.tf` | C3 |
| `database.tf` | C1 |
| `compute.tf` | D2 y E1 |

**2. `apply` solo desde `main`.** Se trabaja en rama, se abre PR, se fusiona, se
hace `git pull` en `main` y **ahí** se aplica. El paralelismo está en escribir el
código, no en aplicarlo: cada `apply` dura minutos y se hacen de a uno.

**3. Antes de planificar en tu rama, trae `main`.**

```bash
git checkout mi-rama
git merge main
terraform plan
```

Sin ese `merge`, tu `plan` mostrará como destrucciones los recursos que otro ya
fusionó. No es que vaya a destruirlos —un `plan` no cambia nada— pero te hará
perder tiempo interpretando un diagnóstico falso.

### Leer el plan de verdad

La línea que importa es la última. Un `Plan: 0 to add, 0 to change, 3 to destroy`
inesperado casi siempre significa que tu rama está desactualizada. **Ante una
destrucción que no esperabas, no apliques: pregunta en el chat del equipo.**

### Y si aun así dos personas coinciden

Verán `Error acquiring the state lock` con el nombre de quién lo tiene. **No es
un fallo, es el mecanismo funcionando.** Se espera y se reintenta. El
procedimiento completo, incluido qué hacer con un bloqueo colgado, está en
[`../../infra/terraform/README.md`](../../infra/terraform/README.md).

---

## Lo que gobierna el gasto

A partir de la **ronda 3** hay VMs y base de datos encendidas, y el crédito
empieza a consumirse en serio: la estimación 24×7 son 133,73 USD/mes y **un cupón
de 50 USD paga unos 11 días** de operación continua.

Con tres personas encendiendo recursos, la política de apagado deja de ser una
recomendación y pasa a ser una coordinación: **quien enciende, apaga**, y
conviene avisar en el chat cuando algo queda encendido a propósito. Las alertas
de presupuesto llegan a los cuatro.

Detalle en [`CONFIGURACION_Y_COSTOS.md`](CONFIGURACION_Y_COSTOS.md).

---

## Antes de empezar, cada uno

```bash
git pull
cd infra/terraform
terraform init
export TF_VAR_db_password="$(gcloud secrets versions access latest \
  --secret=db-password --project=plataforma-mooc-entrega2)"
terraform plan
```

Si sale `No changes. Your infrastructure matches the configuration.`, el entorno
está bien montado: herramientas, autenticación, acceso al estado compartido y al
secreto. Si algo falla, falla ahí y no a mitad de un issue.

La puesta a punto completa está en
[`../../infra/terraform/README.md`](../../infra/terraform/README.md).
