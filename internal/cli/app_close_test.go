package cli

import (
	"errors"
	"testing"
)

type errorCloseApp struct {
	commandTestApp
}

func (e errorCloseApp) Close() error {
	return errors.New("close failed")
}

func TestCloseApp_Nil(t *testing.T) {
	closeApp(nil)
}

func TestCloseApp_Error(t *testing.T) {
	closeApp(errorCloseApp{})
}
