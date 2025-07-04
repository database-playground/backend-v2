package gauth

import (
	"net/http"

	"github.com/gorilla/handlers"
)

type CallbackHandler struct {
	stateStorage StateStorage
}

func NewCallbackHandler(stateStorage StateStorage) http.Handler {
	return handlers.ProxyHeaders(&CallbackHandler{stateStorage: stateStorage})
}

func (h *CallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement OAuth2 callback handling
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

var _ http.Handler = (*CallbackHandler)(nil)
