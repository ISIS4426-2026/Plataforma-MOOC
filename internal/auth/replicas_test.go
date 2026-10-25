package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/auth"
	"github.com/ISIS4426-2026/Plataforma-MOOC/internal/domain"
)

// Sesiones entre replicas de API (issue #196, A4).
//
// # El defecto que esto cierra
//
// En la Entrega 2 cada VM levantaba su propio Redis, asi que cada replica tenia
// su propia cache de sesiones. `Authenticate` cae a PostgreSQL cuando la cache
// falla, de modo que una sesion iniciada en una replica funcionaba en otra: eso
// nunca estuvo roto.
//
// Lo que si estaba roto era el cierre de sesion. `Logout` revoca en la base y
// borra **su** entrada de cache. Otra replica que tuviera la sesion cacheada la
// seguiria sirviendo --su copia dice usable-- hasta que expirara el TTL, que
// por omision son 24 horas. Una sesion cerrada que sigue valiendo un dia no es
// una molestia: es un fallo de seguridad.
//
// La correccion es de despliegue, no de codigo: una sola cache compartida fuera
// de las instancias. Estas pruebas fijan el comportamiento que esa decision
// tiene que producir, para que no se pueda volver atras sin que algo falle.

// replicas monta dos servicios que comparten base y cache, que es exactamente
// lo que son dos instancias de API detras del balanceador.
type replicas struct {
	a, b     *auth.Service
	usuarios *fakeUserRepo
	cache    *fakeSessionCache
}

func nuevasReplicas(t *testing.T) *replicas {
	t.Helper()

	compartido := auth.Deps{
		Users:               newFakeUserRepo(),
		Sessions:            newFakeSessionRepo(),
		SessionCache:        newFakeSessionCache(),
		VerificationTokens:  newFakeTokenRepo(),
		PasswordResetTokens: newFakeTokenRepo(),
		Mailer:              newFakeMailer(),
	}
	cfg := auth.Config{
		SessionTTL:           time.Hour,
		EmailVerificationTTL: 24 * time.Hour,
		PasswordResetTTL:     time.Hour,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return &replicas{
		a:        auth.NewService(compartido, cfg, logger),
		b:        auth.NewService(compartido, cfg, logger),
		usuarios: compartido.Users.(*fakeUserRepo),
		cache:    compartido.SessionCache.(*fakeSessionCache),
	}
}

// cuentaActiva siembra un usuario verificado y devuelve su correo y contrasena.
func (r *replicas) cuentaActiva(t *testing.T) (string, string) {
	t.Helper()
	const correo, clave = "estudiante@uniandes.edu.co", "Password123!"

	hash, err := auth.HashPassword(clave)
	if err != nil {
		t.Fatalf("hash de la contrasena: %v", err)
	}
	usuario := &domain.User{
		Email: correo, PasswordHash: hash,
		Role: domain.RoleStudent, Status: domain.UserStatusActive,
	}
	if err := r.usuarios.Create(context.Background(), usuario); err != nil {
		t.Fatalf("crear el usuario: %v", err)
	}
	return correo, clave
}

func (r *replicas) inicioDeSesion(t *testing.T, correo, clave string) string {
	t.Helper()
	res, err := r.a.Login(context.Background(), auth.LoginInput{Email: correo, Password: clave})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	return res.Token
}

// Esto ya funcionaba antes de A4, porque Authenticate cae a la base. La prueba
// esta para que siga funcionando: es el criterio de aceptacion que dice que
// iniciar sesion en una replica y operar en otra no obliga a reautenticar.
func TestLaSesionValeEnCualquierReplica(t *testing.T) {
	r := nuevasReplicas(t)
	correo, clave := r.cuentaActiva(t)
	token := r.inicioDeSesion(t, correo, clave)

	if _, _, err := r.b.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("la replica que no atendio el login rechazo la sesion: %v", err)
	}
}

// El defecto, y la prueba de que la cache compartida lo cierra.
//
// Con una cache por replica, la B tendria su propia copia marcada como usable y
// seguiria sirviendo la sesion hasta que expirara el TTL.
func TestElCierreDeSesionValeEnTodasLasReplicas(t *testing.T) {
	r := nuevasReplicas(t)
	correo, clave := r.cuentaActiva(t)
	token := r.inicioDeSesion(t, correo, clave)

	// La replica B la usa primero, asi que la tiene cacheada. Sin este paso la
	// prueba pasaria por vacuidad: B iria a la base y veria la revocacion.
	if _, _, err := r.b.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("preparacion: la replica B no acepto la sesion: %v", err)
	}

	if err := r.a.Logout(context.Background(), token); err != nil {
		t.Fatalf("Logout en la replica A: %v", err)
	}

	_, _, err := r.b.Authenticate(context.Background(), token)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("la replica B siguio aceptando una sesion cerrada: err = %v", err)
	}
}

