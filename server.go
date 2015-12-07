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

func newContextWithRequestID(ctx context.Context, req *http.Request) context.Context {
	return context.WithValue(ctx, requestIDKey, req.Header.Get("X-Request-ID"))
}

func requestIDFromContext(ctx context.Context) string {
	return ctx.Value(requestIDKey).(string)
}

type ContextHandler interface {
	ServeHTTPContext(context.Context, http.ResponseWriter, *http.Request)
}

type ContextHandlerFunc func(context.Context, http.ResponseWriter, *http.Request)

func (h ContextHandlerFunc) ServeHTTPContext(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
	h(ctx, rw, req)
}

type ContextAdapter struct {
	ctx     context.Context
	handler ContextHandler
}

func (ca *ContextAdapter) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ca.handler.ServeHTTPContext(ca.ctx, rw, req)
}

func middleware(h ContextHandler) ContextHandler {
	return ContextHandlerFunc(func(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
		ctx = newContextWithRequestID(ctx, req)
		h.ServeHTTPContext(ctx, rw, req)
	})
}

func contextMiddleware(ctx context.Context, next http.Handler) http.Handler {
	fn := func(rw http.ResponseWriter, req *http.Request) {
		ctx = newContextWithRequestID(ctx, req)
		next.ServeHTTP(rw, req)
		log.Printf("[%s]\n", ctx)
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
	reqId := requestIDFromContext(ctx)
	fmt.Fprintf(rw, "accountId: %s, Request-ID: %s", accountId, reqId)
}

func note(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
	noteId := bone.GetValue(req, "id")
	reqId := requestIDFromContext(ctx)
	fmt.Fprintf(rw, "noteId: %s, Request-ID: %s", noteId, reqId)
}

func simple(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "normal handler\n")
}

func main() {
	mux := bone.New()
	ctx := context.Background()
	accountContextHandler := &ContextAdapter{
		ctx:     ctx,
		handler: middleware(ContextHandlerFunc(account)),
	}
	noteContextHandler := &ContextAdapter{
		ctx:     ctx,
		handler: middleware(ContextHandlerFunc(note)),
	}
	simpleHandler := http.HandlerFunc(simple)

	commonHandlers := TackNew(
		contextMiddleware,
		Adapt(loggingMiddleware),
		Adapt(recoverMiddleware),
	)

	mux.Get("/account/:id", commonHandlers.ThenHandlerFunc(accountContextHandler))
	mux.Get("/note/:id", commonHandlers.ThenHandlerFunc((noteContextHandler)))
	mux.Get("/simple", commonHandlers.ThenHandlerFunc(simpleHandler))
	http.ListenAndServe(":8080", mux)
}
