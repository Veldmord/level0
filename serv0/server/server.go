package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"Level0/cache"

	"github.com/gorilla/mux"
)

type Server struct {
	cache  *cache.Cache
	router *mux.Router
}

func NewServer(cache *cache.Cache) *Server {
	router := mux.NewRouter()
	server := &Server{
		cache:  cache,
		router: router,
	}

	router.HandleFunc("/orders/{id}", server.GetOrderHandler).Methods("GET")

	return server
}

func (s *Server) Start(port int) error {
	log.Printf("Server started on port :%d\n", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), s.router)
}

func (s *Server) GetOrderHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	order, err := s.cache.GetOrder(orderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(order)
}

func (s *Server) Stop(ctx context.Context) error {
	return nil // Optional: graceful shutdown logic
}
