package auth

import (
	"errors"
	"net/http"
	"strings"
)

const (
	authorizationHeader = "Authorization"
	bearerPrefix        = "Bearer "
)

var (
	errMissingAuthorizationHeader = errors.New("missing Authorization header")
	errInvalidAuthorizationHeader = errors.New("invalid Authorization header")
)

func parseBearerToken(r *http.Request) (string, error) {
	reqToken := r.Header.Get(authorizationHeader)
	if reqToken == "" {
		return "", errMissingAuthorizationHeader
	}
	splitToken := strings.Split(reqToken, bearerPrefix)
	if len(splitToken) != 2 {
		return "", errInvalidAuthorizationHeader
	}
	return strings.TrimSpace(splitToken[1]), nil
}
