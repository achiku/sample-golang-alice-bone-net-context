// https://github.com/alexedwards/stack/blob/master/stack.go
package main

import (
	"net/http"

	"golang.org/x/net/context"
)

type chainHandler func(context.Context) http.Handler
type chainMiddleware func(context.Context, http.Handler) http.Handler

type Chain struct {
	mws []chainMiddleware
	h   chainHandler
}

func TackNew(mws ...chainMiddleware) Chain {
	return Chain{mws: mws}
}

func (c Chain) Append(mws ...chainMiddleware) Chain {
	newMws := make([]chainMiddleware, len(c.mws)+len(mws))
	copy(newMws[:len(c.mws)], c.mws)
	copy(newMws[len(c.mws):], mws)
	c.mws = newMws
	return c
}

func (c Chain) Then(chf func(ctx context.Context, w http.ResponseWriter, r *http.Request)) HandlerChain {
	c.h = adaptContextHandlerFunc(chf)
	return newHandlerChain(c)
}

func (c Chain) ThenHandler(h http.Handler) HandlerChain {
	c.h = adaptHandler(h)
	return newHandlerChain(c)
}

func (c Chain) ThenHandlerFunc(fn func(http.ResponseWriter, *http.Request)) HandlerChain {
	c.h = adaptHandlerFunc(fn)
	return newHandlerChain(c)
}

type HandlerChain struct {
	context context.Context
	Chain
}

func NewContext() context.Context {
	return context.Background()
}

func newHandlerChain(c Chain) HandlerChain {
	return HandlerChain{context: NewContext(), Chain: c}
}

func (hc HandlerChain) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	final := hc.h(hc.context)
	for i := len(hc.mws) - 1; i >= 0; i-- {
		final = hc.mws[i](hc.context, final)
	}
	final.ServeHTTP(w, r)
}

// Adapt third party middleware with the signature
// func(http.Handler) http.Handler into chainMiddleware
func Adapt(fn func(http.Handler) http.Handler) chainMiddleware {
	return func(ctx context.Context, h http.Handler) http.Handler {
		return fn(h)
	}
}

// Adapt http.Handler into a chainHandler
func adaptHandler(h http.Handler) chainHandler {
	return func(ctx context.Context) http.Handler {
		return h
	}
}

// Adapt a function with the signature
// func(http.ResponseWriter, *http.Request) into a chainHandler
func adaptHandlerFunc(fn func(w http.ResponseWriter, r *http.Request)) chainHandler {
	return adaptHandler(http.HandlerFunc(fn))
}

// Adapt a function with the signature
// func(Context, http.ResponseWriter, *http.Request) into a chainHandler
func adaptContextHandlerFunc(fn func(ctx context.Context, w http.ResponseWriter, r *http.Request)) chainHandler {
	return func(ctx context.Context) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fn(ctx, w, r)
		})
	}
}
