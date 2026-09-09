package handler

import (
	"net/http"

	"github.com/ISIS4426-2026/Plataforma-MOOC/api"
)

// SpecPath is where the raw OpenAPI document is served. The documentation page
// fetches it from here, and tooling can point at it directly.
const SpecPath = "/api/v1/openapi.yaml"

// swaggerUIVersion pins the Swagger UI distribution. An exact version keeps the
// documentation from changing under us when the CDN publishes a new release.
const swaggerUIVersion = "5.17.14"

// NewOpenAPISpecHandler serves the embedded contract.
func NewOpenAPISpecHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(api.Spec)
	}
}

// NewDocsHandler serves the browsable Swagger UI page.
//
// Serving it from the API itself, rather than from a separate container, is
// what makes the "Try it out" button work: the page and the endpoints share an
// origin, so the browser issues no cross-origin request and no CORS policy is
// needed. It also means the docs are always those of the instance answering the
// requests.
func NewDocsHandler() http.HandlerFunc {
	page := docsPage()

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	}
}

func docsPage() string {
	return `<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Plataforma MOOC — API v1</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@` + swaggerUIVersion + `/swagger-ui.css">
  <style>
    body { margin: 0; background: #fafafa; }
    .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@` + swaggerUIVersion + `/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: '` + SpecPath + `',
      dom_id: '#swagger-ui',
      deepLinking: true,
      displayRequestDuration: true,
      presets: [SwaggerUIBundle.presets.apis],
      layout: 'BaseLayout'
    });
  </script>
</body>
</html>
`
}