// Misma historia para el cierre de todas las sesiones de un usuario, que es lo
// que usa la administracion al suspender una cuenta.
func TestRevocarTodasLasSesionesValeEnTodasLasReplicas(t *testing.T) {
	r := nuevasReplicas(t)
	correo, clave := r.cuentaActiva(t)

	primera := r.inicioDeSesion(t, correo, clave)
	segunda := r.inicioDeSesion(t, correo, clave)

	for _, token := range []string{primera, segunda} {
		if _, _, err := r.b.Authenticate(context.Background(), token); err != nil {
			t.Fatalf("preparacion: la replica B no acepto la sesion: %v", err)
		}
	}

	sesion, _, err := r.a.Authenticate(context.Background(), primera)
	if err != nil {
		t.Fatalf("Authenticate en la replica A: %v", err)
	}

	// Revocar una por una desde la replica A, que es lo que hace la pantalla de
	// sesiones del usuario. La administracion usa DeleteAllForUser, que recorre
	// el mismo camino.
	activas, err := r.a.ListSessions(context.Background(), sesion.UserID)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	for _, activa := range activas {
		if err := r.a.RevokeSession(context.Background(), sesion.UserID, activa.ID); err != nil {
			t.Fatalf("RevokeSession %s: %v", activa.ID, err)
		}
	}

	for i, token := range []string{primera, segunda} {
		if _, _, err := r.b.Authenticate(context.Background(), token); !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("sesion %d: la replica B la siguio aceptando: err = %v", i+1, err)
		}
	}
}

// Ninguna instancia conserva estado de sesion propio: con la cache compartida
// caida, las dos replicas siguen resolviendo contra la base.
//
// Esto es lo que hace que una cache compartida sea aceptable. Si la caida de
// Redis dejara a todo el mundo sin sesion, sacarla de las instancias habria
// cambiado un fallo de seguridad por un punto unico de fallo.
func TestSinCacheLasReplicasSiguenResolviendoContraLaBase(t *testing.T) {
	r := nuevasReplicas(t)
	correo, clave := r.cuentaActiva(t)
	token := r.inicioDeSesion(t, correo, clave)

	r.cache.setDisabled(true)

	for nombre, replica := range map[string]*auth.Service{"A": r.a, "B": r.b} {
		if _, _, err := replica.Authenticate(context.Background(), token); err != nil {
			t.Errorf("replica %s con la cache caida: %v", nombre, err)
		}
	}
}

// Suspender una cuenta corta sus sesiones en todas las replicas, incluso en las
// que la tienen cacheada.
//
// Funciona por una razon distinta a las de arriba, y conviene que este fijada:
// Authenticate lee el usuario de la base en **cada** peticion, no de la cache.
// Lo unico que se cachea es la sesion. Asi que una suspension surte efecto en
// la siguiente peticion de cualquier replica, con cache compartida o sin ella.
//
// Es lo que hace `internal/admin` al suspender, y lo que el issue pide revisar
// «con el mismo criterio» que el cierre de sesion.
func TestSuspenderLaCuentaCortaLaSesionEnTodasLasReplicas(t *testing.T) {
	r := nuevasReplicas(t)
	correo, clave := r.cuentaActiva(t)
	token := r.inicioDeSesion(t, correo, clave)

	// Que la replica B la tenga cacheada, o la prueba no diria nada.
	sesion, usuario, err := r.b.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("preparacion: la replica B no acepto la sesion: %v", err)
	}
	_ = sesion

	usuario.Status = domain.UserStatusSuspended
	if err := r.usuarios.Update(context.Background(), usuario); err != nil {
		t.Fatalf("suspender la cuenta: %v", err)
	}

	for nombre, replica := range map[string]*auth.Service{"A": r.a, "B": r.b} {
		if _, _, err := replica.Authenticate(context.Background(), token); !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("replica %s siguio aceptando la sesion de una cuenta suspendida: err = %v", nombre, err)
		}
	}
}

// El reverso de la anterior: la suspension no depende de la cache compartida.
// Si dependiera, sacarla de las instancias habria sido un cambio de riesgo y no
// una correccion.
func TestLaSuspensionNoDependeDeLaCache(t *testing.T) {
	r := nuevasReplicas(t)
	correo, clave := r.cuentaActiva(t)
	token := r.inicioDeSesion(t, correo, clave)

	_, usuario, err := r.b.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("preparacion: %v", err)
	}

	r.cache.setDisabled(true)
	usuario.Status = domain.UserStatusSuspended
	if err := r.usuarios.Update(context.Background(), usuario); err != nil {
		t.Fatalf("suspender la cuenta: %v", err)
	}

	if _, _, err := r.b.Authenticate(context.Background(), token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("con la cache caida la suspension no surtio efecto: err = %v", err)
	}
}
