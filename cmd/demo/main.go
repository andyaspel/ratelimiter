package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	ratelimiter "github.com/andyaspel/ratelimiter"
)

func exampleHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "OK")
}

func main() {
	// 10 token burst, 5 tokens per second
	rl, err := ratelimiter.NewTokenBucketRateLimiter(10, 5)
	if err != nil {
		log.Fatalf("rate limiter config error: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", ratelimiter.HTTPMiddleware(rl, nil)(http.HandlerFunc(exampleHandler)))

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		// simple demo of Wait
		ctx := context.Background()
		for i := 0; i < 5; i++ {
			if err := rl.Wait(ctx); err != nil {
				log.Printf("wait error: %v", err)
				return
			}
			log.Printf("request %d allowed by Wait()", i+1)
		}
	}()

	log.Println("listening on :8080")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
