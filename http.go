package matchguru

type MessageRequest struct {
	Message string `json:"message"`
	ChatID  int    `json:"chat_id"`
	GameId  int    `json:"game_id"`
}

type MessageResponse struct {
	Response string `json:"response"`
}
