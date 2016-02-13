package main

import (
	"flag"
	"github.com/mrd0ll4r/rock-paper-scissors-bot"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var token = flag.String("token", "<your-token-here>", "Telegram bot API token")

func main() {
	flag.Parse()
	closing := make(chan struct{})

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		bot.RunBot(*token, closing)
	}()

	// ensure a clean shutdown
	shutdown := make(chan os.Signal)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-shutdown
		signal.Stop(shutdown)
		close(shutdown)
		close(closing)
	}()

	// wait for the signal
	wg.Wait()

}
