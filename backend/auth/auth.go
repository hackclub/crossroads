// Package auth signs people in with Hack Club Auth and keeps them signed in
// with a session cookie.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/scherepi/crossroads-backend/db"
	"github.com/scherepi/crossroads-backend/hca"
	"github.com/scherepi/crossroads-backend/respond"
)

const (
	sessionCookie = "crossroads_session"
	stateCookie   = "crossroads_oauth_state"
	sessionLength = 30 * 24 * time.Hour
)

type Auth struct {
	Queries *db.Queries
	HCA     *hca.Client
	// SecureCookies stops browsers sending cookies over plain HTTP. Leave it
	// off only for local development.
	SecureCookies bool
}

// Routes are mounted at /auth. The callback path has to match the redirect URL
// registered on the Hack Club Auth app.
func (a *Auth) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/login", a.login)
	r.Get("/hcacallback", a.callback)
	r.Post("/logout", a.logout)
	return r
}

// login sends the browser to Hack Club Auth. ?next= is where to land after
// signing in, and has to be a path on this site.
func (a *Auth) login(w http.ResponseWriter, r *http.Request) {
	state := rand.Text()
	next := safeNext(r.URL.Query().Get("next"))
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state + ":" + url.QueryEscape(next),
		Path:     "/auth/",
		MaxAge:   int((10 * time.Minute).Seconds()),
		HttpOnly: true,
		Secure:   a.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, a.HCA.AuthorizeURL(state), http.StatusFound)
}

func (a *Auth) callback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	cookie, err := r.Cookie(stateCookie)
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Path: "/auth/", MaxAge: -1})
	if err != nil {
		http.Error(w, "Sign-in timed out. Please try again.", http.StatusBadRequest)
		return
	}
	state, next, _ := strings.Cut(cookie.Value, ":")
	if state == "" || subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(state)) != 1 {
		http.Error(w, "Sign-in timed out. Please try again.", http.StatusBadRequest)
		return
	}
	if query.Get("error") != "" {
		http.Error(w, "Sign-in was cancelled.", http.StatusForbidden)
		return
	}

	identity, err := a.HCA.IdentityFromCode(r.Context(), query.Get("code"))
	if err != nil {
		log.Printf("signing in: %v", err)
		http.Error(w, "Couldn't reach Hack Club Auth. Please try again.", http.StatusBadGateway)
		return
	}
	var birthday *time.Time
	if t, err := time.Parse(time.DateOnly, identity.Birthday); err == nil {
		birthday = &t
	}
	user, err := a.Queries.UpsertUserFromHCA(r.Context(), db.UpsertUserFromHCAParams{
		HcaID:              identity.ID,
		SlackID:            nonEmpty(identity.SlackID),
		Email:              identity.PrimaryEmail,
		Name:               strings.TrimSpace(identity.FirstName + " " + identity.LastName),
		FirstName:          identity.FirstName,
		LastName:           identity.LastName,
		Birthday:           birthday,
		PhoneNumber:        nonEmpty(identity.PhoneNumber),
		Address:            primaryAddress(identity),
		VerificationStatus: identity.VerificationStatus,
		YswsEligible:       identity.YSWSEligible,
	})
	if err != nil {
		log.Printf("saving user %s: %v", identity.ID, err)
		http.Error(w, "Something went wrong signing in.", http.StatusInternalServerError)
		return
	}

	token, err := a.startSession(r.Context(), user.ID)
	if err != nil {
		log.Printf("starting session for user %d: %v", user.ID, err)
		http.Error(w, "Something went wrong signing in.", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionLength.Seconds()),
		HttpOnly: true,
		Secure:   a.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	next, _ = url.QueryUnescape(next)
	http.Redirect(w, r, safeNext(next), http.StatusFound)
}

