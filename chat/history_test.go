package chat

import (
	"reflect"
	"testing"
)

func TestFindChatMessages(t *testing.T) {
	tests := []struct {
		name     string
		chats    []*firestoreChat
		chatID   int
		expected []*firestoreMessage
	}{
		{
			name: "Chat found",
			chats: []*firestoreChat{
				{ChatID: 1, Messages: []*firestoreMessage{{}, {}}},
				{ChatID: 2, Messages: []*firestoreMessage{{}}},
			},
			chatID:   1,
			expected: []*firestoreMessage{{}, {}},
		},
		{
			name: "Chat not found",
			chats: []*firestoreChat{
				{ChatID: 1, Messages: []*firestoreMessage{{}, {}}},
				{ChatID: 2, Messages: []*firestoreMessage{{}}},
			},
			chatID:   3,
			expected: nil,
		},
		{
			name:     "Empty chats",
			chats:    []*firestoreChat{},
			chatID:   1,
			expected: nil,
		},
		{
			name:     "nil chats",
			chats:    nil,
			chatID:   1,
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := findChatMessages(test.chats, test.chatID)
			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("findChatMessages(%v, %d) = %v; want %v", test.chats, test.chatID, result, test.expected)
			}
		})
	}
}
