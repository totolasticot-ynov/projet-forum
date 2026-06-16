package main

import (
	"log"
	"net/http"

	"forum/internal/web"
)

func main() {
	log.Println("Serveur démarré sur http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", web.New()))
}
