package funnel

import (
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBasic(t *testing.T) {
	fnl := New()

	var ops uint64 = 0
	var wg sync.WaitGroup
	numOfOperations := 50
	numOfGoroutines := 50
	wg.Add(numOfGoroutines * numOfOperations)

	for op := 0; op < numOfOperations; op++ {
		opId := "operation" + strconv.Itoa(op)
		for i := 0; i < numOfGoroutines; i++ {
			go func(numOfGoroutine int, id string) {
				defer wg.Done()
				if numOfGoroutine%2 == 1 {
					time.Sleep(time.Millisecond * 200)
				}
				res, err := fnl.Execute(id, func() (interface{}, error) {
					time.Sleep(time.Millisecond * 100)
					atomic.AddUint64(&ops, 1)
					return id + "ended successfully", errors.New("no error")
				})

				if res != id+"ended successfully" || err.Error() != "no error" {
					t.Error("The results of operations is not as expected")
				}

			}(i, opId)

		}
	}

	wg.Wait()
	opsFinal := atomic.LoadUint64(&ops)
	if int(opsFinal) != numOfOperations*2 {
		t.Error("Number of operations performed was higher than expected, expected ", numOfOperations*2, ", got ", opsFinal)
	}
}

func TestWithCacheTtl(t *testing.T) {
	fnl := New(WithCacheTtl(time.Second))

	var ops uint64 = 0
	var wg sync.WaitGroup
	numOfOperations := 50
	numOfGoroutines := 50
	wg.Add(numOfGoroutines * numOfOperations)

	for op := 0; op < numOfOperations; op++ {
		opId := "operation" + strconv.Itoa(op)
		for i := 0; i < numOfGoroutines; i++ {
			go func(numOfGoroutine int, id string) {
				defer wg.Done()
				if numOfGoroutine%2 == 1 {
					time.Sleep(time.Millisecond * 500)
				}
				res, err := fnl.Execute(id, func() (interface{}, error) {
					time.Sleep(time.Millisecond * 100)
					atomic.AddUint64(&ops, 1)
					return id + "ended successfully", errors.New("no error")
				})

				if res != id+"ended successfully" || err.Error() != "no error" {
					t.Error("The results of operations is not as expected")
				}

			}(i, opId)

		}
	}

	wg.Wait()
	opsFinal := atomic.LoadUint64(&ops)
	if int(opsFinal) != numOfOperations {
		t.Error("Number of operations performed was higher than expected, expected ", numOfOperations, ", got ", opsFinal)
	}
}

func TestEndsWithPanic(t *testing.T) {
	fnl := New()

	var numOfGoREndWithPanic uint64 = 0
	var wg sync.WaitGroup
	numOfOperations := 50
	numOfGoroutines := 50
	wg.Add(numOfGoroutines * numOfOperations)

	for op := 0; op < numOfOperations; op++ {
		opId := "operation" + strconv.Itoa(op)
		for i := 0; i < numOfGoroutines; i++ {
			go func(numOfGoroutine int, id string) {
				defer func() {
					if rr := recover(); rr != nil {
						if rr != "test ends with panic" {
							t.Error("unexpected panic message")
						}
						atomic.AddUint64(&numOfGoREndWithPanic, 1)
						wg.Done()
					}
				}()

				if numOfGoroutine%2 == 1 {
					time.Sleep(time.Millisecond * 500)
				}
				fnl.Execute(id, func() (interface{}, error) {
					time.Sleep(time.Millisecond * 100)
					panic("test ends with panic")
				})

				t.Error("Should not reach this line because panic should occur")

			}(i, opId)
		}
	}

	wg.Wait()
	numOfGoREndWithPanicFinal := atomic.LoadUint64(&numOfGoREndWithPanic)
	if int(numOfGoREndWithPanicFinal) != numOfOperations*numOfGoroutines {
		t.Error("Number of operations that ended with panic is not as expected, expected ", numOfOperations*numOfGoroutines, ", got ", numOfGoREndWithPanicFinal)
	}
}

func TestWithTimeout(t *testing.T) {
	fnl := New(WithTimeout(time.Millisecond * 50))

	var numOfGoREndWithTimeout uint64 = 0
	var wg sync.WaitGroup
	numOfOperations := 50
	numOfGoroutines := 50
	wg.Add(numOfGoroutines * numOfOperations)

	for op := 0; op < numOfOperations; op++ {
		opId := "operation" + strconv.Itoa(op)
		for i := 0; i < numOfGoroutines; i++ {
			go func(numOfGoroutine int, id string) {
				defer wg.Done()
				res, err := fnl.Execute(id, func() (interface{}, error) {
					time.Sleep(time.Millisecond * 100)
					return id + "ended successfully", errors.New("no error")
				})

				if res == nil && err.Error() == "Timeout expired when waiting to operation in process" {
					atomic.AddUint64(&numOfGoREndWithTimeout, 1)
				}

			}(i, opId)

		}
	}

	wg.Wait()
	numOfGoREndWithTimeoutFinal := atomic.LoadUint64(&numOfGoREndWithTimeout)
	if int(numOfGoREndWithTimeoutFinal) != numOfOperations*numOfGoroutines {
		t.Error("Number of operations that ended with timeout expired is not as expected, expected ", numOfOperations*numOfGoroutines, ", got ", numOfGoREndWithTimeoutFinal)
	}
}
