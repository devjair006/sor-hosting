package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"github.com/go-chi/chi/v5"
	"net"
	"net/http"
	"sor.hosting/backend/internal/config"
	"sor.hosting/backend/internal/database/sqlc"
	"sor.hosting/backend/internal/httpx"
	"strings"
	"sync"
	"time"
)

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}
type identity struct {
	User  User
	Token string
}
type identityKey struct{}
type attempt struct {
	N     int
	Until time.Time
}
type Service struct {
	DB        *sql.DB
	Q         *sqlc.Queries
	Cfg       config.Config
	Clock     func() time.Time
	mu        sync.Mutex
	Attempts  map[string]attempt
	hashSlots chan struct{}
}

func NewService(db *sql.DB, cfg config.Config) *Service {
	return &Service{DB: db, Q: sqlc.New(db), Cfg: cfg, Clock: time.Now, Attempts: map[string]attempt{}, hashSlots: make(chan struct{}, 2)}
}
func RandomID() string {
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		panic("secure random unavailable")
	}
	return hex.EncodeToString(b)
}
func HashToken(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
func CurrentUser(ctx context.Context) (User, bool) {
	v, ok := ctx.Value(identityKey{}).(identity)
	return v.User, ok
}
func (s *Service) cookieName() string {
	if s.Cfg.Mode == "development" {
		return "sor_dev_session"
	}
	return "__Host-sor_session"
}
func (s *Service) setCookie(w http.ResponseWriter, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: s.cookieName(), Value: value, Path: "/", Secure: s.Cfg.Mode != "development", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: age})
}
func (s *Service) csrf(token string) string {
	h := hmac.New(sha256.New, s.Cfg.Key)
	h.Write([]byte("csrf:" + token))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
func (s *Service) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := r.Cookie(s.cookieName())
		if e != nil || len(c.Value) != 64 {
			httpx.Error(w, r, 401, "UNAUTHENTICATED", "Inicia sesión para continuar.")
			return
		}
		ss, e := s.Q.Session(r.Context(), HashToken(c.Value))
		now := s.Clock().Unix()
		if e != nil || ss.ExpiresAt <= now || ss.LastSeenAt+1800 <= now {
			httpx.Error(w, r, 401, "UNAUTHENTICATED", "Tu sesión ha caducado.")
			return
		}
		u, e := s.Q.UserByID(r.Context(), ss.UserID)
		if e != nil {
			httpx.Error(w, r, 401, "UNAUTHENTICATED", "Sesión inválida.")
			return
		}
		if e = s.Q.TouchSession(r.Context(), sqlc.TouchSessionParams{LastSeenAt: now, TokenHash: ss.TokenHash}); e != nil {
			httpx.Error(w, r, 503, "DATABASE_UNAVAILABLE", "No se pudo validar la sesión.")
			return
		}
		ctx := context.WithValue(r.Context(), identityKey{}, identity{User: User{u.ID, u.Email, u.Role}, Token: c.Value})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (s *Service) RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			v, ok := r.Context().Value(identityKey{}).(identity)
			if !ok || r.Header.Get("Origin") != s.Cfg.Origin || !hmac.Equal([]byte(r.Header.Get("X-CSRF-Token")), []byte(s.csrf(v.Token))) {
				httpx.Error(w, r, 403, "CSRF_REJECTED", "No se pudo verificar la solicitud.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Service) limited(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.Clock()
	if len(s.Attempts) > 10000 {
		for k, a := range s.Attempts {
			if now.After(a.Until) {
				delete(s.Attempts, k)
			}
		}
		if len(s.Attempts) > 10000 {
			return true
		}
	}
	a := s.Attempts[key]
	if now.After(a.Until) {
		a = attempt{Until: now.Add(15 * time.Minute)}
	}
	a.N++
	s.Attempts[key] = a
	return a.N > 10
}
func (s *Service) Login(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != s.Cfg.Origin {
		httpx.Error(w, r, 403, "CSRF_REJECTED", "Origen no autorizado.")
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if s.limited("ip:"+ip) || s.limited("user:"+strings.ToLower(in.Email)) {
		httpx.Error(w, r, 429, "RATE_LIMITED", "Demasiados intentos. Inténtalo más tarde.")
		return
	}
	select {
	case s.hashSlots <- struct{}{}:
		defer func() { <-s.hashSlots }()
	default:
		httpx.Error(w, r, 429, "RATE_LIMITED", "Inténtalo de nuevo en unos segundos.")
		return
	}
	u, e := s.Q.UserByEmail(r.Context(), strings.TrimSpace(strings.ToLower(in.Email)))
	hash := u.PasswordHash
	if e != nil {
		hash = "$argon2id$v=19$m=65536,t=3,p=1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	valid := VerifyPassword(hash, in.Password)
	if e != nil || !valid {
		httpx.Error(w, r, 401, "INVALID_CREDENTIALS", "Correo o contraseña incorrectos.")
		return
	}
	tokenBytes := make([]byte, 32)
	if _, e = rand.Read(tokenBytes); e != nil {
		httpx.Error(w, r, 500, "INTERNAL_ERROR", "No se pudo iniciar sesión.")
		return
	}
	token := hex.EncodeToString(tokenBytes)
	now := s.Clock().Unix()
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		httpx.Error(w, r, 503, "DATABASE_UNAVAILABLE", "Inténtalo más tarde.")
		return
	}
	defer tx.Rollback()
	q := s.Q.WithTx(tx)
	if old, e := r.Cookie(s.cookieName()); e == nil {
		_ = q.DeleteSession(r.Context(), HashToken(old.Value))
	}
	e = q.CreateSession(r.Context(), sqlc.CreateSessionParams{TokenHash: HashToken(token), UserID: u.ID, CreatedAt: now, LastSeenAt: now, ExpiresAt: now + 43200})
	if e == nil {
		e = q.InsertAudit(r.Context(), sqlc.InsertAuditParams{ID: RandomID(), UserID: u.ID, Action: "auth.login", ResourceID: u.ID, Outcome: "succeeded", CreatedAt: s.Clock().UTC().Format(time.RFC3339)})
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		httpx.Error(w, r, 503, "DATABASE_UNAVAILABLE", "No se pudo guardar la sesión.")
		return
	}
	s.setCookie(w, token, 43200)
	httpx.JSON(w, 200, map[string]any{"user": User{u.ID, u.Email, u.Role}})
}
func RegisterRoutes(r chi.Router, s *Service) {
	r.Post("/auth/login", s.Login)
	r.Group(func(r chi.Router) {
		r.Use(s.RequireSession, s.RequireCSRF)
		r.Get("/auth/me", func(w http.ResponseWriter, r *http.Request) {
			u, _ := CurrentUser(r.Context())
			httpx.JSON(w, 200, map[string]any{"user": u})
		})
		r.Get("/auth/csrf", func(w http.ResponseWriter, r *http.Request) {
			v := r.Context().Value(identityKey{}).(identity)
			httpx.JSON(w, 200, map[string]string{"csrf_token": s.csrf(v.Token)})
		})
		r.Post("/auth/logout", s.Logout)
		r.Post("/auth/password", s.ChangePassword)
	})
}
func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	v := r.Context().Value(identityKey{}).(identity)
	if e := s.Q.DeleteSession(r.Context(), HashToken(v.Token)); e != nil {
		httpx.Error(w, r, 503, "DATABASE_UNAVAILABLE", "No se pudo cerrar la sesión.")
		return
	}
	s.setCookie(w, "", -1)
	w.WriteHeader(204)
}
func (s *Service) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	select {
	case s.hashSlots <- struct{}{}:
		defer func() { <-s.hashSlots }()
	default:
		httpx.Error(w, r, 429, "RATE_LIMITED", "Inténtalo más tarde.")
		return
	}
	u, _ := CurrentUser(r.Context())
	row, e := s.Q.UserByID(r.Context(), u.ID)
	if e != nil || !VerifyPassword(row.PasswordHash, in.Current) {
		httpx.Error(w, r, 422, "INVALID_CREDENTIALS", "Contraseña actual incorrecta.")
		return
	}
	h, e := HashPassword(in.New)
	if e != nil {
		httpx.Error(w, r, 422, "VALIDATION_ERROR", e.Error())
		return
	}
	tx, e := s.DB.BeginTx(r.Context(), nil)
	if e != nil {
		httpx.Error(w, r, 503, "DATABASE_UNAVAILABLE", "Inténtalo más tarde.")
		return
	}
	defer tx.Rollback()
	q := s.Q.WithTx(tx)
	e = q.ChangePassword(r.Context(), sqlc.ChangePasswordParams{PasswordHash: h, ID: u.ID})
	if e == nil {
		e = q.DeleteUserSessions(r.Context(), u.ID)
	}
	if e == nil {
		e = q.InsertAudit(r.Context(), sqlc.InsertAuditParams{ID: RandomID(), UserID: u.ID, Action: "auth.password", ResourceID: u.ID, Outcome: "succeeded", CreatedAt: s.Clock().UTC().Format(time.RFC3339)})
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		httpx.Error(w, r, 503, "DATABASE_UNAVAILABLE", "No se pudo guardar el cambio.")
		return
	}
	s.setCookie(w, "", -1)
	w.WriteHeader(204)
}
