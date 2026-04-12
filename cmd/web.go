package cmd

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jzhang046/croned-twitcasting-recorder/admin"
	"github.com/jzhang046/croned-twitcasting-recorder/service"
)

const WebAdminCmdName = "web"

func RecordWeb(args []string) {
	webCmd := flag.NewFlagSet(WebAdminCmdName, flag.ExitOnError)
	addr := webCmd.String("addr", envOrDefault("TWITCAST_WEB_ADDR", "127.0.0.1:8080"), "web console listen address")
	username := webCmd.String("username", envOrDefault("TWITCAST_WEB_USERNAME", ""), "basic auth username")
	password := webCmd.String("password", envOrDefault("TWITCAST_WEB_PASSWORD", ""), "basic auth password")
	autoStart := webCmd.Bool("auto-start", envBool("TWITCAST_WEB_AUTO_START"), "start the recorder automatically when the web console boots")
	allowPublicNoAuth := webCmd.Bool("allow-public-no-auth", envBool("TWITCAST_WEB_ALLOW_PUBLIC_NO_AUTH"), "allow listening on a non-loopback address without built-in basic auth")
	webCmd.Parse(args)

	if (*username == "") != (*password == "") {
		log.Fatalln("Both --username and --password must be set together")
	}
	if isPublicListen(*addr) && *username == "" && !*allowPublicNoAuth {
		log.Fatalln("Refusing to bind a public web address without authentication. Set --username/--password or pass --allow-public-no-auth if a reverse proxy already protects it.")
	}

	rootDir, err := os.Getwd()
	if err != nil {
		log.Fatalln("Failed resolving working directory:", err)
	}

	manager := service.NewManager()
	if *autoStart {
		if err := manager.Start(); err != nil {
			log.Printf("Recorder auto-start failed: %v\n", err)
		}
	}

	restartRequested := make(chan struct{}, 1)
	server := admin.NewServer(admin.Options{
		Address:  *addr,
		RootDir:  rootDir,
		Username: *username,
		Password: *password,
	}, manager, restartRequested)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Web console listening on http://%s\n", *addr)
		errCh <- server.ListenAndServe()
	}()

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	restartAfterShutdown := false
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalln("Web console stopped unexpectedly:", err)
		}
	case <-signalCtx.Done():
	case <-restartRequested:
		restartAfterShutdown = true
		log.Println("Bot restart requested from web console")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 整個 bot 重啟要由主循環統一收斂：先停 listener、再停 recorder，最後才重啟進程。
	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("Web console shutdown returned an error: %v\n", err)
	}
	if err := manager.Stop(shutdownCtx); err != nil {
		log.Printf("Recorder shutdown returned an error: %v\n", err)
	}

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Web console exited with error: %v\n", err)
		}
	default:
	}

	if restartAfterShutdown {
		executable, err := os.Executable()
		if err != nil {
			log.Fatalln("Failed resolving executable for restart:", err)
		}
		if err := restartProcess(executable, os.Args[1:]); err != nil {
			log.Fatalln("Failed restarting bot process:", err)
		}
	}
}

func envOrDefault(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(name string) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func isPublicListen(addr string) bool {
	host := strings.TrimSpace(addr)
	if parsedHost, _, err := net.SplitHostPort(addr); err == nil {
		host = parsedHost
	}

	switch host {
	case "", "0.0.0.0", "::":
		return true
	case "127.0.0.1", "localhost", "::1":
		return false
	default:
		return true
	}
}
