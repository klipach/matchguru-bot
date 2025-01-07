package matchguru

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/firestore"
	"cloud.google.com/go/logging"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/klipach/matchguru/auth"
	"github.com/klipach/matchguru/contract"
	"github.com/klipach/matchguru/game"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

const (
	fromUser            = "User"
	fromAI              = "AI"
	gcloudFuncSourceDir = "serverless_function_source_code"
)

var (
	openaiAPIKey     = os.Getenv("OPENAI_API_KEY")
	perplexityApiKey = os.Getenv("PERPLEXITY_API_KEY")
)

func init() {
	functions.HTTP("Bot", Bot)
	fixDir()
}

// in GCP Functions, source code is placed in a directory named "serverless_function_source_code"
// need to change the dir to get access to template file
func fixDir() {
	fileInfo, err := os.Stat(gcloudFuncSourceDir)
	if err == nil && fileInfo.IsDir() {
		_ = os.Chdir(gcloudFuncSourceDir)
	}
}

func Bot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := initLogger(ctx)

	logger.Println("bot function called")

	token, err := auth.Authenticate(r)
	if err != nil {
		logger.Printf("error while authenticating: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userID := token.UID

	if r.Method != http.MethodPost {
		logger.Printf("invalid method: %s", r.Method)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	mainPrompt, err := template.New("main.tmpl").ParseFiles("prompts/main.tmpl")
	if err != nil {
		logger.Printf("error while parsing mainPrompt: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	projectID, err := metadata.ProjectIDWithContext(ctx)
	if err != nil {
		logger.Printf("failed to get project ID: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	firestoreClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		logger.Printf("failed to initiate firestore client: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer firestoreClient.Close()

	var msg MessageRequest
	data, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Printf("error while reading request body: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	logger.Printf("message: %v", string(data))

	if err := json.Unmarshal(data, &msg); err != nil {
		logger.Printf("error while decoding request: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// test message:
	if strings.TrimSpace(msg.Message) == "test" {
		resp := MessageResponse{
			Response: `<b> hi there</b>
			<a href="/team/346">Team</a>
			<a href="/player/4237">Player</a>
			<a href="/league/384">League</a>
			<a href="/fixture/19155228">Fixture</a>
			<s>strikethrough</s>
			<i>italic</i>
			`,
		}
		err = json.NewEncoder(w).Encode(resp)
		if err != nil {
			logger.Printf("error while encoding response: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	gg := &game.Game{}
	if msg.GameID != 0 {
		gg, err = game.Fetch(ctx, msg.GameID)
		if err != nil {
			logger.Printf("error while fetching game: %v", err)
		}
		logger.Printf("game fetched %v+", gg)
	}

	user, err := firestoreClient.Collection("users").Doc(userID).Get(ctx)
	if err != nil {
		logger.Printf("error while getting user: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !user.Exists() {
		logger.Print("user not found")
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	logger.Print("user found")

	firestoreUser := contract.FirestoreUser{}
	user.DataTo(&firestoreUser)

	var mainPromptStr strings.Builder
	err = mainPrompt.Execute(
		&mainPromptStr,
		struct {
			Username       string
			Today          string
			GameName       string
			GameStartingAt time.Time
			GameLeague     string
			Season         string
		}{
			Username:       firestoreUser.DisplayName,
			Today:          time.Now().Format("2006-01-02"),
			GameName:       gg.Name,
			GameStartingAt: gg.StartingAt,
			GameLeague:     gg.League,
			Season:         gg.Season,
		})
	if err != nil {
		logger.Printf("error while executing mainPrompt: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shortPrompt, err := template.New("short.tmpl").ParseFiles("prompts/short.tmpl")
	if err != nil {
		logger.Printf("error while parsing shortPrompt: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	var shortPromptStr strings.Builder
	err = shortPrompt.Execute(
		&shortPromptStr,
		struct {
			Today          string
			GameName       string
			GameStartingAt time.Time
			GameLeague     string
			Season         string
		}{
			Today:          time.Now().Format("2006-01-02"),
			GameName:       gg.Name,
			GameStartingAt: gg.StartingAt,
			GameLeague:     gg.League,
			Season:         gg.Season,
		})

	if err != nil {
		logger.Printf("error while executing shortPrompt: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var messages []llms.MessageContent
	chatID := msg.ChatID
	if len(firestoreUser.Chats) > chatID {
		logger.Printf("chat found: %d", chatID)
		for _, m := range firestoreUser.Chats[chatID].Messages {
			switch m.From {
			case fromUser:
				messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, m.Message))
			case fromAI:
				messages = append(messages, llms.TextParts(llms.ChatMessageTypeAI, m.Message))
			default:
				logger.Printf("invalid message role: %s", m.From)
			}
		}
	} else {
		logger.Printf("chat not found: %d", chatID)
	}
	messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, msg.Message))
	logger.Printf("messages: %v", messages)

	gpt4Turbo, err := openai.New(
		openai.WithModel("gpt-4-turbo"),
		openai.WithToken(openaiAPIKey),
	)
	if err != nil {
		logger.Printf("error while creating gpt4Turbo: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shortResp, err := gpt4Turbo.GenerateContent(ctx, messages, llms.WithMaxTokens(1000))
	if err != nil {
		logger.Printf("CreateChatCompletion short error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	logger.Printf("short response: %s", shortResp.Choices[0].Content)
	if strings.ToLower(shortResp.Choices[0].Content) != "no" {
		logger.Printf("external knowledge required")
		perplexityClient, err := openai.New(
			// Supported models: https://docs.perplexity.ai/docs/model-cards
			openai.WithModel("llama-3.1-sonar-small-128k-online"),
			openai.WithBaseURL("https://api.perplexity.ai"),
			openai.WithToken(perplexityApiKey),
		)
		if err != nil {
			logger.Printf("error while creating perplexity client: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		perplexityPrompt, err := template.New("perplexity.tmpl").ParseFiles("prompts/perplexity.tmpl")
		if err != nil {
			logger.Printf("error while parsing perplexityPrompt: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		var perplexityPromptStr strings.Builder
		err = perplexityPrompt.Execute(&perplexityPromptStr, struct{ Today string }{Today: time.Now().Format("2006-01-02")})
		if err != nil {
			logger.Printf("error while executing perplexityPrompt: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		perplexityResp, err := perplexityClient.GenerateContent(ctx,
			[]llms.MessageContent{
				llms.TextParts(llms.ChatMessageTypeSystem, perplexityPromptStr.String()),
				llms.TextParts(llms.ChatMessageTypeHuman, shortResp.Choices[0].Content),
			},
			llms.WithMaxTokens(1000),
		)
		if err != nil {
			logger.Printf("error while generating from single prompt: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		logger.Printf("perplexity response: %s", perplexityResp.Choices[0].Content)

		mainPromptStr.WriteString("Additional info for the request: " + perplexityResp.Choices[0].Content)
	}

	completion, err := gpt4Turbo.GenerateContent(
		ctx,
		append(
			[]llms.MessageContent{
				llms.TextParts(llms.ChatMessageTypeSystem, mainPromptStr.String()),
			},
			messages...,
		),
	)

	if err != nil {
		logger.Printf("ChatCompletion error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	resp := MessageResponse{
		Response: completion.Choices[0].Content,
	}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		logger.Printf("error while encoding response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func initLogger(ctx context.Context) *log.Logger {
	projectID, err := metadata.ProjectIDWithContext(ctx)
	if err != nil {
		log.Fatalf("failed to get project ID: %v", err)
	}
	client, err := logging.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("failed to create logging client: %v", err)
	}
	return client.Logger("bot").StandardLogger(logging.Info)
}
