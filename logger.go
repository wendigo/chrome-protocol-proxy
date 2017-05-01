package main

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/fatih/color"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
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
	fieldTargetId    = "targetId"
	fieldRequest     = "request"
	fieldMethod      = "method"
	fieldInspectorId = "inspectorId"
)

const (
	requestReplyFormat = "%-17s %s % 48s(%s) = %s\n"
	requestFormat      = "%-17s %s % 48s(%s)\n"
	eventFormat        = "%-17s %s % 48s(%s)\n"
	protocolFormat     = "%-17s %s\n"
	timeFormat         = "15:04:05.00000000"
)

const logsDir = "logs/%s.log"

var (
	responseColor     = color.New(color.FgHiGreen).SprintfFunc()
	requestColor      = color.New(color.FgHiBlue).SprintFunc()
	requestReplyColor = color.New(color.FgGreen).SprintfFunc()
	eventsColor       = color.New(color.FgHiRed).SprintfFunc()
	protocolColor     = color.New(color.FgYellow).SprintfFunc()
	protocolError     = color.New(color.FgHiYellow, color.BgRed).SprintfFunc()
	targetColor       = color.New(color.FgHiWhite).SprintfFunc()
	methodColor       = color.New(color.FgHiYellow).SprintfFunc()
	errorColor        = color.New(color.BgRed, color.FgWhite).SprintfFunc()
)

type FramesFormatter struct {
}

func (f FramesFormatter) Format(e *logrus.Entry) ([]byte, error) {
	message := e.Message
	timestamp := e.Time.Format(timeFormat)

	var protocolType int = -1
	var protocolMethod string = ""

	protocolLevel := e.Data[fieldLevel].(int)

	if val, ok := e.Data[fieldType].(int); ok {
		protocolType = val
	}

	if val, ok := e.Data[fieldMethod].(string); ok {
		protocolMethod = val
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
		targetId := e.Data[fieldTargetId].(string)

		switch protocolType {
		case typeEvent:
			return []byte(fmt.Sprintf(eventFormat, timestamp, targetColor(targetId), methodColor(protocolMethod), eventsColor(message))), nil

		case typeRequest:
			return []byte(fmt.Sprintf(requestFormat, timestamp, targetColor(targetId), methodColor(protocolMethod), requestColor(message))), nil

		case typeRequestResponse:
			return []byte(fmt.Sprintf(requestReplyFormat, timestamp, targetColor(targetId), methodColor(protocolMethod), requestReplyColor(e.Data[fieldRequest].(string)), responseColor(message))), nil

		case typeRequestResponseError:
			return []byte(fmt.Sprintf(requestReplyFormat, timestamp, targetColor(targetId), methodColor(protocolMethod), requestReplyColor(e.Data[fieldRequest].(string)), errorColor(message))), nil
		}
	}

	return []byte(fmt.Sprintf("unsupported entry: %+v", e)), nil
}

func createLogWriter(filename string) (io.Writer, error) {

	if filename == "" {
		if *flagQuiet {
			return ioutil.Discard, nil
		}

		return os.Stdout, nil
	}

	logFilePath := fmt.Sprintf(logsDir, filename)

	dir := filepath.Dir(logFilePath)

	if _, err := os.Stat(dir); err != nil {
		if err := os.MkdirAll(dir, 0777); err != nil {
			return nil, err
		}
	}

	writer, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0777)
	if err != nil {
		return nil, err
	}

	if *flagQuiet {
		return writer, nil
	}

	return io.MultiWriter(writer, os.Stdout), nil
}

func createLogger(filename string) (*logrus.Logger, error) {

	writer, err := createLogWriter(filename)
	if err != nil {
		return nil, err
	}

	return &logrus.Logger{
		Out:       writer,
		Formatter: new(FramesFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.DebugLevel,
	}, nil
}
