package main

import (
	"fmt"
	"time"

	"github.com/changkun/goscheduler"
)

// CustomTask define your custom task struct
type CustomTask struct {
	ID          string    `json:"uuid"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Information string    `json:"info"`
}

// Identifier should returns a unique string for the task, usually can return an UUID
func (c CustomTask) Identifier() string {
	return c.ID
}

// GetExecuteTime should returns the excute time of the task
func (c CustomTask) GetExecuteTime() time.Time {
	return c.End
}

// SetExecuteTime can set the execution time of the task
func (c *CustomTask) SetExecuteTime(t time.Time) time.Time {
	c.End = t
	return c.End
}

// Execute defines the actual running task
func (c *CustomTask) Execute() error {
	// implement your task execution in
	fmt.Println("Task is Running: ", c.Information)
	return nil
}

// FailRetryDuration returns the task retry duration if fails
func (c CustomTask) FailRetryDuration() time.Duration {
	return time.Second
}

func main() {
	// Init goscheduler database
	goscheduler.Init(&goscheduler.Config{
		DatabaseURI: "redis://127.0.0.1:6379/8",
	})

	// When goscheduler database is initiated,
	// call Poller to recover all unfinished task
	var task CustomTask
	goscheduler.Poll(&task)

	// A task should be executed in 10 seconds
	task = CustomTask{
		ID:          "123",
		Start:       time.Now().UTC(),
		End:         time.Now().UTC().Add(time.Duration(10) * time.Second),
		Information: "this is a task message message",
	}
	fmt.Println("Retry duration if execution failed: ", task.FailRetryDuration())

	// first schedule the task at 10 seconds later
	goscheduler.Schedule(&task)
	// however we decide to boot the task immediately
	goscheduler.Boot(&task)

	// let's sleep 2 secs wait for the retult of the task
	time.Sleep(time.Second * 2)
}
