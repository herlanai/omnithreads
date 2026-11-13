package main

import (
	app "omnithreads"
	"omnithreads/internal/community"
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"github.com/herlanai/omnigo"
	"github.com/herlanai/omnigo/omniweb"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	browser := flag.Bool("serve", false, "serve browser preview on 127.0.0.1:7331")
	dir := flag.String("data", "", "community data directory")
	flag.Parse()
	instanceID := "ai.omnithreads.desktop"
	if *dir != "" {
		absolute, err := filepath.Abs(*dir)
		if err != nil {
			log.Fatal(err)
		}
		instanceID = fmt.Sprintf("ai.omnithreads.desktop.%x", sha256.Sum256([]byte(absolute)))
	}
	if *dir == "" {
		base, e := os.UserConfigDir()
		if e != nil {
			log.Fatal(e)
		}
		*dir = filepath.Join(base, "OmniThreads")
	}
	s, e := community.Open(*dir)
	if e != nil {
		log.Fatal(e)
	}
	defer s.Close()
	if *browser {
		serve(s)
		return
	}
	var desktop *omnigo.Application
	desktop, e = omnigo.New(omnigo.Options{Name: "OmniThreads", Assets: app.Assets(), Icon: app.Icon, SingleInstance: &omnigo.SingleInstanceOptions{ID: instanceID}, Window: &omnigo.WindowOptions{Title: "OmniThreads", Width: 1360, Height: 900, MinWidth: 900, MinHeight: 640, TitleBarStyle: omnigo.TitleBarHidden}, Services: []omnigo.Service{omnigo.Background("community.runner", s.Work)}, Registrations: []omnigo.Registration{omnigo.Route("/media/", http.HandlerFunc(s.ServeImage)), omnigo.Query("community.state", s.Snapshot), omnigo.Method("community.action", s.Dispatch)}})
	if e != nil {
		log.Fatal(e)
	}
	if e = desktop.RunMain(); e != nil {
		log.Print(e)
	}
}
func serve(s *community.Store) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	r := omniweb.NewRouter(32, 1<<20)
	if e := omniweb.Bind(r, "community.state", "", func(context.Context, struct{}) (community.State, error) { return s.Snapshot(), nil }); e != nil {
		log.Fatal(e)
	}
	if e := omniweb.Bind(r, "community.action", "", s.Dispatch); e != nil {
		log.Fatal(e)
	}
	h, e := omniweb.NewHTTP(r, omniweb.HTTPOptions{Host: "127.0.0.1:7331", Origins: []string{"http://127.0.0.1:7331", "http://127.0.0.1:5173", "http://localhost:5173"}})
	if e != nil {
		log.Fatal(e)
	}
	mux := http.NewServeMux()
	mux.Handle("/omniweb/", h)
	mux.HandleFunc("/media/", s.ServeImage)
	mux.Handle("/", http.FileServerFS(app.Assets()))
	srv := &http.Server{Addr: "127.0.0.1:7331", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	done := make(chan struct{})
	go func() { defer close(done); _ = s.Work(ctx) }()
	go func() {
		<-ctx.Done()
		c, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = srv.Shutdown(c)
	}()
	log.Print("OmniThreads: http://127.0.0.1:7331")
	if e = srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
	<-done
}
