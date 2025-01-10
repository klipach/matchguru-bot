package contract

type BotRequest struct {
	Message  string `json:"message"`
	ChatID   int    `json:"chat_id"`
	GameID   int    `json:"game_id"`
	Timezone string `json:"timezone"`
}

type BotResponse struct {
	Response string `json:"response"`
}
