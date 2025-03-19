package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"

	firebase "firebase.google.com/go"
	"google.golang.org/api/option"
)

type SignInResponse struct {
	IDToken      string `json:"idToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    string `json:"expiresIn"`
	LocalID      string `json:"localId"`
}

func main() {
	ctx := context.Background()
	uidPtr := flag.String("uid", "", "User UID for token generation")
	apiKeyPtr := flag.String("apikey", "", "Firebase API key for Identity Toolkit REST API")
	flag.Parse()

	if *uidPtr == "" {
		log.Fatalf("Please provide a user UID using the -uid flag")
	}

	absPath, err := filepath.Abs("./service_account_key.json")
	if err != nil {
		log.Fatalf("failed to get absolute path: %v", err)
	}
	opt := option.WithCredentialsFile(absPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		log.Fatalf("error initializing app: %v", err)
	}

	client, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("error getting Auth client: %v", err)
	}

	// Generate a custom token
	customToken, err := client.CustomToken(ctx, *uidPtr)
	if err != nil {
		log.Fatalf("error creating custom token: %v", err)
	}

	// Exchange custom token for an ID token using Firebase's REST API
	url := fmt.Sprintf("https://identitytoolkit.googleapis.com/v1/accounts:signInWithCustomToken?key=%s", *apiKeyPtr)
	payload := map[string]any{
		"token":             customToken,
		"returnSecureToken": true,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("error marshaling payload: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.Fatalf("error making POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Fatalf("non-OK HTTP status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("error reading response body: %v", err)
	}

	var signInResp SignInResponse
	if err := json.Unmarshal(body, &signInResp); err != nil {
		log.Fatalf("error unmarshalling response: %v", err)
	}

	fmt.Println(signInResp.IDToken)
}