// primaryAddress returns identity's primary mailing address as JSON for
// users.address, or nil if they haven't given Hack Club Auth one.
func primaryAddress(identity hca.Identity) []byte {
	if len(identity.Addresses) == 0 {
		return nil
	}
	a := identity.Addresses[0]
	for _, candidate := range identity.Addresses {
		if candidate.Primary {
			a = candidate
		}
	}
	address, _ := json.Marshal(db.Address{
		Name:       strings.TrimSpace(a.FirstName + " " + a.LastName),
		Line1:      a.Line1,
		Line2:      a.Line2,
		City:       a.City,
		State:      a.State,
		PostalCode: a.PostalCode,
		Country:    a.Country,
	})
	return address
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// startSession returns a new session token for the cookie. The database only
// stores its hash, so a leaked database can't be used to sign in as anyone.
func (a *Auth) startSession(ctx context.Context, userID int64) (string, error) {
	if err := a.Queries.DeleteExpiredSessions(ctx, userID); err != nil {
		return "", err
	}
	token := rand.Text()
	err := a.Queries.CreateSession(ctx, db.CreateSessionParams{
		TokenHash: hashToken(token),
		UserID:    userID,
		ExpiresAt: time.Now().Add(sessionLength),
	})
	return token, err
}

func (a *Auth) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if err := a.Queries.DeleteSession(r.Context(), hashToken(cookie.Value)); err != nil {
			log.Printf("deleting session: %v", err)
			http.Error(w, "Something went wrong signing out.", http.StatusInternalServerError)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

func hashToken(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

// safeNext returns next if it's a path on this site, and "/" otherwise, so
// sign-in can't be used to bounce someone to another site.
func safeNext(next string) string {
	u, err := url.Parse(next)
	if err != nil || u.Scheme != "" || u.Host != "" ||
		!strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/"
	}
	return next
}

type userKey struct{}

// LoadSession puts the signed-in user, if there is one, in the request
// context for User to find.
func (a *Auth) LoadSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(sessionCookie); err == nil {
			user, err := a.Queries.GetSessionUser(r.Context(), hashToken(cookie.Value))
			switch {
			case err == nil:
				r = r.WithContext(WithUser(r.Context(), &user))
			case errors.Is(err, pgx.ErrNoRows):
				// Signed out or expired: carry on as nobody.
			default:
				respond.ServerError(w, r, fmt.Errorf("loading session: %w", err))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// WithUser returns a copy of ctx with user signed in.
func WithUser(ctx context.Context, user *db.User) context.Context {
	return context.WithValue(ctx, userKey{}, user)
}

// User returns the signed-in user, or nil if nobody is signed in.
func User(ctx context.Context) *db.User {
	user, _ := ctx.Value(userKey{}).(*db.User)
	return user
}

// RequireUser responds 401 unless someone is signed in.
func RequireUser(next http.Handler) http.Handler {
	return RequireRole(db.UserRoleParticipant, db.UserRoleReviewer, db.UserRoleAdmin)(next)
}

// RequireRole responds 401 unless someone is signed in, and 403 unless they
// have one of roles.
func RequireRole(roles ...db.UserRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := User(r.Context())
			if user == nil {
				respond.Error(w, http.StatusUnauthorized, "Sign in first.")
				return
			}
			if !slices.Contains(roles, user.Role) {
				respond.Error(w, http.StatusForbidden, "You don't have access to this.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MeJSON is the signed-in user, as GET /api/me shows them.
type MeJSON struct {
	ID                 int64       `json:"id"`
	Name               string      `json:"name"`
	Email              string      `json:"email"`
	SlackID            *string     `json:"slack_id"`
	Role               db.UserRole `json:"role"`
	Stamps             int32       `json:"stamps"`
	VerificationStatus string      `json:"verification_status"`
	YSWSEligible       bool        `json:"ysws_eligible"`
	// Where physical items ship by default; null if Hack Club Auth has none.
	Address json.RawMessage `json:"address"`
}

func ToMeJSON(user *db.User) MeJSON {
	return MeJSON{user.ID, user.Name, user.Email, user.SlackID, user.Role, user.Stamps, user.VerificationStatus, user.YswsEligible, nullableJSON(user.Address)}
}

// Me responds with the signed-in user. Wrap it in RequireUser.
func Me(w http.ResponseWriter, r *http.Request) {
	respond.JSON(w, http.StatusOK, ToMeJSON(User(r.Context())))
}

// nullableJSON turns a missing jsonb value into a JSON null.
func nullableJSON(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("null")
	}
	return b
}
