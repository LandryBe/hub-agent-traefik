package basicauth

import (
	"fmt"
	"net/http"
	"strings"

	goauth "github.com/abbot/go-http-auth"
	"github.com/rs/zerolog/log"
	"github.com/traefik/hub-agent-traefik/pkg/edge"
)

const defaultRealm = "hub"

// Users holds a list of users.
type Users []string

// Handler is a basic auth ACP Handler.
type Handler struct {
	auth               *goauth.BasicAuth
	users              map[string]string
	forwardUsername    string
	stripAuthorization bool
	name               string
}

// NewHandler creates a new basic auth ACP Handler.
func NewHandler(cfg *edge.ACPBasicAuthConfig, name string) (*Handler, error) {
	users, err := getUsers(cfg.Users, basicUserParser)
	if err != nil {
		return nil, err
	}

	h := &Handler{
		users:              users,
		forwardUsername:    cfg.ForwardUsernameHeader,
		stripAuthorization: cfg.StripAuthorizationHeader,
		name:               name,
	}

	realm := defaultRealm
	if len(cfg.Realm) > 0 {
		realm = cfg.Realm
	}

	h.auth = &goauth.BasicAuth{Realm: realm, Secrets: h.secretBasic}

	return h, nil
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	logger := log.With().Str("handler_type", "BasicAuth").Str("handler_name", h.name).Logger()

	username, password, ok := req.BasicAuth()
	if ok {
		secret := h.auth.Secrets(username, h.auth.Realm)
		if secret == "" || !goauth.CheckSecret(password, secret) {
			ok = false
		}
	}

	if !ok {
		logger.Debug().Msg("Authentication failed")

		h.auth.RequireAuth(rw, req)
		return
	}

	if h.forwardUsername != "" {
		rw.Header().Set(h.forwardUsername, username)
	}

	if h.stripAuthorization {
		rw.Header().Add("Authorization", "")
	}

	rw.WriteHeader(http.StatusOK)
}

func (h *Handler) secretBasic(user, _ string) string {
	if secret, ok := h.users[user]; ok {
		return secret
	}

	return ""
}

func basicUserParser(user string) (username, password string, err error) {
	split := strings.Split(user, ":")
	if len(split) != 2 {
		return "", "", fmt.Errorf("parse BasicUser: %v", user)
	}
	return split[0], split[1], nil
}

// userParser parses a string and returns a userName/userHash. It returns an error for invalid string format.
type userParser func(user string) (username, password string, err error)

func getUsers(users []string, parser userParser) (map[string]string, error) {
	userMap := make(map[string]string)
	for _, user := range users {
		userName, userHash, err := parser(user)
		if err != nil {
			return nil, err
		}
		userMap[userName] = userHash
	}

	return userMap, nil
}
