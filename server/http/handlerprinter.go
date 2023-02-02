package httpserver

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// A printer for printing handler requests and responses, separate from application logging
type HandlerPrinter struct {
	writer io.Writer
}

func NewHandlerPrinter() HandlerPrinter {
	return NewHandlerPrinterWithWriter(os.Stdout)
}

func NewHandlerPrinterWithWriter(writer io.Writer) HandlerPrinter {
	return HandlerPrinter{writer}
}

// Prints the method and path
func (res HandlerPrinter) PrintPath(req *http.Request) {
	fmt.Fprintln(res.writer, getPath(req))
}

// Prints the method, path, and status code
func (res HandlerPrinter) PrintStatus(req *http.Request, statusCode int) {
	res.PrintStatusMessage(req, statusCode, http.StatusText(statusCode))
}

// Prints the method, path, status code, and message
func (res HandlerPrinter) PrintStatusMessage(req *http.Request, statusCode int, message string) {
	if statusCode >= 100 && statusCode < 400 {
		fmt.Fprintf(res.writer, "%s\t%d %s\n", getPath(req), statusCode, message)
	} else {
		fmt.Fprintf(res.writer, "%s\tError (%d): %s\n", getPath(req), statusCode, message)
	}
}

func getPath(req *http.Request) string {
	return fmt.Sprintf("%s\t%s\t%s", getTime(), req.Method, req.URL.Path)
}

func getTime() string {
	return time.Now().UTC().Local().Format(time.RFC3339)
}
