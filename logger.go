package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

const (
	typeRequest              = 1 << iota
	typeRequestResponse      = 1 << iota
	typeRequestResponseError = 1 << iota
	typeEvent                = 1 << iota
)

const (
	levelConnection = 1 << iota
	levelProtocol   = 1 << iota
	levelTarget     = 1 << iota
)

const (
	fieldLevel       = "level"
	fieldType        = "type"
	fieldTargetID    = "targetID"
	fieldRequest     = "request"
	fieldMethod      = "method"
	fieldInspectorID = "inspectorId"
)

const (
	requestReplyFormat = "%-17s %-32s % 48s(%s) = %s\n"
	requestFormat      = "%-17s %-32s % 48s(%s)\n"
	eventFormat        = "%-17s %-32s % 48s(%s)\n"
	protocolFormat     = "%-17s %-32s\n"
	timeFormat         = "15:04:05.00000000"
	deltaFormat        = "Δ%8.2fms"
)

var (
	responseColor     = color.New(color.FgHiRed).SprintfFunc()
	requestColor      = color.New(color.FgHiBlue).SprintFunc()
	requestReplyColor = color.New(color.FgHiWhite).SprintfFunc()
	eventsColor       = color.New(color.FgGreen).SprintfFunc()
	eventsLabelColor  = color.New(color.FgCyan).SprintfFunc()
	protocolColor     = color.New(color.FgYellow).SprintfFunc()
	protocolError     = color.New(color.FgHiYellow, color.BgRed).SprintfFunc()
	targetColor       = color.New(color.FgHiWhite).SprintfFunc()
	methodColor       = color.New(color.FgHiYellow).SprintfFunc()
	errorColor        = color.New(color.BgRed, color.FgWhite).SprintfFunc()
	protocolTargetID  = center("browser", 32)
)

type FramesFormatter struct {
	lastTime int64
}

func (f *FramesFormatter) Format(e *logrus.Entry) ([]byte, error) {
	message := e.Message
	var timestamp string

	if *flagMicroseconds {
		timestamp = fmt.Sprintf("%d", e.Time.UnixNano()/int64(time.Millisecond))
	} else {
		timestamp = e.Time.Format(timeFormat)
	}

	if *flagDelta {
		var delta string

		if f.lastTime == 0 {
			delta = fmt.Sprintf(deltaFormat, 0.00)
		} else {
			delta = fmt.Sprintf(deltaFormat, math.Abs(float64(e.Time.UnixNano()-f.lastTime)/float64(time.Millisecond)))
		}

		f.lastTime = e.Time.UnixNano()
		timestamp = fmt.Sprintf("%s %s", timestamp, delta)
	}

	var protocolType = -1
	var protocolMethod = ""

	protocolLevel := e.Data[fieldLevel].(int)

	if val, ok := e.Data[fieldType].(int); ok {
		protocolType = val
	}

	if val, ok := e.Data[fieldMethod].(string); ok {
		protocolMethod = val
	}

	if !accept(protocolMethod, message) {
		return []byte{}, nil
	}

	switch protocolLevel {
	case levelConnection:
		switch e.Level {
		case logrus.ErrorLevel:
			return []byte(fmt.Sprintf(protocolFormat, timestamp, errorColor(message))), nil
		case logrus.InfoLevel:
			return []byte(fmt.Sprintf(protocolFormat, timestamp, protocolColor(message))), nil
		}

	case levelProtocol, levelTarget:
		targetID := e.Data[fieldTargetID].(string)

		switch protocolType {
		case typeEvent:
			return []byte(fmt.Sprintf(eventFormat, timestamp, targetColor(targetID), eventsLabelColor(protocolMethod), eventsColor(message))), nil

		case typeRequest:
			return []byte(fmt.Sprintf(requestFormat, timestamp, targetColor(targetID), methodColor(protocolMethod), requestColor(message))), nil

		case typeRequestResponse:
			return []byte(fmt.Sprintf(requestReplyFormat, timestamp, targetColor(targetID), methodColor(protocolMethod), requestReplyColor(e.Data[fieldRequest].(string)), responseColor(message))), nil

		case typeRequestResponseError:
			return []byte(fmt.Sprintf(requestReplyFormat, timestamp, targetColor(targetID), methodColor(protocolMethod), requestReplyColor(e.Data[fieldRequest].(string)), errorColor(message))), nil
		}
	}

	return []byte(fmt.Sprintf("unsupported entry: %+v", e)), nil
}

type multiWriter struct {
	io.Writer
	writers []io.Writer
}

func newMultiWriter(writers ...io.Writer) *multiWriter {
	return &multiWriter{
		Writer:  io.MultiWriter(writers...),
		writers: writers,
	}
}

func (m *multiWriter) Close() (err error) {
	for _, writer := range m.writers {
		if v, ok := writer.(io.Closer); ok && v != os.Stdout {
			v.Close()
		}
	}

	return nil
}

var loggers = make(map[string]*logrus.Logger)

func createLogWriter(filename string) (io.Writer, error) {

	if filename == "" {
		if *flagQuiet {
			return ioutil.Discard, nil
		}

		return os.Stdout, nil
	}

	logFilePath := fmt.Sprintf(*flagDirLogs+"/%s.log", filename)
	dir := filepath.Dir(logFilePath)

	if _, err := os.Stat(dir); err != nil {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return nil, err
		}
	}

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
	if err != nil {
		return nil, err
	}

	if *flagQuiet {
		return newMultiWriter(logFile), nil
	}

	return newMultiWriter(logFile, os.Stdout), nil
}

func createLogger(name string) (*logrus.Logger, error) {

	if _, exists := loggers[name]; !exists {
		writer, err := createLogWriter(name)
		if err != nil {
			return nil, err
		}

		loggers[name] = &logrus.Logger{
			Out:       writer,
			Formatter: new(FramesFormatter),
			Hooks:     make(logrus.LevelHooks),
			Level:     logrus.DebugLevel,
		}
	}

	return loggers[name], nil
}

func destroyLogger(name string) error {
	if logger, exists := loggers[name]; exists {
		if closer, ok := logger.Out.(io.Closer); ok {
			closer.Close()
		}

		delete(loggers, name)
	}

	return nil
}
