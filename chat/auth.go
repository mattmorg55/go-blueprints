package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	oidc "github.com/coreos/go-oidc"
	objx "github.com/stretchr/objx"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

type authHandler struct {
	next http.Handler
}

type loginHandler struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	config   oauth2.Config
	context  context.Context
}

var (
	// clientID     = os.Getenv("OIDC_CLIENT_ID")
	// clientSecret = os.Getenv("OIDC_CLIENT_SECRET")
	clientID     = "go-client"
	clientSecret = "Test1234"
	state        = "foobar" // don't do this in prod
)

func newLoginHandler(c context.Context) *loginHandler {
	handler := &loginHandler{context: c}
	provider, err := oidc.NewProvider(c, "https://staging-auth-server.sis-online.org")
	if err != nil {
		log.Fatal(err)
	}
	handler.provider = provider
	oidcConfig := &oidc.Config{
		ClientID: clientID,
	}
	handler.verifier = provider.Verifier(oidcConfig)
	handler.config = oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  "http://localhost:8080/auth/callback/ajboggs",
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	return handler
}

func (h *authHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, err := r.Cookie("auth")
	if err == http.ErrNoCookie {
		// not authenticated
		w.Header().Set("Location", "/login")
		w.WriteHeader(http.StatusTemporaryRedirect)
		return
	}
	if err != nil {
		// some other error
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// success - call the next handler
	h.next.ServeHTTP(w, r)
}

// MustAuth adapts handler to ensure authentication has occurred.
func MustAuth(handler http.Handler) http.Handler {
	return &authHandler{next: handler}
}

// loginHandler handles the third-party login process.
// format: /auth/{action}/{provider}
func (h *loginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	segs := strings.Split(r.URL.Path, "/")
	action := segs[2]
	//provider := segs[3]
	switch action {
	case "login":
		http.Redirect(w, r, h.config.AuthCodeURL(state), http.StatusFound)
	case "callback":
		h.handleCallback(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Auth action %s not supported", action)
	}
}

func (h *loginHandler) handleCallback(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling callback from auth provider.")
	if r.URL.Query().Get("state") != state {
		http.Error(w, "state did not match", http.StatusBadRequest)
		return
	}

	oauth2Token, err := h.config.Exchange(h.context, r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "No id_token field in oauth2 token.", http.StatusInternalServerError)
		return
	}
	idToken, err := h.verifier.Verify(h.context, rawIDToken)
	if err != nil {
		http.Error(w, "Failed to verify ID Token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := struct {
		OAuth2Token   *oauth2.Token
		IDTokenClaims *json.RawMessage // ID Token payload is just JSON.
	}{oauth2Token, new(json.RawMessage)}

	if err := idToken.Claims(&resp.IDTokenClaims); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Print(string(data))
	var claims struct {
		Name string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	authCookieValue := objx.New(map[string]interface{}{
		"name": claims.Name,
	}).MustBase64()
	http.SetCookie(w, &http.Cookie{
		Name:  "auth",
		Value: authCookieValue,
		Path:  "/",
	})
	http.Redirect(w, r, "/chat", http.StatusTemporaryRedirect)
}
