package main

import (
	"time"

	"github.com/sirupsen/logrus"
	"github.com/talkanbaev-artur/b2broker-interview/src/server"
)

func main() {
	w := server.NewWriter()

	go func() {
		if err := w.Run(); err != nil {
			logrus.WithError(err).Error("stopped with error")
		}
	}()

	time.Sleep(time.Second)

	err := w.Send(server.MethodRequest{
		ReqID:  "XXX-1",
		Method: server.MethodAuth,
		Args: map[string]string{
			"login":    "foo",
			"password": "bar",
		},
	})
	if err != nil {
		logrus.WithError(err).Error("could not authorize")
	}

	err = w.Send(server.MethodRequest{
		ReqID:  "XXX-2",
		Method: server.MethodExecutions,
		Args: map[string]string{
			"symbol": "BTC/USDT",
		},
	})
	if err != nil {
		logrus.WithError(err).Error("could not authorize")
	}

	read := w.Read()
	for event := range read {
		logrus.WithField("payload", string(event)).Info("got new message")
	}
}
