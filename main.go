package main

import (
	"log"
	"os"

	"github.com/jzhang046/croned-twitcasting-recorder/applog"
	"github.com/jzhang046/croned-twitcasting-recorder/cmd"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	if err := applog.ConfigureFromEnv(); err != nil {
		log.Printf("Failed to configure log output from environment: %v\n", err)
	}
}

var availableCmds = []string{cmd.CronedRecordCmdName, cmd.DirectRecordCmdName, cmd.WebAdminCmdName}

func main() {
	if len(os.Args) < 2 {
		log.Println("Record mode not specified; supported modes:", availableCmds)
		cmd.RecordCroned()
	} else {
		switch os.Args[1] {
		case cmd.CronedRecordCmdName:
			cmd.RecordCroned()
		case cmd.DirectRecordCmdName:
			cmd.RecordDirect(os.Args[2:])
		case cmd.WebAdminCmdName:
			cmd.RecordWeb(os.Args[2:])
		default:
			log.Fatalf(
				"Unknown record mode [%s]; supported modes: %s",
				os.Args[1],
				availableCmds,
			)
		}
	}
}
