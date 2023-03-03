package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/talkanbaev-artur/b2broker-interview/src/server"
)

//tests for public logic

func TestNewHandlerCorrectParams(t *testing.T) {
	login, password := "foo", "bar"
	h := NewHandler(login, password)
	assert.NotNilf(t, h.nots, "Handler's notifications is nil")
	assert.Equalf(t, login, h.creds.login, "Login is not set")
	assert.Equalf(t, password, h.creds.password, "Password is not set")
	assert.Equal(t, 1, h.readersCnt)
}

func createNewHandler() *Handler {
	login, password := "foo", "bar"
	return NewHandler(login, password)
}

func TestWithTestSetsTheServer(t *testing.T) {
	h := createNewHandler()
	w := &server.Writer{}
	h = h.WithServer(w)
	assert.Equalf(t, w, h.w, "Server is not set")
}

func TestNewHandlerIsNotStatingWithoutServer(t *testing.T) {
	h := createNewHandler()
	h, err := h.Start(context.TODO())
	assert.ErrorIs(t, err, ErrServerIsNotSet)
	assert.Equalf(t, false, h.isRunning, "expected is running flag to be false")
}

func TestNewHanlderStartsWithServer(t *testing.T) {
	w := server.NewWriter() // it is mock server anyway
	h := createNewHandler().WithServer(w)
	h, err := h.Start(context.TODO())
	assert.NoErrorf(t, err, "Server didn't start")
	assert.Equalf(t, true, h.isRunning, "Server is running flag is not true")
}

func TestServerDoesNotStartSecondTime(t *testing.T) {
	w := server.NewWriter() // it is mock server anyway
	h := createNewHandler().WithServer(w)
	h, err := h.Start(context.TODO())
	assert.NoErrorf(t, err, "Server didn't start")
	h, err = h.Start(context.TODO())
	assert.ErrorIsf(t, err, ErrServerIsAlreadyRunning, "Second start call is unhandeled")
	assert.Equalf(t, true, h.isRunning, "Server is running flag is not true")
}

func TestGetOut(t *testing.T) {
	h := createNewHandler()
	c := h.GetOut()
	assert.IsType(t, make(chan map[string]string), c)
}

func TestAddSymbols(t *testing.T) {
	symbols := []string{"USDT", "BTC"}
	h := createNewHandler()
	h.AddSymbols(symbols...)
	assert.Equal(t, symbols, h.symbols)
}

func TestReadersCntChangeCorrect(t *testing.T) {
	h := createNewHandler().WithNReaders(10)
	assert.Equal(t, 10, h.readersCnt)
}

func TestReadersCntChangeInCorrect(t *testing.T) {
	h := createNewHandler().WithNReaders(-10)
	assert.Equal(t, 1, h.readersCnt)
}
