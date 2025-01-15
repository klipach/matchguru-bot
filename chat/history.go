package chat

import (
	"context"
	"fmt"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/firestore"
	"github.com/klipach/matchguru/logger"
	"github.com/tmc/langchaingo/llms"
)

const (
	firestoreUserCollection = "users"
	fromUser                = "User"
	fromAI                  = "AI"
)

type firestoreMessage struct {
	From    string `firestore:"from"`
	Message string `firestore:"message"`
}

type firestoreUser struct {
	DisplayName string `firestore:"display_name"`
	Chats       []struct {
		ChatID  int `firestore:"chat_id"`
		Messages []firestoreMessage `firestore:"messages"`
	} `firestore:"chats"`
}

func LoadChatHistory(ctx context.Context, userID string, chatID int) ([]llms.MessageContent, error) {
	logger := logger.FromContext(ctx)

	var chatHistory []llms.MessageContent

	projectID, err := metadata.ProjectIDWithContext(ctx)
	if err != nil {
		return chatHistory, err
	}

	firestoreClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return chatHistory, err
	}
	defer firestoreClient.Close()

	userDoc, err := firestoreClient.Collection(firestoreUserCollection).Doc(userID).Get(ctx)
	if err != nil {
		return chatHistory, err
	}
	if !userDoc.Exists() {
		logger.Printf("user not found: %s", userID)
		return chatHistory, nil
	}

	user := firestoreUser{}
	userDoc.DataTo(&user)

	var messages []firestoreMessage
	chatFound := false
	for _, c := range user.Chats {
		if c.ChatID == chatID {
			messages = c.Messages
			logger.Printf("chat found: %d", chatID)
			chatFound = true
			break
		}
	}
	if !chatFound {
		logger.Printf("chat not found: %d", chatID)
		return chatHistory, nil
	}

	for _, m := range messages {
		switch m.From {
		case fromUser:
			chatHistory = append(chatHistory, llms.TextParts(llms.ChatMessageTypeHuman, m.Message))
		case fromAI:
			chatHistory = append(chatHistory, llms.TextParts(llms.ChatMessageTypeAI, m.Message))
		default:
			return chatHistory, fmt.Errorf("invalid message role: %s", m.From)
		}
	}
	return chatHistory, nil
}
