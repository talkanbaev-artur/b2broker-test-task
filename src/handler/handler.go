package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/talkanbaev-artur/b2broker-interview/src/server"
	"github.com/talkanbaev-artur/b2broker-interview/src/util"
)

//inner struct to conviniently store cred pair
type credentials struct {
	login    string
	password string
}

//
type Handler struct {
	w     *server.Writer
	creds credentials

	ctx    context.Context
	cancel context.CancelFunc
	cnt    int

	symbols    []string
	readersCnt int

	isRunning bool
	nots      *util.Notifications

	out chan map[string]string
}

//creates the Handler that can connect to a server if it is set with certain creds
func NewHandler(login, password string) *Handler {
	creds := credentials{login: login, password: password}
	h := &Handler{creds: creds, nots: util.NewNotifications()}
	h.out = make(chan map[string]string, 100)
	h.readersCnt = 1
	return h
}

var ErrServerIsNotSet = errors.New("server for the handler is not set")
var ErrServerIsAlreadyRunning = errors.New("server is already running")

var ErrAuthenticationFailed = errors.New("authentication for the remote server failed")
var ErrSubscriptionFailed = errors.New("subcription for a symbol failed")

//sets the writer server for the handler
func (h *Handler) WithServer(w *server.Writer) *Handler {
	h.w = w
	return h
}

//Sets the default readers count for the server writer
//
//Values less than 1 have no meaning, so the default min is always 1
func (h *Handler) WithNReaders(count int) *Handler {
	if count < 1 {
		count = 1 //set minimal count to 2 so expiration auth won't block the read
	}
	h.readersCnt = count
	return h
}

//Tries to start the handler. In case of error handler does not start and won't process msgs
//
//It can be restarted though (in case of the transport issue or smth)
func (h *Handler) Start(ctx context.Context) (*Handler, error) {
	if h.isRunning {
		return h, ErrServerIsAlreadyRunning
	}
	if h.w == nil {
		return h, ErrServerIsNotSet
	}
	ctx, cancel := context.WithCancel(ctx)
	h.ctx, h.cancel = ctx, cancel
	for i := 0; i < h.readersCnt; i++ {
		go h.read()
	}
	err := h.authenticate()
	if err != nil {
		cancel()
		return h, ErrAuthenticationFailed
	}
	err = h.subscribeToAll(h.symbols)
	if err != nil {
		cancel()
		return h, ErrSubscriptionFailed
	}
	h.isRunning = true
	return h, nil
}

//util function to call subscribe on a list
func (h *Handler) subscribeToAll(symbols []string) error {
	for _, s := range symbols {
		err := h.subscribe(s)
		if err != nil {
			return err
		}
	}
	logrus.Debug("Subscribed successfully to", symbols)
	return nil
}

//Subscribes for the list of symbols, in a sync way
func (h *Handler) AddSymbols(symbols ...string) error {
	h.symbols = append(h.symbols, symbols...)
	if h.isRunning {
		err := h.subscribeToAll(symbols)
		if err != nil {
			return err
		}
	}
	return nil
}

//subscribe is a syncronous function that sends the subscribe request
//and then waits for the answer
//
//Made it syncronous to all the immediate feedback for the library user
//though it make us require a lot of readers just to make sure that we would reade the msg
func (h *Handler) subscribe(symbol string) error {
	id := h.getNextReqID()
	err := h.w.Send(server.MethodRequest{
		ReqID:  id,
		Method: server.MethodExecutions,
		Args: map[string]string{
			"symbol": symbol,
		},
	})
	if err != nil {
		return err
	}
	c, err := h.nots.AddPromiseChan(id)
	if err != nil {
		return err
	}
	ctx, _ := context.WithTimeout(h.ctx, time.Second*10)
	select {
	case <-ctx.Done():
		return errors.New("timeout")
	case err := <-c:
		h.nots.Delete(id)
		return err
	}
}

//authenticate is a syncronous function that sends the auth request
//and then waits for the answer
//syncronous behaviour here is preffered, as failed authentication
//should not be discarded
func (h *Handler) authenticate() error {
	id := h.getNextReqID()
	err := h.w.Send(server.MethodRequest{
		ReqID:  id,
		Method: server.MethodAuth,
		Args: map[string]string{
			"login":    h.creds.login,
			"password": h.creds.password,
		},
	})
	if err != nil {
		return err
	}
	c, err := h.nots.AddPromiseChan(id)
	if err != nil {
		return err
	}
	ctx, _ := context.WithTimeout(h.ctx, time.Second*10)
	select {
	case <-ctx.Done():
		return errors.New("timeout")
	case err := <-c:
		h.nots.Delete(id)
		if err != nil {
			return err
		}
		logrus.Debugf("Authentication successful")
		return nil
	}
}

//async version to not block the caller
func (h *Handler) asyncAuth() error {
	id := h.getNextReqID()
	err := h.w.Send(server.MethodRequest{
		ReqID:  id,
		Method: server.MethodAuth,
		Args: map[string]string{
			"login":    h.creds.login,
			"password": h.creds.password,
		},
	})
	if err != nil {
		return err
	}
	c, err := h.nots.AddPromiseChan(id)
	if err != nil {
		return err
	}
	ctx, _ := context.WithTimeout(h.ctx, time.Second*10)
	go func() {
		select {
		case <-ctx.Done():
			return
		case err := <-c:
			h.nots.Delete(id)
			if err != nil {
				logrus.Warnf("Authentication failed, reason: %s", err.Error())
				//h.asyncAuth()
				//optional - I don't want to create a recursion here, but it will anyway be cut byt the server
				//if auth won't be alive for a long time
				return
			}
			logrus.Debugf("Authentication successful")
		}
	}()
	return nil
}

//generate the id for the next call and bumps the counter
func (h *Handler) getNextReqID() string {
	val := h.cnt
	h.cnt++
	return fmt.Sprintf("XXX-%d", val)
}

//util fstruct to accept all msgs and then map by type
type msg struct {
	ReqID  *string            `json:"req_id"`
	Method string             `json:"method"`
	Data   *map[string]string `json:"data"`
	Status bool               `json:"status"`
	Error  string             `json:"error"`
}

//Returns the channel with data from the writer server
func (h *Handler) GetOut() chan map[string]string {
	return h.out
}

func (h *Handler) read() {
	out := h.w.Read()
	for {
		select {
		case <-h.ctx.Done():
			return
		case data, ok := <-out:
			if !ok {
				h.cancel()
				return
			}
			var m msg
			json.Unmarshal(data, &m)
			//Handle Status responses
			if m.ReqID != nil {
				var err error = nil
				if m.Error != "" {
					err = errors.New(m.Error)
				}
				h.nots.Notify(*m.ReqID, err)
				continue
			}
			//handle auth expiration
			if m.Method == server.MethodAuthExpiring {
				logrus.Debugf("Authentication is expiring, sending the auth request")
				err := h.asyncAuth() //used async version to not block the reader while waiting for the response
				if err != nil {
					logrus.Warnf("failed to call async auth, reason: %s", err.Error())
				}
				//anyway if it fails there is no special logic, it will be just resent next time
				//or just check the handler.go:217
				continue
			}
			//handle data
			if m.Method == server.MethodExecutions && m.Data != nil {
				h.out <- *m.Data
			}
		}
	}
}
