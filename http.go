package matchguru

type MessageRequest struct {
	Message string `json:"message"`
	UserId  string `json:"user_id"`
	ChatId  int    `json:"chat_id"`
	GameId  int    `json:"game_id"`
}

type MessageResponse struct {
	Response string `json:"response"`
}
