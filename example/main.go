// Command go-rataliy_lib-example runs an HTTP server demonstrating
// github.com/Lapius7/go-rataliy_lib: a permissive token bucket endpoint, a
// strict fixed window endpoint, a Router dispatching between them, and a
// live dashboard of both limiters' current state on a separate port.
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Lapius7/go-rataliy_lib"
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

	dashboard := ratelimit.NewDashboard(map[string]*ratelimit.Limiter{
		"hello":  hello,
		"strict": strict,
	})
	dashboardAddr := ":18182"
	go func() {
		log.Printf("dashboard listening on %s", dashboardAddr)
		log.Fatal(dashboard.ListenAndServe(dashboardAddr))
	}()

	addr := ":18181"
	log.Printf("listening on %s", addr)
	log.Println("try:")
	log.Printf("  curl -i http://localhost%s/hello   # allows 5/min", addr)
	log.Printf("  curl -i http://localhost%s/strict  # allows 2/min", addr)
	log.Printf("  open http://localhost%s/           # live dashboard", dashboardAddr)
	log.Fatal(http.ListenAndServe(addr, rt.Middleware(mux)))
}
