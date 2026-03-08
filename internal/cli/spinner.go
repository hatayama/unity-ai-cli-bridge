package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const (
	spinnerInitialDelay = 200 * time.Millisecond
	spinnerFrameDelay   = 120 * time.Millisecond
)

type operationSpinner struct {
	writer  io.Writer
	message string
	enabled bool
	stop    chan struct{}
	done    chan struct{}
}

func newOperationSpinner(writer io.Writer, message string) *operationSpinner {
	return &operationSpinner{
		writer:  writer,
		message: strings.TrimSpace(message),
		enabled: supportsSpinner(writer) && strings.TrimSpace(message) != "",
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

func (spinner *operationSpinner) Start() {
	if !spinner.enabled {
		return
	}

	go spinner.run()
}

func (spinner *operationSpinner) Stop() {
	if !spinner.enabled {
		return
	}

	close(spinner.stop)
	<-spinner.done
}

func (spinner *operationSpinner) run() {
	defer close(spinner.done)

	frames := []string{"|", "/", "-", "\\"}
	initialDelay := time.NewTimer(spinnerInitialDelay)
	ticker := time.NewTicker(spinnerFrameDelay)
	defer initialDelay.Stop()
	defer ticker.Stop()

	frameIndex := 0
	rendered := false
	for {
		select {
		case <-spinner.stop:
			if rendered {
				spinner.clear()
			}
			return
		case <-initialDelay.C:
			rendered = true
			spinner.render(frames[frameIndex])
		case <-ticker.C:
			if !rendered {
				continue
			}

			frameIndex = (frameIndex + 1) % len(frames)
			spinner.render(frames[frameIndex])
		}
	}
}

func (spinner *operationSpinner) render(frame string) {
	_, _ = fmt.Fprintf(spinner.writer, "\r%s %s", frame, spinner.message)
}

func (spinner *operationSpinner) clear() {
	_, _ = fmt.Fprint(spinner.writer, "\r\033[2K\r")
}

func supportsSpinner(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

func runWithSpinner[T any](writer io.Writer, message string, operation func() (T, error)) (T, error) {
	spinner := newOperationSpinner(writer, message)
	spinner.Start()

	result, err := operation()

	spinner.Stop()
	return result, err
}
