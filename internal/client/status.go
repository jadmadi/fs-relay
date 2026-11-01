package client

import "log"

func logConnected(url, id string) {
	log.Printf("fs-relay-bind: connected to %s, client %s", url, id)
}

func logReconnected(url, id string) {
	log.Printf("fs-relay-bind: reconnected to %s, client %s", url, id)
}

func logLost(url string) {
	log.Printf("fs-relay-bind: connection to %s lost", url)
}
