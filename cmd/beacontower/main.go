// Command beacontower runs the standalone zero-trust encrypted relay server
// for Hyla BeaconTower remote connections.
//
// It multiplexes native binary TCP/TLS ("BT01") and HTTP/WebSocket (/ws,
// /healthz) on a single port, authenticates hosts and clients without seeing
// plaintext tokens, and forwards end-to-end encrypted AEAD frames untouched.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"hyla/internal/beacontower"
)

var version = "dev"

func main() {
	defaultListen := os.Getenv("BEACONTOWER_LISTEN")
	if defaultListen == "" {
		defaultListen = ":" + beacontower.DefaultPort
	}
	defaultToken := os.Getenv("BEACONTOWER_TOKEN")

	listenAddr := flag.String("listen", defaultListen, "address to listen on (env BEACONTOWER_LISTEN)")
	token := flag.String("token", defaultToken, "optional shared relay access token (env BEACONTOWER_TOKEN)")
	tlsCert := flag.String("tls-cert", "", "path to TLS certificate PEM file")
	tlsKey := flag.String("tls-key", "", "path to TLS private key PEM file")
	wsPath := flag.String("ws-path", beacontower.DefaultWSPath, "WebSocket upgrade path")
	maxHosts := flag.Int("max-hosts", 1024, "maximum simultaneous registered hosts")
	maxSessions := flag.Int("max-sessions", 4096, "maximum simultaneous bridged sessions")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("beacontower %s\n", version)
		return
	}
	if (*tlsCert != "") != (*tlsKey != "") {
		fmt.Fprintln(os.Stderr, "beacontower: both -tls-cert and -tls-key must be specified together")
		os.Exit(2)
	}

	logger := log.New(os.Stderr, "beacontower: ", log.LstdFlags)
	srv := beacontower.NewServer(beacontower.ServerConfig{
		ListenAddr:  *listenAddr,
		Token:       *token,
		TLSCert:     *tlsCert,
		TLSKey:      *tlsKey,
		WSPath:      *wsPath,
		MaxHosts:    *maxHosts,
		MaxSessions: *maxSessions,
		Logger:      logger,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Printf("listening on %s (ws=%s tls=%v auth=%v)", *listenAddr, *wsPath, *tlsCert != "", *token != "")
	if err := srv.ListenAndServe(ctx); err != nil {
		logger.Fatalf("server error: %v", err)
	}
	logger.Println("stopped")
}
