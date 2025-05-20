package logging

var ParseLogLine = parseLogLine

var ReadLogFile = func(logger *Logger) ([]byte, error) {
	return logger.readLogFile()
}
