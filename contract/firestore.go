package contract

type FirestoreUser struct {
	Chats       []FirestoreChat `firestore:"chats"`
	DisplayName string          `firestore:"display_name"`
}

type FirestoreChat struct {
	Messages []FirestoreMessage `firestore:"messages"`
}

type FirestoreMessage struct {
	From    string `firestore:"from"`
	Message string `firestore:"message"`
}
