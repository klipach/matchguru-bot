package matchguru

type MessageRequest struct {
	Message string `json:"message"`
	UserId  string `json:"user_id"`
}

type MessageResponse struct {
	Response string `json:"response"`
}
