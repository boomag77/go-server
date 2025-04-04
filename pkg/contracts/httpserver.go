package contracts

import "net/http"

type HttpServer interface {
	Service
	SetHandler(string, http.HandlerFunc)
}
