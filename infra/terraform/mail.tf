# Correo transaccional (issue #126, F1).
#
# Aqui no hay servidor de correo: la plataforma habla con un proveedor SMTP
# administrado. Lo unico que necesita infraestructura es **donde vive la
# contrasena del proveedor y quien puede leerla**.
#
# Por que un proveedor y no una VM con Mailpit: el docente autorizo una VM
# dedicada, pero tambien aclaro que no le interesa probar el correo dentro de las
# pruebas de carga. 730 horas mensuales de computo para un componente que no se
# mide no se justifican, y con un proveedor la verificacion de cuentas queda
# como en produccion.

# El contenedor del secreto, no su valor.
#
# Esta distincion es la que permite declararlo aqui sin romper la regla de B4
# (#117): `google_secret_manager_secret` crea el recipiente y sus politicas, que
# no son sensibles. **El valor entra aparte**, con
# `gcloud secrets versions add smtp-password`, y nunca pasa por el estado de
# Terraform.
#
# Es distinto de `db-password`, que se creo a mano antes de que existiera esta
# configuracion y que por eso se referencia con un `data` en database.tf. Para
# uno nuevo no hay razon para repetir el paso manual.
resource "google_secret_manager_secret" "smtp_password" {
  secret_id = "smtp-password"
  project   = var.project_id

  labels = var.labels

  # The credential survives the final infrastructure destroy, just like
  # db-password. Deleting a Terraform declaration must not revoke an external
  # provider credential or erase its rotation history.
  deletion_policy = "ABANDON"

  replication {
    auto {}
  }

  depends_on = [
    google_project_service.required["secretmanager.googleapis.com"]
  ]
}

# La API es la unica que envia correo.
#
# El worker no aparece a proposito, y no es un olvido: procesa video y no manda
# mensajes. Darle acceso seria ampliar lo que una credencial filtrada alcanza sin
# que nadie gane nada.
#
# Con alcance al secreto y no al proyecto, igual que la contrasena de la base:
# `secretAccessor` a nivel de proyecto daria a cada cuenta acceso a los secretos
# de las demas.
resource "google_secret_manager_secret_iam_member" "smtp_password_reader" {
  project   = var.project_id
  secret_id = google_secret_manager_secret.smtp_password.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.web_server.email}"
}

# El usuario del proveedor NO es un secreto, y tratarlo como tal seria teatro:
# en SendGrid es la cadena literal "apikey" y en Brevo el correo de la cuenta.
# Va como variable de entorno normal en el Compose de la VM. Lo que protege la
# cuenta es la contrasena, y esa si esta arriba.

output "smtp_password_secret" {
  description = <<-EOT
    Nombre del secreto con la contrasena del proveedor SMTP. Es un
    identificador, no la credencial: el valor se anade con
    `gcloud secrets versions add` y se lee en ejecucion.
  EOT
  value       = google_secret_manager_secret.smtp_password.secret_id
}
