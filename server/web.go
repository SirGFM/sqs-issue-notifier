package main

import (
	"encoding/json"
	"fmt"
	"github.com/SirGFM/sqs-issue-notifier/server/local_storage"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// endpoint allows associating a given (resource, method) to its handler in
// a map.
type endpoint struct {
	// The resource (i.e., the first field in the URL).
	resource string

	// The HTTP method associated with the method.
	method string
}

// endpointHandler defines a function used to handle HTTP requests, much
// like http.HandlerFunc. Differently from that, this type also accepts
// the decoded and split path as res.
type endpointHandler func(w http.ResponseWriter, req *http.Request, res []string)

// server wraps the httpServer and all of its components, so it may be
// gracefully stopped.
type server struct {
	// The server's HTTP server.
	httpServer *http.Server

	// Handlers for each endpoint.
	handlers map[endpoint]endpointHandler

	// The local storage where messages are stored.
	store local_storage.Store
}

// Close the running web server and clean up resourcers
func (s *server) Close() error {
	if s.httpServer != nil {
		s.httpServer.Close()
		s.httpServer = nil
	}

	return nil
}

// ServeHTTP is called by Go's http package whenever a new HTTP request arrives
func (s *server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	uri := cleanURL(req.URL)
	log.Printf("%s - %s - %s", req.RemoteAddr, req.Method, uri)

	// TODO: Authentication

	res := strings.Split(uri, "/") 
	if len(res) == 0 {
		httpTextReply(http.StatusNotFound, "No resource was specified", w)
		log.Printf("[%s] %s - %s: 404", req.Method, uri, req.RemoteAddr)
		return
	}

	f, ok := s.handlers[endpoint{res[0], req.Method}]
	if !ok || f == nil {
		httpTextReply(http.StatusNotFound, "Invalid resource", w)
		log.Printf("[%s] %s - %s: 404", req.Method, uri, req.RemoteAddr)
		return
	}

	f(w, req, res)
}

// GetMessage handles GET requests on the 'message' resource, returning the
// number of messages currently stored in the server.
func (s *server) GetMessage(w http.ResponseWriter, req *http.Request, res []string) {
	num := s.store.Count()

	if len(res) > 1 {
		httpTextReply(http.StatusNotFound, "Invalid resource", w)
		log.Printf("[%s] %s - %s: 404", req.Method, strings.Join(res, "/"), req.RemoteAddr)
		return
	}

	switch req.Header.Get("Accept") {
	case "application/json":
		resp := struct{MessageCount int}{num}
		data, err := json.Marshal(&resp)
		if err != nil {
			serr := "Failed to encode the response"
			httpTextReply(http.StatusInternalServerError, serr, w)
			log.Printf("[%s] %s - %s: %s (%+v)", req.Method, res[0], req.RemoteAddr, serr, err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeData(data, w)
	default:
		// By default, force "text/plain"
		fallthrough
	case "text/plain":
		msg := fmt.Sprintf("Local message count: %d", num)
		httpTextReply(http.StatusOK, msg, w)
	}
}

// PostMessage handles POST requests on the 'message' resource, accepting a
// single message and forwarding it to the local storage.
func (s *server) PostMessage(w http.ResponseWriter, req *http.Request, res []string) {
	if len(res) > 1 {
		log.Printf("[%s] %s - %s: 404", req.Method, strings.Join(res, "/"), req.RemoteAddr)
		httpTextReply(http.StatusNotFound, "Invalid resource", w)
		return
	}

	var msg struct{
		Channel string
		Message string
	}
	dec := json.NewDecoder(req.Body)
	err := dec.Decode(&msg)
	if err != nil {
		log.Printf("[%s] %s - %s: Failed to parse request: %+v", req.Method, res[0], req.RemoteAddr, err)
		httpTextReply(http.StatusBadRequest, "Invalid data", w)
		return
	}

	// Re-encode the message, to possibly add more fields.
	data, err := json.Marshal(&msg)
	if err != nil {
		serr := "Failed to encode the message"
		httpTextReply(http.StatusInternalServerError, serr, w)
		log.Printf("[%s] %s - %s: %s (%+v)", req.Method, res[0], req.RemoteAddr, serr, err)
		return
	}

	err = s.store.Store(data)
	if err != nil {
		serr := "Failed to store the message"
		httpTextReply(http.StatusInternalServerError, serr, w)
		log.Printf("[%s] %s - %s: %s (%+v)", req.Method, res[0], req.RemoteAddr, serr, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// cleanURL so everything is properly escaped/encoded and so it may be split into each of its components.
//
// Use `url.Unescape` to retrieve the unescaped path, if so desired.
func cleanURL(uri *url.URL) string {
	// Normalize and strip the URL from its leading prefix (and slash)
	resUrl := path.Clean(uri.EscapedPath())
	if len(resUrl) > 0 && resUrl[0] == '/' {
		resUrl = resUrl[1:]
	} else if len(resUrl) == 1 && resUrl[0] == '.' {
		// Clean converts an empty path into a single "."
		resUrl = ""
	}

	return resUrl
}

// httpTextReply send a simple HTTP response as a plain text.
func httpTextReply(status int, msg string, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(status)

	writeData([]byte(msg), w)
}

// writeData, account for incomplete writes.
func writeData(data []byte, w io.Writer) {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			log.Printf("Failed to send: %+v", err)
			return
		}
		data = data[n:]
	}
}

// RunWeb starts the web server and return an io.Closer, so the server may
// be stopped.
func RunWeb(args Args, store local_storage.Store) io.Closer {
	var srv server

	srv.httpServer = &http.Server {
		Addr: fmt.Sprintf("%s:%d", args.IP, args.Port),
		Handler: &srv,
	}
	srv.handlers = map[endpoint]endpointHandler {
		endpoint{"message", http.MethodGet}: srv.GetMessage,
		endpoint{"message", http.MethodPost}: srv.PostMessage,
	}

	srv.store = store

	go func() {
		log.Printf("Waiting...")
		srv.httpServer.ListenAndServe()
	} ()

	return &srv
}
