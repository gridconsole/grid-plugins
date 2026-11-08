// A tiny dev server used by lifecycle tests, built as a separate Go process.
package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func main() {
	mode := "healthy"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	// Hold a lifetime witness in each process, including the tree's parent.
	// Readers see EOF only after all processes exit, even if zombies remain.
	lifetime, err := os.OpenFile("server.lifetime", os.O_WRONLY, 0)
	if err != nil {
		panic(err)
	}
	defer lifetime.Close()
	if _, err := fmt.Fprintln(lifetime, "started"); err != nil {
		panic(err)
	}
	if mode == "tree" {
		signal.Ignore(os.Interrupt, syscall.SIGTERM)
		executable, err := os.Executable()
		if err != nil {
			os.Exit(1)
		}
		child := exec.Command(executable, "healthy")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Run(); err != nil {
			os.Exit(1)
		}
		return
	}
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		for s := range signals {
			if mode == "term" && s == syscall.SIGTERM {
				os.Exit(0)
			}
			if mode != "stubborn" && mode != "term" {
				os.Exit(0)
			}
		}
	}()
	listener, err := net.Listen("tcp4", "127.0.0.1:"+os.Getenv("PORT"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile("server.pid", []byte(fmt.Sprint(os.Getpid())), 0600); err != nil {
		os.Exit(1)
	}
	http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if mode == "unhealthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if mode == "redirect" {
			http.Redirect(w, r, "/other", http.StatusFound)
			return
		}
		fmt.Fprintln(w, "ok")
	}))
}
