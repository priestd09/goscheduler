package goscheduler

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"sync"
	"testing"
	"time"
)

func lenSyncMap(m *sync.Map) (length int64) {
	length = 0
	m.Range(func(_, _ interface{}) bool {
		length++
		return true
	})
	return length
}

func isTaskExists(num int) error {
	s := newScheduler()

	// check if all task record in redis has been removed
	keys, err := s.db.Keys(prefixTask + "*").Result()
	if err != nil {
		return errors.New("Interacting with database error: " + err.Error())
	}
	if len(keys) != num {
		return errors.New("There are keys unremoved: " + fmt.Sprintf("%s", keys))
	}
	return nil
}
func isTaskScheduled() error {
	s := newScheduler()

	// check if all task record in redis has been removed
	keys, err := s.db.Keys(prefixTask + "*").Result()
	if err != nil {
		return errors.New("Interacting with database error: " + err.Error())
	}
	if len(keys) != 0 {
		return errors.New("There are keys unremoved: " + fmt.Sprintf("%s", keys))
	}

	// check if manager is not empty
	if lenSyncMap(newScheduler().manager) != 0 {
		return errors.New("Manager is not empty: " + fmt.Sprint(newScheduler().manager))
	}
	return nil
}

func TestInitializer(t *testing.T) {
	Init(&Config{
		DatabaseURI: "",
	})
	Init(&Config{
		DatabaseURI: "redis://127.0.0.1:6379/8",
	})
}

