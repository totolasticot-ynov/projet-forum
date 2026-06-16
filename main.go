package main

import (
    "log"
    "net/http"

    "forum/internal/web"
)

func main() {
    log.Fatal(http.ListenAndServe(":8080", web.New()))
}
