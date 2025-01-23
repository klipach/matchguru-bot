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

type firestoreUser struct {
	DisplayName string           `firestore:"display_name"`
	Chats       []*firestoreChat `firestore:"chats"`
}

type firestoreMessage struct {
	From    string `firestore:"from"`
	Message string `firestore:"message"`
}

type firestoreChat struct {
	ChatID   int                 `firestore:"chat_id"`
	Messages []*firestoreMessage `firestore:"messages"`
}

func LoadHistory(ctx context.Context, userID string, chatID int) ([]llms.MessageContent, error) {
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

	messages := findChatMessages(user.Chats, chatID)
	if messages == nil {
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

func findChatMessages(chats []*firestoreChat, chatID int) []*firestoreMessage {
	for _, chat := range chats {
		if chat.ChatID == chatID {
			return chat.Messages
		}
	}
	return nil
}
