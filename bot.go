package matchguru

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"text/template"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/firestore"
	"cloud.google.com/go/logging"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	openai "github.com/sashabaranov/go-openai"
)

const (
	fromUser            = "User"
	fromAI              = "AI"
	gcloudFuncSourceDir = "serverless_function_source_code"
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
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	logger := initLogger(ctx)

	logger.Println("bot function called")
	if r.Method != http.MethodPost {
		logger.Printf("invalid method: %s", r.Method)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	systemPromptTemplate, err := template.New("prompt.tmpl").ParseFiles("prompt.tmpl")
	if err != nil {
		logger.Printf("error while parsing systemPromptTemplate: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	projectID, err := metadata.ProjectIDWithContext(ctx)
	if err != nil {
		logger.Fatalf("failed to get project ID: %v", err)
	}

	firestoreClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		logger.Printf("failed to initiate firestore client: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer firestoreClient.Close()

	openaiClient := openai.NewClient(openaiAPIKey)

	var msg MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		logger.Printf("error while decoding request: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if msg.UserId == "" {
		logger.Print("missing user_id")
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	user, err := firestoreClient.Collection("users").Doc(msg.UserId).Get(ctx)
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

	firestoreUser := FirestoreUser{}
	user.DataTo(&firestoreUser)

	var result bytes.Buffer
	err = systemPromptTemplate.Execute(&result, struct{ Username string }{Username: firestoreUser.DisplayName})
	if err != nil {
		logger.Printf("error while executing systemPromptTemplate: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: result.String(),
		},
	}

	chatId := msg.ChatId
	if len(firestoreUser.Chats) > chatId {
		for _, msg := range firestoreUser.Chats[chatId].Messages {
			switch msg.From {
			case fromUser:
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: msg.Message,
				})
			case fromAI:
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: msg.Message,
				})
			default:
				logger.Printf("invalid message role: %s", msg.From)
			}
		}
	}

	completion, err := openaiClient.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: append(
				messages,
				openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: msg.Message,
				},
			),
		},
	)

	if err != nil {
		logger.Printf("ChatCompletion error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	resp := MessageResponse{
		Response: completion.Choices[0].Message.Content,
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
