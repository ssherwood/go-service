package main

import (
	"github.com/gorilla/mux"
	"locationservice/database"
	"locationservice/handlers"
	"log"
	"net/http"
)

func main() {
	dbPool, err := database.ConnectDB()
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	defer dbPool.Close()

	r := mux.NewRouter()
	handlers.RegisterLocationHandlers(r, dbPool)

	log.Println("Server running on port 8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Could not start server: %v", err)
	}
}
