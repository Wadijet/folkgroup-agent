/*
Package logger: bridge chuyển standard log package sang logrus.
Mọi log.Printf/log.Println đều đi qua logrus → cùng format và filter.
*/
package logger

import (
	"bytes"
	"io"
	"strings"
	"sync"
)

const stdLogLoggerName = "stdlog"

// StdLogBridge là io.Writer chuyển output của standard log package sang logrus.
// Mỗi dòng (kết thúc bằng \n) được ghi thành một entry logrus với logger "stdlog",
// level Info, theo đúng format chung và đi qua log filter.
type StdLogBridge struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

// NewStdLogBridge tạo bridge để dùng với log.SetOutput(bridge).
// Sau khi gọi log.SetOutput(logger.NewStdLogBridge()), mọi log.Printf/log.Println
// sẽ đi qua logrus với logger_name=stdlog, cùng format và filter.
func NewStdLogBridge() io.Writer {
	return &StdLogBridge{}
}

// Write implement io.Writer. Gom dữ liệu theo dòng, mỗi dòng ghi một entry logrus.
func (b *StdLogBridge) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	n = len(p)
	_, _ = b.buf.Write(p)
	for {
		line, err := b.buf.ReadString('\n')
		if err == io.EOF {
			// Đẩy lại vào buffer để xử lý lần sau
			b.buf.WriteString(line)
			break
		}
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimRight(line, "\r")
		if line != "" {
			b.logLine(line)
		}
	}
	return n, nil
}

func (b *StdLogBridge) logLine(line string) {
	l := GetLogger(stdLogLoggerName)
	l.WithField("source", "stdlog").Info(line)
}
