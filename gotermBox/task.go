package gotermBox

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/antongulenko/golib"
)

// Assert that CliLogBoxTask implements the golib.Task interface.
var _ golib.Task = &CliLogBoxTask{}

// CliLogBoxTask implements the golib.Task interface by creating a CliLogBox,
// capturing all log entries, and regularly updating the screen in a separate goroutine.
type CliLogBoxTask struct {
	CliLogBox
	updateTask    *golib.LoopTask
	updateTrigger chan interface{}

	// UpdateInterval configures the wait-period between screen-refresh cycles.
	UpdateInterval time.Duration

	// MinUpdateInterval can be set to >0 to reduce the screen-refresh frequency
	// even if TriggerUpdate() is called more frequently than every MinUpdateInterval.
	MinUpdateInterval time.Duration

	// Update is called on every refresh cycle to fill the screen with content.
	// See also CliLogBox.Update().
	Update func(out io.Writer, width int) error
}

// Init initializes the receiver and starts collecting log messages.
// It should be called as early as possible in order to not miss any log messages.
// If any log message is fire before calling this, it will not be displayed in the log
// box, and the log box will overwrite the log message on the console.
func (t *CliLogBoxTask) Init() {
	t.updateTrigger = make(chan interface{}, 1)
	t.CliLogBox.Init()
	t.RegisterMessageHooks()

	// Try to directly refresh the screen every time a new message comes in
	t.PushMessageHook = func(msg string) {
		t.TriggerUpdate()
	}
}

// String implements the golib.Task interface.
func (t *CliLogBoxTask) String() string {
	return fmt.Sprintf("CliLogBoxTask (updated every %v)", t.UpdateInterval)
}

// Start implements the golib.Task interface. It intercepts the default logger
// and starts a looping goroutine for refreshing the screen content. When
// the task is stopped, it will automatically restore the operation of the default logger.
func (t *CliLogBoxTask) Start(wg *sync.WaitGroup) golib.StopChan {
	if t.Update == nil {
		return golib.NewStoppedChan(errors.New("CliLogBoxTask.Update cannot be nil"))
	}
	t.InterceptLoggers()
	t.updateTask = &golib.LoopTask{
		Description: "CliLogBoxTask",
		StopHook: func() {
			err := t.updateBox() // One last screen refresh to make sure no messages get lost.
			t.RestoreLoggers()
			golib.Printerr(err)
		},
		Loop: func(stop golib.StopChan) (err error) {
			err = t.updateBox()
			if err == nil {
				// Wait between t.MinUpdateInterval and t.UpdateInterval,
				// but wake up from stop.WaitChan() and t.updateTrigger.
				sleepStart := time.Now()
				select {
				case <-time.After(t.UpdateInterval):
				case <-stop.WaitChan():
				case <-t.updateTrigger:
				}
				sleepTime := time.Now().Sub(sleepStart)
				if diff := t.MinUpdateInterval - sleepTime; diff > 0 {
					select {
					// Don't wait for t.updateTrigger here
					case <-time.After(diff):
					case <-stop.WaitChan():
					}
				}
			}
			return
		},
	}
	return t.updateTask.Start(wg)
}

// Stop stops the goroutine performing screen refresh cycles, and restores the operation of
// the default logger.
func (t *CliLogBoxTask) Stop() {
	t.updateTask.Stop()
}

// Update triggers an immediate screen update.
func (t *CliLogBoxTask) TriggerUpdate() {
	select {
	case t.updateTrigger <- nil:
	default:
	}
}

func (t *CliLogBoxTask) updateBox() (err error) {
	t.CliLogBox.Update(func(out io.Writer, width int) {
		err = t.Update(out, width)
	})
	return
}