type CustomTask struct {
	ID          string    `json:"uuid"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Information string    `json:"info"`
	Executed    bool      `json:"is_executed"`
}

func (c CustomTask) Identifier() string {
	return c.ID
}
func (c *CustomTask) SetExecuteTime(t time.Time) time.Time {
	c.End = t
	return c.End
}
func (c CustomTask) GetExecuteTime() time.Time {
	return c.End
}
func (c *CustomTask) Execute() error {
	fmt.Println("Custom Task ", c.ID, " is Running: ", c.Information, ", time: ", time.Now().UTC())
	c.Executed = true
	return nil
}
func (c CustomTask) FailRetryDuration() time.Duration {
	return time.Second
}
func TestPoller(t *testing.T) {
	Init(&Config{
		DatabaseURI: "redis://127.0.0.1:6379/8",
	})

	// prepare data into redis database
	s := newScheduler()
	start := time.Now().UTC()
	task1 := &CustomTask{
		ID:          "123",
		Start:       start,
		End:         start.Add(time.Duration(1) * time.Second),
		Information: "TestPoller task 1",
		Executed:    false,
	}
	task3 := &CustomTask{
		ID:          "789",
		Start:       start,
		End:         start.Add(time.Duration(3) * time.Second),
		Information: "TestPoller task 3",
		Executed:    false,
	}
	task2 := &CustomTask{
		ID:          "456",
		Start:       start,
		End:         start.Add(time.Duration(2) * time.Second),
		Information: "TestPoller task 2",
		Executed:    false,
	}
	s.save(task1)
	s.save(task2)
	s.save(task3)

	// test poller
	var customType CustomTask
	if err := Poll(&customType); err != nil {
		t.Error("schedue task fail: ", err)
	}
	if err := isTaskExists(3); err != nil {
		t.Error("task is not scheduled: ", err)
	}

	// sleep to wait execution, a strict wait condition
	// 1 sec tolerance
	wait := start.Add(time.Second * time.Duration(3)).Sub(time.Now().UTC())
	time.Sleep(wait + time.Second)

	// check if customType is used
	if customType != reflect.Zero(reflect.TypeOf(customType)).Interface() {
		t.Error("`customType` has been used: ", customType)
		t.FailNow()
	}

	if err := isTaskScheduled(); err != nil {
		t.Error("There is sill task unschduled, reason: ", err)
	}
	return
}

func TestSchedule(t *testing.T) {
	// prepare database
	Init(&Config{
		DatabaseURI: "redis://127.0.0.1:6379/8",
	})

	// check if manager is not empty
	if lenSyncMap(newScheduler().manager) != 0 {
		t.Error("manager is not empty: ", newScheduler().manager)
		t.FailNow()
	}

	// prepare tasks
	// original: task 1 should execute before task 2
	start := time.Now().UTC()
	task1 := &CustomTask{
		ID:          "123",
		Start:       start,
		End:         start.Add(time.Duration(1) * time.Second),
		Information: "TestSchedule task 1",
	}
	task2 := &CustomTask{
		ID:          "456",
		Start:       start,
		End:         start.Add(time.Duration(2) * time.Second),
		Information: "TestSchedule task 2",
	}

	// schedule tasks 1
	if err := Schedule(task1); err != nil {
		t.Errorf("schedule task 1 error: %s", err.Error())
		t.FailNow()
	}
	// reschedule task 1, then task 1 should execute after task 2
	task1.SetExecuteTime(task1.GetExecuteTime().Add(time.Second * time.Duration(2)))
	if err := Schedule(task1); err != nil {
		t.Errorf("error: %s", err.Error())
		t.FailNow()
	}
	// schedule task 2
	if err := Schedule(task2); err != nil {
		t.Errorf("schedule task 2 error: %s", err.Error())
		t.FailNow()
	}

	// sleep to wait execution, a strict wait condition
	// 1 sec tolerance
	wait := start.Add(time.Second * time.Duration(3)).Sub(time.Now().UTC())
	time.Sleep(wait + time.Second)

	if err := isTaskScheduled(); err != nil {
		t.Error("There is sill task unschduled, reason: ", err)
		t.FailNow()
	}
}

func TestReschedule(t *testing.T) {
	// prepare database
	Init(&Config{
		DatabaseURI: "redis://127.0.0.1:6379/8",
	})

	// check if manager is not empty
	if lenSyncMap(newScheduler().manager) != 0 {
		t.Error("manager is not empty: ", newScheduler().manager)
		t.FailNow()
	}

	start := time.Now().UTC()
	task := &CustomTask{
		ID:          "123",
		Start:       start,
		End:         start.Add(time.Duration(1) * time.Second),
		Information: "TestSchedule task 1",
	}
	// schedule task second later
	if err := Schedule(task); err != nil {
		t.Errorf("schedule task 1 error: %s", err.Error())
		t.FailNow()
	}
	// somehow the database has changed
	// for example the user changed the database manually
	result, err := newScheduler().db.Get(prefixTask + "123").Result()
	if err != nil {
		t.Error("Get from database error 1")
		t.FailNow()
	}
	r := &record{}
	if err := json.Unmarshal([]byte(result), r); err != nil {
		t.Error("Get from database error 2")
		t.FailNow()
	}
	// change
	r.Execution = start.Add(2 * time.Second)
	bytes, err := json.Marshal(r)
	if err != nil {
		t.Error("Get from database error 3")
		t.FailNow()
	}
	if _, err := newScheduler().db.Set(prefixTask+"123", string(bytes), 0).Result(); err != nil {
		t.Error("Get from database error 4")
		t.FailNow()
	}

	// sleep to wait execution, a strict wait condition
	// 1 sec tolerance
	wait := start.Add(time.Second * time.Duration(2)).Sub(time.Now().UTC())
	time.Sleep(wait + time.Second)

	if err := isTaskScheduled(); err != nil {
		t.Error("There is sill task unschduled, reason: ", err)
		t.FailNow()
	}
}

func TestBoot(t *testing.T) {
	Init(&Config{
		DatabaseURI: "redis://127.0.0.1:6379/8",
	})

	// check if manager is not empty
	if lenSyncMap(newScheduler().manager) != 0 {
		t.Error("manager is not empty: ", newScheduler().manager)
		t.FailNow()
	}

	// directly boot an task was originally scheduled 10 secs later
	Boot(&CustomTask{
		ID:          "123",
		Start:       time.Now().UTC(),
		End:         time.Now().UTC().Add(time.Duration(10) * time.Second),
		Information: "TestBoot task 123",
	})
	time.Sleep(time.Duration(5) * time.Millisecond)
	if err := isTaskScheduled(); err != nil {
		t.Error("There is sill task unschduled, reason: ", err)
		t.FailNow()
	}

	start := time.Now().UTC()
	task := &CustomTask{
		ID:          "456",
		Start:       start,
		End:         start.Add(time.Duration(10) * time.Second),
		Information: "TestBoot task 456",
	}
	Schedule(task)
	// boot the task immediately
	Boot(task)
	time.Sleep(time.Duration(5) * time.Millisecond)
	if err := isTaskScheduled(); err != nil {
		t.Error("There is sill task unschduled, reason: ", err)
		t.FailNow()
	}
}

type Func func()

func (c Func) Identifier() string {
	return ""
}
func (c *Func) SetExecuteTime(t time.Time) time.Time {
	return time.Now()
}
func (c Func) GetExecuteTime() time.Time {
	return time.Now()
}
func (c Func) Execute() error {
	return nil
}
func (c Func) FailRetryDuration() time.Duration {
	return time.Second
}
func TestSaveFail(t *testing.T) {
	f := Func(func() { return })
	if err := Schedule(&f); err == nil {
		t.FailNow()
	}
	if _, err := newScheduler().lock(&f, time.Second); err == nil {
		t.FailNow()
	}
}

func TestPollerFail(t *testing.T) {
	Init(&Config{DatabaseURI: "redis://127.0.0.1:6379/8"})
	s := newScheduler()
	defer func() {
		if _, err := s.db.Del(prefixTask + "777").Result(); err == nil {
			return
		}
		t.FailNow()
	}()
	s.db.Set(prefixTask+"777", "123123123", 0).Result()
	var c CustomTask
	Poll(&c)
}

type FailTask struct {
	failCount int
	ID        string
	End       time.Time
}

func (f FailTask) Identifier() string {
	return f.ID
}
func (f FailTask) GetExecuteTime() time.Time {
	return f.End
}
func (f *FailTask) SetExecuteTime(t time.Time) time.Time {
	f.End = t
	return t
}

// Execute defines the actual running task
func (f *FailTask) Execute() error {
	if f.failCount == 3 {
		fmt.Println("success!")
		return nil
	}
	fmt.Println("fail count: ", f.failCount)
	f.failCount++
	return errors.New("still fail")
}

// FailRetryDuration returns the task retry duration if fails
func (f FailTask) FailRetryDuration() time.Duration {
	return time.Second
}
func TestTaskFailRetry(t *testing.T) {
	Init(&Config{DatabaseURI: "redis://127.0.0.1:6379/8"})
	start := time.Now().UTC()

	Boot(&FailTask{
		ID:  "456",
		End: start.Add(time.Duration(1) * time.Second),
	})

	// FailTask is designed to fail three times
	time.Sleep(time.Second * 5)
	if err := isTaskScheduled(); err != nil {
		t.Error("There is sill task unschduled, reason: ", err)
		t.FailNow()
	}
}

func TestDatabaseOperationFail(t *testing.T) {
	Init(&Config{
		DatabaseURI: "redis://127.0.0.1:6379/8",
	})

	s := newScheduler()
	s.db.Close()
	start := time.Now().UTC()
	task := &CustomTask{
		ID:          "456",
		Start:       start,
		End:         start.Add(time.Duration(2) * time.Second),
		Information: "TestSchedule task",
	}

	if err := Schedule(task); err == nil {
		t.Error("TestDatabaseOperationFail schedule task not error")
		t.FailNow()
	}
	if err := Poll(task); err == nil {
		t.Error("TestDatabaseOperationFail poll task not error")
		t.FailNow()
	}
	executeOnce(task)
	if err := recover(task, "random"); err == nil {
		t.Error("TestDatabaseOperationFail recover task not error")
		t.FailNow()
	}
	if _, err := s.getExecuteTime(task); err == nil {
		t.Error("TestDatabaseOperationFail get task execute not error")
		t.FailNow()
	}

	if _, err := s.lock(task, time.Second); err == nil {
		t.Error("TestDatabaseOperationFail lock task not error")
	}
	if err := s.unlock(task); err == nil {
		t.Error("TestDatabaseOperationFail unlock task not error")
	}
	execute(&FailTask{
		failCount: 0,
		ID:        "456",
		End:       time.Now().UTC(),
	})
	retry(&FailTask{
		failCount: 0,
		ID:        "456",
		End:       time.Now().UTC(),
	})
}

func emptyTime() time.Time {
	for {
		if err := isTaskExists(0); err != nil {
			continue
		}
		break
	}
	return time.Now().UTC()
}
func waitSchedule() {
	for {
		if err := isTaskExists(0); err != nil {
			continue
		}
		break
	}
	return
}

func uuid() string {
	out, _ := exec.Command("uuidgen").Output()
	return string(out)
}

func BenchmarkSchedulerDelay(b *testing.B) {
	Init(&Config{
		DatabaseURI: "redis://127.0.0.1:6379/8",
	})
	// Schedule all tasks and end at the same time.
	t1 := time.Now()
	for i := 0; i < b.N; i++ {
		Schedule(&CustomTask{
			ID:          uuid(),
			Start:       time.Now(),
			End:         time.Now(),
			Information: "Benchmark Schedule Task",
		})
		waitSchedule()
	}
	t2 := time.Now()
	fmt.Println("Average scheduling delay: ", t2.Sub(t1)/time.Duration(int64(b.N)))
}

func BenchmarkSchedulerConcurrency(b *testing.B) {
	Init(&Config{
		DatabaseURI: "redis://127.0.0.1:6379/8",
	})
	t1 := time.Now().UTC()
	endTime := t1.Add(time.Second * 2)
	// Schedule all tasks and end at the same time.
	for i := 0; i < b.N; i++ {
		Schedule(&CustomTask{
			ID:          uuid(),
			Start:       t1,
			End:         endTime,
			Information: "Benchmark Schedule Task",
		})
	}
	// Sleep until the end time
	time.Sleep(endTime.Sub(time.Now().UTC()))
	t2 := emptyTime()
	fmt.Println("Total   delay time: ", t2.Sub(endTime))
	fmt.Println("Average delay time: ", t2.Sub(endTime)/time.Duration(int64(b.N)))
}
