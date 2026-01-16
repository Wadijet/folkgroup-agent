/*
Package logger chứa hook để filter log dựa trên config.
Hook này sẽ kiểm tra mỗi log entry và quyết định có nên ghi log hay không.
*/
package logger

import (
	"io"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

// LogFilterHook là hook để filter log dựa trên config
type LogFilterHook struct {
	// consoleWriter: Writer cho console (có thể là os.Stdout hoặc io.Discard)
	consoleWriter io.Writer

	// fileWriter: Writer cho file (có thể là file writer hoặc io.Discard)
	fileWriter io.Writer

	// originalConsoleWriter: Writer gốc cho console (để restore khi cần)
	originalConsoleWriter io.Writer

	// originalFileWriter: Writer gốc cho file (để restore khi cần)
	originalFileWriter io.Writer

	// mu: Mutex để bảo vệ writers
	mu sync.RWMutex
}

// NewLogFilterHook tạo hook mới
func NewLogFilterHook(consoleWriter, fileWriter io.Writer) *LogFilterHook {
	return &LogFilterHook{
		consoleWriter:        consoleWriter,
		fileWriter:           fileWriter,
		originalConsoleWriter: consoleWriter,
		originalFileWriter:    fileWriter,
	}
}

// Levels trả về các log levels mà hook này sẽ xử lý
func (h *LogFilterHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire được gọi mỗi khi có log entry
// Hook này sẽ kiểm tra config và quyết định có nên ghi log hay không
func (h *LogFilterHook) Fire(entry *logrus.Entry) error {
	// Lấy agent ID từ entry fields hoặc từ biến môi trường
	agentID := ""
	if agentIDVal, ok := entry.Data["agentId"].(string); ok {
		agentID = agentIDVal
	} else if agentIDVal, ok := entry.Data["agent_id"].(string); ok {
		agentID = agentIDVal
	} else {
		// Fallback: lấy từ environment variable
		agentID = os.Getenv("AGENT_ID")
	}

	// Trích xuất context từ log entry
	ctx := ExtractLogContext(entry, agentID)

	// Kiểm tra xem có nên log hay không
	shouldLog := ShouldLog(ctx)
	if !shouldLog {
		// Không log, nhưng vẫn cần return nil để không làm gián đoạn flow
		return nil
	}

	// Cần log, nhưng hook này không thực sự ghi log
	// Hook chỉ filter, việc ghi log vẫn do logger thực hiện
	// Tuy nhiên, để filter theo log method (console/file), ta cần can thiệp vào writer

	// Kiểm tra config để xem có cần filter theo log method không
	config := GetLogFilterConfig()
	if config != nil && config.Enabled {
		// Kiểm tra console
		ctx.LogMethod = "console"
		shouldLogConsole := ShouldLog(ctx)
		if !shouldLogConsole {
			// Chặn console log bằng cách set writer thành io.Discard
			h.mu.Lock()
			if h.consoleWriter != io.Discard {
				h.consoleWriter = io.Discard
			}
			h.mu.Unlock()
		} else {
			// Cho phép console log
			h.mu.Lock()
			if h.consoleWriter == io.Discard {
				h.consoleWriter = h.originalConsoleWriter
			}
			h.mu.Unlock()
		}

		// Kiểm tra file
		ctx.LogMethod = "file"
		shouldLogFile := ShouldLog(ctx)
		if !shouldLogFile {
			// Chặn file log bằng cách set writer thành io.Discard
			h.mu.Lock()
			if h.fileWriter != io.Discard {
				h.fileWriter = io.Discard
			}
			h.mu.Unlock()
		} else {
			// Cho phép file log
			h.mu.Lock()
			if h.fileWriter == io.Discard {
				h.fileWriter = h.originalFileWriter
			}
			h.mu.Unlock()
		}
	}

	return nil
}

// SetConsoleWriter cập nhật console writer
func (h *LogFilterHook) SetConsoleWriter(writer io.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.consoleWriter = writer
	h.originalConsoleWriter = writer
}

// SetFileWriter cập nhật file writer
func (h *LogFilterHook) SetFileWriter(writer io.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.fileWriter = writer
	h.originalFileWriter = writer
}

// GetConsoleWriter trả về console writer hiện tại
func (h *LogFilterHook) GetConsoleWriter() io.Writer {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.consoleWriter
}

// GetFileWriter trả về file writer hiện tại
func (h *LogFilterHook) GetFileWriter() io.Writer {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.fileWriter
}

// FilteringWriter là wrapper đơn giản cho io.Writer
// Việc filter thực sự được thực hiện bởi FilteringFormatter
// Writer này chỉ là wrapper để giữ interface
type FilteringWriter struct {
	writer    io.Writer
	logMethod string
}

// NewFilteringWriter tạo writer mới (wrapper đơn giản)
func NewFilteringWriter(writer io.Writer, logMethod string) *FilteringWriter {
	return &FilteringWriter{
		writer:    writer,
		logMethod: logMethod,
	}
}

// Write ghi dữ liệu vào writer
// Việc filter thực sự được thực hiện bởi FilteringFormatter trước khi Write được gọi
func (fw *FilteringWriter) Write(p []byte) (n int, err error) {
	// Nếu p rỗng (đã bị filter bởi formatter), không ghi
	if len(p) == 0 {
		return 0, nil
	}
	return fw.writer.Write(p)
}

// FilteringEntryHook là hook đơn giản hơn, chỉ kiểm tra và trả về error nếu không nên log
// Hook này sẽ được gọi trước khi log được format và ghi
type FilteringEntryHook struct{}

// NewFilteringEntryHook tạo hook mới
func NewFilteringEntryHook() *FilteringEntryHook {
	return &FilteringEntryHook{}
}

// Levels trả về các log levels mà hook này sẽ xử lý
func (h *FilteringEntryHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire được gọi mỗi khi có log entry
// Hook này sẽ kiểm tra config và quyết định có nên ghi log hay không
// Nếu không nên log, sẽ set entry.Data["__filtered__"] = true để các hook khác có thể kiểm tra
func (h *FilteringEntryHook) Fire(entry *logrus.Entry) error {
	// Lấy agent ID từ entry fields hoặc từ biến môi trường
	agentID := ""
	if agentIDVal, ok := entry.Data["agentId"].(string); ok {
		agentID = agentIDVal
	} else if agentIDVal, ok := entry.Data["agent_id"].(string); ok {
		agentID = agentIDVal
	} else {
		// Fallback: lấy từ environment variable
		agentID = os.Getenv("AGENT_ID")
	}

	// Trích xuất context từ log entry
	ctx := ExtractLogContext(entry, agentID)

	// Kiểm tra xem có nên log hay không (không phân biệt console/file ở đây)
	// Vì ta cần kiểm tra cả 2 phương thức
	shouldLogConsole := true
	shouldLogFile := true

	config := GetLogFilterConfig()
	if config != nil && config.Enabled {
		// Kiểm tra console
		ctx.LogMethod = "console"
		shouldLogConsole = ShouldLog(ctx)

		// Kiểm tra file
		ctx.LogMethod = "file"
		shouldLogFile = ShouldLog(ctx)
	}

	// Lưu kết quả vào entry data để các hook khác hoặc formatter có thể sử dụng
	if !shouldLogConsole && !shouldLogFile {
		// Không log cả 2 phương thức, đánh dấu để skip hoàn toàn
		entry.Data["__filtered__"] = true
		entry.Data["__filter_console__"] = false
		entry.Data["__filter_file__"] = false
	} else {
		entry.Data["__filtered__"] = false
		entry.Data["__filter_console__"] = !shouldLogConsole
		entry.Data["__filter_file__"] = !shouldLogFile
	}

	return nil
}

// IsFiltered kiểm tra xem entry có bị filter không
func IsFiltered(entry *logrus.Entry) bool {
	if filtered, ok := entry.Data["__filtered__"].(bool); ok {
		return filtered
	}
	return false
}

// ShouldLogToConsole kiểm tra xem có nên log ra console không
func ShouldLogToConsole(entry *logrus.Entry) bool {
	if filtered, ok := entry.Data["__filter_console__"].(bool); ok {
		return !filtered
	}
	return true
}

// ShouldLogToFile kiểm tra xem có nên log ra file không
func ShouldLogToFile(entry *logrus.Entry) bool {
	if filtered, ok := entry.Data["__filter_file__"].(bool); ok {
		return !filtered
	}
	return true
}

// FilteringFormatter là formatter wrapper để filter log
type FilteringFormatter struct {
	formatter logrus.Formatter
}

// NewFilteringFormatter tạo formatter mới với khả năng filter
func NewFilteringFormatter(formatter logrus.Formatter) *FilteringFormatter {
	return &FilteringFormatter{
		formatter: formatter,
	}
}

// Format format log entry
func (f *FilteringFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// Kiểm tra xem có bị filter hoàn toàn không
	if IsFiltered(entry) {
		// Trả về empty để không ghi log
		return []byte{}, nil
	}

	// Format log entry
	formatted, err := f.formatter.Format(entry)
	if err != nil {
		return formatted, err
	}

	// Kiểm tra xem có nên log ra console không
	// Nếu không nên log ra console, ta cần loại bỏ phần console (nhưng điều này khó vì ta không biết phần nào là console)
	// Vì vậy, ta sẽ để formatter format bình thường và để writer xử lý
	// Nhưng vấn đề là writer không biết được context

	// Giải pháp: Sử dụng 2 formatter riêng biệt cho console và file
	// Hoặc sử dụng hook để set flag và kiểm tra trong formatter
	// Tạm thời, ta sẽ chỉ filter toàn bộ (không phân biệt console/file)
	// Để filter riêng console/file, cần một cách tiếp cận khác (có thể dùng 2 logger riêng)

	return formatted, nil
}

// FilteringMultiWriter là multi-writer với khả năng filter
type FilteringMultiWriter struct {
	consoleWriter io.Writer
	fileWriter    io.Writer
	mu            sync.RWMutex
}

// NewFilteringMultiWriter tạo multi-writer mới với khả năng filter
func NewFilteringMultiWriter(consoleWriter, fileWriter io.Writer) *FilteringMultiWriter {
	return &FilteringMultiWriter{
		consoleWriter: consoleWriter,
		fileWriter:    fileWriter,
	}
}

// Write ghi dữ liệu vào cả 2 writers (console và file) dựa trên filter
func (fmw *FilteringMultiWriter) Write(p []byte) (n int, err error) {
	// Lưu ý: Ở đây ta không thể biết được context (agent, job, level) từ p[]
	// Vì vậy, ta sẽ ghi vào cả 2 writers và để hook filter ở trên xử lý
	// Hoặc ta có thể parse p[] để lấy thông tin, nhưng điều này phức tạp hơn

	// Tạm thời, ta sẽ ghi vào cả 2 writers
	// Việc filter sẽ được xử lý bởi FilteringEntryHook và FilteringFormatter

	fmw.mu.RLock()
	consoleWriter := fmw.consoleWriter
	fileWriter := fmw.fileWriter
	fmw.mu.RUnlock()

	// Ghi vào console
	if consoleWriter != nil {
		consoleWriter.Write(p)
	}

	// Ghi vào file
	if fileWriter != nil {
		fileWriter.Write(p)
	}

	return len(p), nil
}

// SetConsoleWriter cập nhật console writer
func (fmw *FilteringMultiWriter) SetConsoleWriter(writer io.Writer) {
	fmw.mu.Lock()
	defer fmw.mu.Unlock()
	fmw.consoleWriter = writer
}

// SetFileWriter cập nhật file writer
func (fmw *FilteringMultiWriter) SetFileWriter(writer io.Writer) {
	fmw.mu.Lock()
	defer fmw.mu.Unlock()
	fmw.fileWriter = writer
}
