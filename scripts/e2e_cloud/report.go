package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ---- resultados ------------------------------------------------------------

// step is one verified interaction: what was called, what the deployment
// answered, and the state check that backs the answer. The issue's acceptance
// criterion is precisely that pairing -- an HTTP code on its own does not show
// that anything was persisted, so every step that expects success also carries
// the state it went on to observe.
type step struct {
	Flow    string
	Name    string
	Request string
	Status  string
	State   string
	Passed  bool
	Detail  string
	// Info marks a row that is neither a pass nor a failure: a leg the run
	// deliberately does not drive. It must not read as a green check.
	Info bool
}

// run accumulates the steps and writes the run log as it goes, so a run that
// dies halfway still leaves the evidence of everything it did reach.
type run struct {
	api     *client
	log     io.Writer
	steps   []step
	flow    string
	started time.Time
}

func (r *run) flowIs(name string) {
	r.flow = name
	fmt.Fprintf(r.log, "\n=== %s\n", name)
}

// check records one step. want is the expected HTTP status; state describes what
// was observed afterwards and stateOK whether that observation held. A step
// passes only when both halves do.
func (r *run) check(name string, res *response, want int, state string, stateOK bool) bool {
	passed := res.err == nil && res.status == want && stateOK

	detail := ""
	switch {
	case res.err != nil:
		detail = "transporte: " + res.err.Error()
	case res.status != want:
		detail = fmt.Sprintf("se esperaba %d y respondió %d; code=%q", want, res.status, res.errorCode())
	case !stateOK:
		detail = "el estado observado no coincide con lo esperado"
	}

	status := fmt.Sprintf("%d", res.status)
	if res.err != nil {
		status = "—"
	}
	if state == "" {
		state = "el código de estado es la verificación"
	}

	r.steps = append(r.steps, step{
		Flow: r.flow, Name: name, Request: res.label(), Status: status,
		State: state, Passed: passed, Detail: detail,
	})

	mark := "OK  "
	if !passed {
		mark = "FALLA"
	}
	fmt.Fprintf(r.log, "[%s] %-58s %-44s %s -> %s\n", mark, name, res.label(), status, state)
	if detail != "" {
		fmt.Fprintf(r.log, "        %s\n", detail)
	}
	return passed
}

// note records something observed that is not an HTTP call of its own -- a wait
// that ended, a file that round-tripped -- so the table reads as one narrative.
func (r *run) note(name, request, state string, ok bool, detail string) bool {
	r.steps = append(r.steps, step{
		Flow: r.flow, Name: name, Request: request, Status: "—",
		State: state, Passed: ok, Detail: detail,
	})
	mark := "OK  "
	if !ok {
		mark = "FALLA"
	}
	fmt.Fprintf(r.log, "[%s] %-58s %-44s %s -> %s\n", mark, name, request, "—", state)
	// The detail explains a failure, so printing it on a step that passed would
	// contradict the line above it.
	if !ok && detail != "" {
		fmt.Fprintf(r.log, "        %s\n", detail)
	}
	return ok
}

// outOfScope records a leg the run deliberately does not drive, so the table
// says so in its own voice rather than leaving a silent gap or a green check that
// nothing actually verified.
func (r *run) outOfScope(name, request, why string) {
	r.steps = append(r.steps, step{
		Flow: r.flow, Name: name, Request: request, Status: "—",
		State: why, Passed: true, Info: true,
	})
	fmt.Fprintf(r.log, "[nota ] %-58s %-44s %s -> %s\n", name, request, "—", why)
}

func (r *run) failed() int {
	n := 0
	for _, s := range r.steps {
		if !s.Passed {
			n++
		}
	}
	return n
}

func (r *run) informational() int {
	n := 0
	for _, s := range r.steps {
		if s.Info {
			n++
		}
	}
	return n
}

