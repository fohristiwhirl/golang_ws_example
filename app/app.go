package app

import (
	"net/http"
)

func Run() {
	go hub_runner()
	http.HandleFunc("/", handler)
	http.ListenAndServe("127.0.0.1:8080", nil)
}

func hub_runner() {
	var hub Hub
	hub.Spin()
}
