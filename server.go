// https://joeshaw.org/net-context-and-http-handler/
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-zoo/bone"
	"golang.org/x/net/context"
)

type key int

const requestIDKey key = 0

func contextMiddleware(ctx context.Context, next http.Handler) http.Handler {
	fn := func(rw http.ResponseWriter, req *http.Request) {
		context.WithValue(ctx, requestIDKey, req.Header.Get("X-Request-ID"))
		log.Printf("[%s]\n", ctx.Value(requestIDKey))
		next.ServeHTTP(rw, req)
	}
	return http.HandlerFunc(fn)
}

func loggingMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		t1 := time.Now()
		next.ServeHTTP(w, r)
		t2 := time.Now()
		log.Printf("[%s] %q %v\n", r.Method, r.URL.String(), t2.Sub(t1))
	}
	return http.HandlerFunc(fn)
}

func recoverMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Println("panic: %+v", err)
				http.Error(w, http.StatusText(500), 500)
			}
		}()
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func account(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
	accountId := bone.GetValue(req, "id")
	reqId := ctx.Value(requestIDKey).(string)
	fmt.Fprintf(rw, "accountId: %s, Request-ID: %s", accountId, reqId)
}

func note(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
	noteId := bone.GetValue(req, "id")
	reqId := ctx.Value(requestIDKey).(string)
	fmt.Fprintf(rw, "noteId: %s, Request-ID: %s", noteId, reqId)
}

func simple(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "normal handler\n")
}

func main() {

	commonHandlers := TackNew(
		contextMiddleware,
		Adapt(loggingMiddleware),
		Adapt(recoverMiddleware),
	)

	accountContextHandler := account
	noteContextHandler := note
	simpleHandler := http.HandlerFunc(simple)

	mux := bone.New()
	mux.Get("/account/:id", commonHandlers.Then(accountContextHandler))
	mux.Get("/note/:id", commonHandlers.Then((noteContextHandler)))
	mux.Get("/simple", commonHandlers.ThenHandlerFunc(simpleHandler))
	http.ListenAndServe(":8080", mux)
}
