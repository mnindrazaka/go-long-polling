package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Message struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	Body      string    `json:"body"`
}

type ChatService struct {
	mutex    sync.RWMutex
	messages map[uuid.UUID]Message
}

func (service *ChatService) CreateMessage(body string) (Message, error) {
	message := Message{
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		Body:      body,
	}

	service.mutex.Lock()
	defer service.mutex.Unlock()

	service.messages[message.ID] = message

	return message, nil
}

func (service *ChatService) FindMessages(after *uuid.UUID) ([]Message, error) {
	if after == nil {
		after = &uuid.UUID{}
	}

	service.mutex.RLock()
	defer service.mutex.RUnlock()

	messages := make([]Message, len(service.messages))
	for _, message := range service.messages {
		messages = append(messages, message)
	}

	return messages, nil
}

type Server struct {
	chatService *ChatService
}

type CreateMessage struct {
	Body string `json:"body"`
}

func (server *Server) CreateMessageHandler(w http.ResponseWriter, req *http.Request) {
	var input CreateMessage

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		httpError(w, err)
		return
	}

	if err := req.Body.Close(); err != nil {
		httpError(w, err)
		return
	}

	if err := json.Unmarshal(body, &input); err != nil {
		httpError(w, err)
		return
	}

	message, err := server.chatService.CreateMessage(input.Body)
	if err != nil {
		httpError(w, err)
		return
	}

	httpOk(w, message)
}

func (server *Server) FindMessagesHandler(w http.ResponseWriter, req *http.Request) {
	after := req.URL.Query().Get("after")
	if after == "" {
		after = "00000000-0000-0000-0000-000000000000"
	}

	afterUUID, err := uuid.Parse(after)
	if err != nil {
		httpError(w, err)
		return
	}

	for i := 0; i < 10; i++ {
		messages, err := server.chatService.FindMessages(&afterUUID)
		if err != nil {
			httpError(w, err)
			return
		}

		if len(messages) > 0 {
			httpOk(w, messages)
			return
		}

		time.Sleep(time.Second)
	}

	messages := []Message{}
	httpOk(w, messages)
}

func httpError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(err)
}

func httpOk(w http.ResponseWriter, resp interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func main() {
	r := chi.NewRouter()
	chatService := ChatService{
		mutex:    sync.RWMutex{},
		messages: map[uuid.UUID]Message{},
	}

	server := Server{
		chatService: &chatService,
	}

	r.Get("/messages", server.FindMessagesHandler)
	r.Post("/messages", server.CreateMessageHandler)

	http.ListenAndServe(":8080", r)
}
