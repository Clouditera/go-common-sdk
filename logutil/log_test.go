package logutil

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// nolint:errcheck
func TestLog(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	logrus.SetOutput(buffer)
	Init()
	SetLogLevel("fatal")
	SetLogLevel("error")
	SetLogLevel("info")
	SetLogLevel("debug")
	LogError("this is a test")
	lines := strings.Split(buffer.String(), "\n")
	require.Equal(t, len(lines), 5)

	buffer.Reset()
	// 这里增加了一层调用来显示warn只会保留三层调用栈
	func() {
		LogWarn("this is a test")
	}()
	lines = strings.Split(buffer.String(), "\n")
	require.Equal(t, len(lines), 5)
}
