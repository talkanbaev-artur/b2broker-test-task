package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/talkanbaev-artur/b2broker-interview/src/handler"
	"github.com/talkanbaev-artur/b2broker-interview/src/server"
)

func main() {
	w := server.NewWriter()

	go func() {
		if err := w.Run(); err != nil {
			logrus.WithError(err).Error("stopped with error")
		}
	}()

	//Seeting to debug level to see the inner msgs about handling commands
	logrus.SetLevel(logrus.DebugLevel)

	ctx, cancel := context.WithCancel(context.Background())
	//using chain create to  create and start a server
	h, err := handler.NewHandler("foo", "bar").WithServer(w).Start(ctx)
	if err != nil {
		logrus.Fatal(err)
	}
	h.AddSymbols("BTC")
	time.AfterFunc(time.Second*10, func() {
		h.AddSymbols("USDT", "NOT BTC", "ETH", "WHTEVR")
	})
	out := h.GetOut()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

L:
	for {
		select {
		case <-c:
			cancel()
			logrus.Infof("Shutdown...")
			break L
		case msg := <-out:
			fmt.Printf("Data: %v\n", msg)
		}
	}
}
