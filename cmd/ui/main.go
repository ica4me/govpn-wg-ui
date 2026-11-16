package main

import (
	"context"
	"govpn-wg-ui/internal/console"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func env(k, d string) string {
	if s := os.Getenv(k); s != "" {
		return s
	}
	return d
}
func main() {
	app, e := console.New(env("GOVPN_API_URL", "http://127.0.0.1:8080/api/v1"), env("GOVPN_COOKIE_SECURE", "false") == "true")
	if e != nil {
		log.Fatal(e)
	}
	if e = app.SetTrustedProxies(os.Getenv("GOVPN_TRUSTED_PROXY_CIDRS")); e != nil {
		log.Fatal(e)
	}
	srv := &http.Server{Addr: env("GOVPN_UI_LISTEN", "127.0.0.1:8090"), Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(c)
	}()
	log.Printf("GoVPN UI listening on %s", srv.Addr)
	if e = srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
}
