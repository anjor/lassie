package httpserver

import (
	"net/http"
	"os"
)

func pingHandler(res http.ResponseWriter, req *http.Request) {
	printer := NewHandlerPrinterWithWriter(os.Stdout)
	printer.PrintPath(req)

	res.Header().Set("Content-Type", "text/plain; charset=utf-8")
	res.Write([]byte("pong"))

	printer.PrintStatus(req, http.StatusOK)
}
