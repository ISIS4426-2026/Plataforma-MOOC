# Versiones y estado remoto (issue #115).
#
# El estado vive en un bucket compartido y no en el disco de nadie. Es lo que
# permite que los cuatro integrantes apliquen cambios sobre la misma
# infraestructura: el backend de GCS toma un bloqueo mientras dura una
# operacion, asi que dos `apply` simultaneos no se pisan -- el segundo espera o
# falla, nunca corrompe el estado.
#
# El nombre del bucket esta escrito aqui a proposito, en lugar de pasarse por
# `-backend-config`. Si cada uno pudiera apuntar a un bucket distinto,
# volveriamos a tener estados divergentes, que es justo lo que el estado remoto
# evita.

terraform {
  # Terraform 1.16.4 es la version con la que se escribio y verifico esta
  # configuracion. El rango admite parches pero no cambios de version menor,
  # para que nadie migre el formato del estado sin querer.
  required_version = "~> 1.16.0"

  required_providers {
    google = {
      source = "hashicorp/google"
      # Version exacta, como pide el issue. Una version flotante haria que
      # `plan` diera resultados distintos segun quien lo ejecute y cuando.
      version = "8.4.0"
    }
  }

  backend "gcs" {
    bucket = "plataforma-mooc-entrega2-tfstate"
    prefix = "entrega2"
  }
}
