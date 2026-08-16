package middleware

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/seakee/cpa-manager-plus/apps/manager-server/internal/http/response"
)

type AdminVerifier interface {
	VerifyHeader(ctx context.Context, authorizationHeader string) (bool, error)
}

func LoopbackHostOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.TrimSpace(r.Host)
		if parsedHost, _, err := net.SplitHostPort(host); err == nil {
			host = parsedHost
		}
		host = strings.Trim(strings.TrimSpace(host), "[]")
		ip := net.ParseIP(host)
		if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
			response.Error(w, http.StatusForbidden, errors.New("passwordless access requires a loopback host"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type PanelVerifier interface {
	VerifyPanelHeader(ctx context.Context, authorizationHeader string) (bool, error)
	PanelUsesExternalManagementKey(ctx context.Context) (bool, error)
}

func AuthorizeAdmin(w http.ResponseWriter, r *http.Request, verifier AdminVerifier) bool {
	ok, err := verifier.VerifyHeader(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err)
		return false
	}
	if ok {
		return true
	}
	response.Error(w, http.StatusUnauthorized, errors.New("invalid admin key"))
	return false
}

func AuthorizePanel(w http.ResponseWriter, r *http.Request, verifier PanelVerifier) bool {
	ok, err := verifier.VerifyPanelHeader(r.Context(), r.Header.Get("Authorization"))
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err)
		return false
	}
	if ok {
		return true
	}
	external, err := verifier.PanelUsesExternalManagementKey(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err)
		return false
	}
	if external {
		response.Error(w, http.StatusUnauthorized, errors.New("invalid management key"))
		return false
	}
	response.Error(w, http.StatusUnauthorized, errors.New("invalid admin key"))
	return false
}
