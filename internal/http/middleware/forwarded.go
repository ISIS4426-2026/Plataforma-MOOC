package middleware

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// ForwardedHeaders applies proxy metadata only when the connection peer is the
// explicitly trusted proxy. The proxy must overwrite, not append, these values.
func ForwardedHeaders(trustedProxyIP string) Middleware {
	trustedIP := net.ParseIP(trustedProxyIP)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peerIP, peerPort, err := net.SplitHostPort(r.RemoteAddr)
			if trustedIP == nil || err != nil || !trustedIP.Equal(net.ParseIP(peerIP)) {
				next.ServeHTTP(w, r)
				return
			}

			if forwardedIP := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Forwarded-For"))); forwardedIP != nil {
				r.RemoteAddr = net.JoinHostPort(forwardedIP.String(), peerPort)
			}
			if scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); scheme == "http" || scheme == "https" {
				r.URL.Scheme = scheme
			}
			if forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); validForwardedHost(forwardedHost) {
				r.URL.Host = forwardedHost
				r.Host = forwardedHost
			}

			next.ServeHTTP(w, r)
		})
	}
}

func validForwardedHost(host string) bool {
	if host == "" {
		return false
	}
	parsed, err := url.Parse("https://" + host)
	return err == nil && parsed.Host == host && parsed.User == nil && parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == ""
}
