package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jzhang046/croned-twitcasting-recorder/service"
)

const CronedRecordCmdName = "croned"

func RecordCroned() {
	log.Printf("Starting in recording mode [%s] with PID [%d]..\n", CronedRecordCmdName, os.Getpid())

	manager := service.NewManager()
	if err := manager.Start(); err != nil {
		log.Fatalln("Failed starting recorder:", err)
	}

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	<-signalCtx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), terminationGraceDuration)
	defer cancel()
	if err := manager.Stop(shutdownCtx); err != nil {
		log.Printf("Recorder shutdown did not finish cleanly: %v\n", err)
	}
	log.Fatal("Terminated on user interrupt")
}
