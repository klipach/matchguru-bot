package auth

import (
	"net/http"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

func Authenticate(req *http.Request) (*auth.Token, error) {
	ctx := req.Context()
	app, err := firebase.NewApp(ctx, nil)
	if err != nil {
		return nil, err
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, err
	}

	jwtToken, err := bearerTokenFromRequest(req)
	if err != nil {
		return nil, err
	}
	return client.VerifyIDToken(ctx, jwtToken)
}
