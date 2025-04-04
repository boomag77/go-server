package contracts

import "net/http"

type Bot interface {
	SendMessage(chatID int64, text string) error
	WebHookHandler(w http.ResponseWriter, r *http.Request)
}
