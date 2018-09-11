package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/icstglobal/plasma/cmd"
)

func main() {
	cmd.Execute()

	waitForExit()
}

func waitForExit() {
	sch := make(chan os.Signal, 1)
	signal.Notify(sch, os.Interrupt)
	signal.Notify(sch, os.Kill)
	//wait
	<-sch

	cleanup()

	//exit
	close(sch)
	fmt.Println("bye")
}

func cleanup() {
	//do anything before the process terminates
}
