[![Build Status](https://travis-ci.org/mrooz/funnel.svg?branch=master)](https://travis-ci.org/mrooz/funnel)
[![codecov](https://codecov.io/gh/mrooz/funnel/branch/master/graph/badge.svg)](https://codecov.io/gh/mrooz/funnel)
[![Go Report Card](https://goreportcard.com/badge/github.com/mrooz/funnel)](https://goreportcard.com/report/github.com/mrooz/funnel)



# Funnel #

Funnel is a Go library that provides unification of identical operations (e.g. API requests).

## Installation

```
go get github.com/intuit/funnel
```

## Usage ##

```go
import "github.com/intuit/funnel"
```

Funnel package is designed for scenarios where multiple goroutines are trying to execute an identical operation at the same time. It will take care of deduplication, execute the operation only once and return the results to all of them.
A simple example:
```go
import (
        "github.com/intuit/funnel"
        ...
)


func main() {
        fnl := funnel.New()
        var numOfOps uint64 = 0

        //The following operation will be performed only once and all the 100 goroutines will get the same result

        operation := func() (interface{}, error) {
                url := "http://www.golang.org"
                response, err := http.Get(url)
                if err != nil {
                        return nil, err
                }
                defer response.Body.Close()
                atomic.AddUint64(&numOfOps, 1)
                return response.Status, nil
        }

        var wg sync.WaitGroup
        wg.Add(100)
        for i := 0; i < 100; i++ {
                go func() {
                        defer wg.Done()
                        // "operation_id" is a string which uniquely identifies the operation
                        opRes, err := fnl.Execute("operation_id", operation)
                        fmt.Println(opRes)

                }()
        }
        wg.Wait()
        opsFinal := atomic.LoadUint64(&numOfOps)
        fmt.Println(opsFinal) // numOfOps == 1
}
```




In addition, the results of the operation can be cached, to prevent any identical operations being performed for a set period of time.
A simple example:
```go
import (
        "github.com/intuit/funnel"
        ...
)
func main() {
        fnl := funnel.New(funnel.WithCacheTtl(time.Second*5))

        var numOfOps int = 0

        operation1 := func() (interface{}, error) {
                time.Sleep(time.Second)
                numOfOps++
                return "op1-res", nil
        }

        res, err := fnl.Execute("operation1", operation1)

        time.Sleep(time.Second*2)

        res, err = fnl.Execute("operation1", operation1) //It gets the result from the previous operation and not performs the operation again
        // numOfOps == 1

}
```

## Contributing

1. Fork it
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Commit your changes (`git commit -am 'Add some feature'`)
4. Push to the branch (`git push origin my-new-feature`)
5. Create new Pull Request

## License ##

See `LICENSE` for details