// writeTable renders the flow -> result -> evidence table the issue asks for.
func (r *run) writeTable(path, base string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# G3 — Resultados de la corrida E2E en la nube\n\n")
	fmt.Fprintf(&b, "Generado por `scripts/e2e_cloud` el %s contra `%s`.\n",
		r.started.Format("2006-01-02 15:04:05 MST"), base)
	fmt.Fprintf(&b, "No editar a mano: se reescribe en cada corrida.\n\n")
	fmt.Fprintf(&b, "Pasos verificados: %d · fallos: %d · fuera de alcance: %d · duración: %s\n\n",
		len(r.steps)-r.informational(), r.failed(), r.informational(),
		time.Since(r.started).Round(time.Second))

	flow := ""
	for _, s := range r.steps {
		if s.Flow != flow {
			flow = s.Flow
			fmt.Fprintf(&b, "\n## %s\n\n", flow)
			fmt.Fprintf(&b, "| | Paso | Petición | HTTP | Estado verificado |\n")
			fmt.Fprintf(&b, "| :---: | :--- | :--- | :---: | :--- |\n")
		}
		mark := "✅"
		state := s.State
		switch {
		case s.Info:
			mark = "ℹ️"
		case !s.Passed:
			mark = "❌"
			state = s.State + " — **" + s.Detail + "**"
		}
		fmt.Fprintf(&b, "| %s | %s | `%s` | %s | %s |\n",
			mark, s.Name, s.Request, s.Status, state)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// ---- cliente ---------------------------------------------------------------

// client speaks to the deployment the way the platform's own clients do: a
// Bearer token and no cookie jar.
//
// The missing cookie jar is deliberate and is the shape G2 had to correct in the
// Postman collections: the login also sets a Secure session cookie, and a client
// that stores it and keeps sending it turns every later mutation into a CSRF
// rejection unless it also declares an allowed Origin. A pure Bearer client must
// not carry the cookie, so this one never does.
type client struct {
	base string
	hc   *http.Client
}

type response struct {
	method string
	path   string
	status int
	body   []byte
	header http.Header
	err    error
}

func (r *response) label() string {
	return r.method + " " + r.path
}

func (r *response) errorCode() string {
	var e struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(r.body, &e)
	return e.Code
}

// details returns the field names of a validation_failed response, which is how
// the accumulated publication errors are asserted without pinning the wording.
func (r *response) details() []string {
	var e struct {
		Details []struct {
			Field string `json:"field"`
		} `json:"details"`
	}
	_ = json.Unmarshal(r.body, &e)
	out := make([]string, 0, len(e.Details))
	for _, d := range e.Details {
		out = append(out, d.Field)
	}
	return out
}

func (r *response) into(v any) error {
	if r.err != nil {
		return r.err
	}
	return json.Unmarshal(r.body, v)
}

func (c *client) do(method, path, token string, body any, hdr map[string]string) *response {
	res := &response{method: method, path: path}

	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			res.err = err
			return res
		}
		payload = strings.NewReader(string(raw))
	}

	req, err := http.NewRequest(method, c.base+path, payload)
	if err != nil {
		res.err = err
		return res
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}

	return c.send(req, res)
}

// absolute issues a request against a full URL rather than the API base: the
// signed upload and download URLs, and the bucket's public prefixes.
//
// The query string is stripped from the recorded label because on a signed URL
// it is the signature, and evidence files must not carry credentials.
func (c *client) absolute(method, raw string, body io.Reader, hdr map[string]string) *response {
	label := raw
	if u, err := url.Parse(raw); err == nil {
		u.RawQuery = ""
		label = u.String()
		if len(label) > 78 {
			label = label[:75] + "..."
		}
	}
	res := &response{method: method, path: label}

	req, err := http.NewRequest(method, raw, body)
	if err != nil {
		res.err = err
		return res
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return c.send(req, res)
}

func (c *client) send(req *http.Request, res *response) *response {
	resp, err := c.hc.Do(req)
	if err != nil {
		res.err = err
		return res
	}
	defer resp.Body.Close()

	// Bodies are bounded: a signed GET of the original video would otherwise be
	// read into memory in full, and nothing here needs more than the first
	// kilobytes of it.
	res.body, res.err = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	res.status = resp.StatusCode
	res.header = resp.Header
	return res
}
