package gotermBox

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/antongulenko/golib"
)

var _ golib.Task = &CliLogBoxTask{}

type CliLogBoxTask struct {
	CliLogBox
	updateTask *golib.LoopTask

	// UpdateInterval configures the wait-period between screen-refresh cycles
	UpdateInterval time.Duration

	// Update is called on every refresh cycle to fill the screen with content
	Update func(out io.Writer, width int) error
}

// Should be called as early as possible to intercept all log messages
func (t *CliLogBoxTask) Init() {
	t.CliLogBox.Init()
	t.RegisterMessageHook()
}

func (t *CliLogBoxTask) String() string {
	return "CliLogBoxTask"
}

func (t *CliLogBoxTask) Start(wg *sync.WaitGroup) golib.StopChan {
	t.InterceptLogger()
	if t.Update == nil {
		return golib.NewStoppedChan(errors.New("CliLogBoxTask.Update cannot be nil"))
	}
	t.updateTask = &golib.LoopTask{
		Description: "CliLogBoxTask",
		StopHook: func() {
			t.updateBox()
			t.RestoreLogger()
		},
		Loop: func(stop golib.StopChan) (err error) {
			err = t.updateBox()
			if err == nil {
				stop.WaitTimeout(t.UpdateInterval)
			}
			return
		},
	}
	return t.updateTask.Start(wg)
}

func (t *CliLogBoxTask) Stop() {
	t.updateTask.Stop()
}

func (t *CliLogBoxTask) updateBox() (err error) {
	t.CliLogBox.Update(func(out io.Writer, width int) {
		err = t.Update(out, width)
	})
	return
}
