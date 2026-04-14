package ui

import (
	"fmt"
	"sync"
	"time"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner displays an animated spinner with a message while work is in progress.
type Spinner struct {
	msg  string
	stop chan struct{}
	wg   sync.WaitGroup
}

// NewSpinner creates a new Spinner with the given message.
func NewSpinner(msg string) *Spinner {
	return &Spinner{msg: msg, stop: make(chan struct{})}
}

// Start begins spinning in a background goroutine.
func (s *Spinner) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		i := 0
		for {
			select {
			case <-s.stop:
				fmt.Printf("\r\033[K") // clear the spinner line
				return
			default:
				fmt.Printf("\r%s %s", spinnerFrames[i%len(spinnerFrames)], s.msg)
				time.Sleep(80 * time.Millisecond)
				i++
			}
		}
	}()
}

// Stop halts the spinner and clears the line.
func (s *Spinner) Stop() {
	close(s.stop)
	s.wg.Wait()
}
