package logview

type LogWriter struct {
	LogChan chan<- string
}

func NewLogWriter(logChan chan<- string) *LogWriter {
	return &LogWriter{
		LogChan: logChan,
	}
}

func (lw *LogWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	lw.LogChan <- msg
	return len(p), nil
}
