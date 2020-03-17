package funnel

import (
	"errors"
	"math/rand"
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

				if res == nil && err == timeoutError {
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

/*
The following tests the ability of the funnel to create a new operation while an operation of the same id has timed out and its execution function is still running
An unexpired cacheTTL on a timedout operation should also not prohibit creating the new operation
*/
func TestWithTimedoutReruns(t *testing.T) {
	fnl := New(WithTimeout(time.Millisecond*50), WithCacheTtl(time.Millisecond*100))

	var numOfGoREndWithTimeout uint64 = 0
	var numOfStartedOperations uint64 = 0

	numOfOperations := 50
	numOfGoroutines := 50

	for op := 0; op < numOfOperations; op++ {
		var wg sync.WaitGroup
		wg.Add(numOfGoroutines) //Only when numOfGoroutines operations time out we run the next batch of numOfGoroutines operations.  Each such batch is expected to run in a newly created operation request

		for i := 0; i < numOfGoroutines; i++ {
			go func(numOfGoroutine int, id string) {
				defer wg.Done()

				res, err := fnl.Execute(id, func() (interface{}, error) {
					atomic.AddUint64(&numOfStartedOperations, 1)

					time.Sleep(time.Millisecond * 100)
					return id + "ended successfully", errors.New("no error")
				})

				if res == nil && err == timeoutError {
					atomic.AddUint64(&numOfGoREndWithTimeout, 1)
				}

			}(i, "StaticOperationId")

		}
		wg.Wait()
	}

	numOfGoREndWithTimeoutFinal := atomic.LoadUint64(&numOfGoREndWithTimeout)
	if int(numOfGoREndWithTimeoutFinal) != numOfOperations*numOfGoroutines {
		t.Error("Number of operations that ended with timeout expired is not as expected, expected ", numOfOperations*numOfGoroutines, ", got ", numOfGoREndWithTimeoutFinal)

	}

	numOfStartedOperationsFinal := atomic.LoadUint64(&numOfStartedOperations)
	if int(numOfStartedOperationsFinal) != numOfOperations {
		t.Error("Number of operation execution starts is not as expected, expected ", numOfOperations, ", got ", numOfStartedOperations)

	}
}

/*
	All operation execution requests on the same operation instance should timeout at the same time.  The expiry time is determined by the timeout parameter and the time of the first execution request.
*/
func TestOperationAbsoluteTimeout(t *testing.T) {
	funnelTimeout := time.Duration(500 * time.Millisecond)
	operationSleepTime := time.Duration(550 * time.Millisecond)
	numOfOperationRequests := 10
	requestDelay := time.Duration(30 * time.Millisecond)
	operationId := "TestUnifiedTimeout"

	fnl := New(WithTimeout(funnelTimeout))

	var wg sync.WaitGroup
	wg.Add(numOfOperationRequests)

	start := time.Now()
	for i := 0; i < numOfOperationRequests; i++ {
		go func() {
			defer wg.Done()
			fnl.Execute(operationId, func() (interface{}, error) {
				time.Sleep(operationSleepTime)
				return operationId + "ended successfully", errors.New("no error")
			})
		}()
		time.Sleep(requestDelay)
	}

	wg.Wait()

	elapsedTimeAllRequests := time.Since(start)
	expectedOperationTimeoutWithGrace := funnelTimeout + time.Duration(100*time.Millisecond)

	if elapsedTimeAllRequests > expectedOperationTimeoutWithGrace {
		t.Error("Expected all operation request to timeout at the same time, funnelTimeout", funnelTimeout, " Elapsed time for all operations", elapsedTimeAllRequests)
	}
}

var getRandomInt = func() (interface{}, error) {
	time.Sleep(time.Millisecond * 50)
	res := rand.Int()
	return &res, nil
}

/*
Test ExecuteAndCopyResult function. Validate that ExecuteAndCopyResult returns copied object and not a shared object between the requesting callers.
*/
func TestExecuteAndCopyResult(t *testing.T) {
	fnl := New()
	var wg sync.WaitGroup

	var num1, num2 *int

	wg.Add(2)

	// preform getRandomInt twice, with same operation id - both goroutines are expected to get same result.
	// each goroutine stores the returned result, the results would be compared later
	go func() {
		defer wg.Done()
		res, _ := fnl.ExecuteAndCopyResult("opId", getRandomInt)
		num1 = res.(*int)
	}()

	go func() {
		defer wg.Done()
		res, _ := fnl.ExecuteAndCopyResult("opId", getRandomInt)
		num2 = res.(*int)
	}()

	// wait until both goroutines finish
	wg.Wait()

	// we are expecting ExecuteAndCopyResult to preform deep copy, the returned results for same operation
	// should have same values but different addresses
	if *num1 != *num2 {
		t.Error("Objects' values are expected to be the same. values received:", *num1, ",", *num2)
	}

	if &num1 == &num2 {
		t.Error("Objects' addresses are expected to be different. addresses received:", &num1, ",", &num2)
	}

}
