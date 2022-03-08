package setupcloser

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
)

// SetupCloseHandler creates a 'listener' on a new goroutine which will notify the
// program if it receives an interrupt from the OS. We could then handle this by calling
// a "clean up procedure" and exiting the program.
func CloseListener() {
	sigTermlisteningChannel := make(chan os.Signal)
	signal.Notify(sigTermlisteningChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigTermlisteningChannel
		color.Blue("\r- Ctrl+C pressed... exiting... Thank you!")
		// exit indicating success
		os.Exit(0)
	}()
}
