// Command go-ratelimit-example runs an HTTP server demonstrating
// github.com/Lapius7/go-ratelimit: a permissive token bucket endpoint, a
// strict fixed window endpoint, and a Router dispatching between them.
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Lapius7/go-ratelimit"
)

func main() {
	hello := ratelimit.New(ratelimit.TokenBucket, ratelimit.Config{
		Rate: 5,
		Per:  time.Minute,
	})
	defer hello.Close()

	strict := ratelimit.New(ratelimit.FixedWindow, ratelimit.Config{
		Rate: 2,
		Per:  time.Minute,
	})
	defer strict.Close()

	rt := ratelimit.NewRouter()
	rt.Handle("/hello", hello)
	rt.Handle("/strict", strict)

	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello from the token bucket endpoint (5 req/min)")
	})
	mux.HandleFunc("/strict", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello from the fixed window endpoint (2 req/min)")
	})

	addr := ":18181"
	log.Printf("listening on %s", addr)
	log.Println("try:")
	log.Printf("  curl -i http://localhost%s/hello   # allows 5/min", addr)
	log.Printf("  curl -i http://localhost%s/strict  # allows 2/min", addr)
	log.Fatal(http.ListenAndServe(addr, rt.Middleware(mux)))
}
