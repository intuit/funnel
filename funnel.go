package funnel

// Funnel is a Go library that provides unification of identical operations (e.g. API requests).
// In the case of multiple goroutines trying to execute an identical operation at the same time, Funnel will execute the
// operation only once and return the result to all the goroutines attempting to execute it.
// In addition, the results of the operation can be cached to prevent any identical operations being performed for a set period of time.

import (
	"errors"
	"sync"
	"time"

	"github.com/mohae/deepcopy"
	"github.com/tevino/abool"
)

//const error
var timeoutError = errors.New("Timeout expired while waiting for operation execution to complete")

// opResult holds the result from executing of operation
type opResult struct {

	// res contains the actual result from the operation
	res interface{}

	// The error from the operation
	err error

	// panicErr contains the error from panic when a panic occurred during the processing of the operation
	panicErr interface{}
}

type empty struct{}

// operationInProcess holds the data on an operation in progress.
type operationInProcess struct {
	operationId string

	// Channel done will be closed when the operation was completed. The channel does not transmit
	// information (relevant only to its closing) and therefore there is no meaning the type of the channel.
	done chan empty

	// The result from executing of operation, will be available after the channel will be closed.
	opResult

	// true when this operation has been deleted from the funnel
	deleted *abool.AtomicBool

	// Time at which this operation started executing
	startTime time.Time

	// Operation will be marked completed once a result is returned
	completed *abool.AtomicBool
}

// A Config structure is used to configure the Funnel
type Config struct {
	// the result of an operation can be configured to be cacheable. The cache time-to-live indicates the time for
	// which the result can remain cached. A time to live value of 0 prohibits caching.
	cacheTtl time.Duration

	// the maximum time that goroutines will wait for ending of operation.
	timeout time.Duration
}

// The purpose of Funnel is to prevent running of identical operations in concurrently.
// when receiving requests for a specific operation when an identical operation already in process, the other
// operation requests will wait until the end of the operation and then will use the same result.
type Funnel struct {

	// operationInProcess holds all the operations that are currently in progress.
	// Operations will be wiped off the map automatically when the cache time-to-live will be expired.
	opInProcess map[string]*operationInProcess
	sync.Mutex

	// Configuration for Funnel
	config Config
}

type Option func(*Config)

//WithCacheTtl defines the maximum time that goroutines will wait for ending of operation (the default is one minute)
func WithTimeout(t time.Duration) Option {
	return func(cfg *Config) {
		cfg.timeout = t
	}
}

//WithCacheTtl defines the time for which the result can remain cached (the default is 0 )
func WithCacheTtl(cTtl time.Duration) Option {
	return func(cfg *Config) {
		cfg.cacheTtl = cTtl
	}
}

// Return a pointer to a new Funnel. By default the timeout is one minute and
// the cacheTtl is 0. You can pass options to change it, for example:
//
//	// Create Funnel with cacheTtl of 5 seconds and timeout of 3 minutes.
// 	funnel.New(funnel.WithCacheTtl(time.Second*5),funnel.WithTimeout(time.Minute*3))
//
func New(option ...Option) *Funnel {
	cfg := Config{
		timeout:  time.Duration(time.Minute),
		cacheTtl: 0,
	}

	for _, opt := range option {
		opt(&cfg)
	}

	return &Funnel{
		opInProcess: make(map[string]*operationInProcess),
		config:      cfg,
	}
}

// Waiting for completion of the operation and then returns the operation's result or error in case of timeout.
func (op *operationInProcess) wait(timeout time.Duration) (res interface{}, err error) {
	operationElapsedTime := time.Since(op.startTime)
	operationTimeoutRemaining := timeout - operationElapsedTime

	select {
	case <-op.done:
		if op.panicErr != nil { // If the operation ended with panic, this pending request also ends the same way.
			panic(op.panicErr)
		}
		return op.res, op.err
	case <-time.After(operationTimeoutRemaining):
		if op.completed.IsSet() {
			return op.res, op.err
		}
		return nil, timeoutError
	}
}

// getOperationInProcess returns structure that holds the data about an identical operation currently in progress,
// in case an identical operation does not exist, it starts a new one.
func (f *Funnel) getOperationInProcess(operationId string, opExeFunc func() (interface{}, error)) (op *operationInProcess) {
	f.Lock()
	defer f.Unlock()

	if op, found := f.opInProcess[operationId]; found {
		return op
	}

	// In case there is no such an operation in process, it creates a new one and executes it.
	op = &operationInProcess{
		operationId: operationId,
		done:        make(chan empty),
		startTime:   time.Now(),
		deleted:     abool.New(),
		completed:   abool.New(),
	}
	f.opInProcess[operationId] = op

	// Executing the operation
	go func(opInProc *operationInProcess) {
		// closeOperation must be performed within defer function to ensure the closure of the channel.
		defer f.closeOperation(opInProc)
		opInProc.res, opInProc.err = opExeFunc()
		opInProc.completed.Set()
	}(op)

	return op
}

// Closes the operation by updates the operation's result and closure of done channel.
func (f *Funnel) closeOperation(op *operationInProcess) {
	f.Lock()
	defer f.Unlock()

	//Check if the operation completed after a timeout which would result in the operation being deleted from the funnel.
	if op.deleted.IsSet() {
		return
	}

	if rr := recover(); rr != nil {
		op.panicErr = rr
	}

	// Deletion of operationInProcess from the map will occur only when the cache time-to-live will be expired.
	go func() {
		time.Sleep(f.config.cacheTtl)
		f.deleteOperation(op)
	}()

	// Releases all the goroutines which are waiting for the operation result.
	close(op.done)
}

// Delete the operation from the map.
// Once deleted, we do not hold the operation's result anymore, therefore any further request for the
// same operation will require re-execution of it.
func (f *Funnel) deleteOperation(operation *operationInProcess) {
	if operation.deleted.IsSet() {
		return
	}

	f.Lock()
	defer f.Unlock()

	//each timeout will call deleteOperation.  Only the first timeout should carry out deletion since a stalled app may delete a recreated operation with the same id.
	if !operation.deleted.IsSet() {
		delete(f.opInProcess, operation.operationId)
		operation.deleted.SetTo(true)
	}
}

// Execute receives an identifier of the operation and a callback function to execute.
// The first request to funnel with this identifier will result in the callback function being executed in a new goroutine.
// All other requests (with the same identifier) will wait for the result of the first execution.
// IMPORTANT: The returned object is shared between all the requesting callers.
// Use ExecuteAndCopyResult to return a dedicated (copied) object.
func (f *Funnel) Execute(operationId string, opExeFunc func() (interface{}, error)) (res interface{}, err error) {
	op := f.getOperationInProcess(operationId, opExeFunc)
	// If op is completed return the result
	res, err = op.wait(f.config.timeout) // Waiting for completion of operation
	if err == timeoutError {
		f.deleteOperation(op)
	}
	return
}

// IMPORTANT: Only exported field values can be copied over.
func (f *Funnel) ExecuteAndCopyResult(operationId string, opExeFunc func() (interface{}, error)) (res interface{}, err error) {
	opRes, err := f.Execute(operationId, opExeFunc)
	if opRes != nil {
		res = deepcopy.Copy(opRes)
	}
	return res, err
}

func (f *Funnel) IsOpInProgress(operationId string) bool {
	f.Lock()
	defer f.Unlock()

	_, found := f.opInProcess[operationId]
	return found
}
