package main

import (
	"fmt"
	"sync"
	"time"
)

type (
	Service interface {
		GetSub(cusId string) *Response
	}

	Response struct {
		Msg string
	}

	service struct {
		requests sync.Map
		client   *client
	}

	client struct{}
)

// simulated slow http request
func (c *client) GetRequest(cusId string) string {
	fmt.Printf("processing %s...\n", cusId)
	time.Sleep(1 * time.Second)
	return fmt.Sprintf("sub %s", cusId)
}

// public interface method to get sub without having to care about internal batching implementation
func (s *service) GetSub(cusId string) *Response {
	v, ok := s.requests.LoadOrStore(cusId, make(chan chan *Response, 100))
	k := v.(chan chan *Response)
	ch := make(chan *Response)
	k <- ch

	if !ok {
		// actual http request is initiated only if channel of listeners didnt exist
		go func(reqs chan chan *Response) {
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("httprequest", r)
				}
			}()
			l := s.client.GetRequest(cusId)
			// when response arrives, close channel, send response to all the waiting goroutines, then delete key from syncmap
			res := &Response{l}
			fmt.Printf("request for %s closed\n", cusId)
			close(reqs)

			for ch := range reqs {
				ch <- res
			}

			s.requests.Delete(cusId)
		}(k)
	}

	res := <-ch
	return res
}

func main() {
	svc := &service{
		requests: sync.Map{},
		client:   &client{},
	}
	requestswg := &sync.WaitGroup{}

	for i := 0; i < 10; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("getSubCall", r)
				}
			}()
			requestswg.Add(1)
			fmt.Println(svc.GetSub("x").Msg)
			requestswg.Done()
		}()
	}
	for i := 0; i < 10; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("getSubCall", r)
				}
			}()
			requestswg.Add(1)
			fmt.Println(svc.GetSub("y").Msg)
			requestswg.Done()
		}()
	}

	requestswg.Wait()

	// after "x" is done, making another getsub request with x will make another 'processing x' line
	fmt.Println(svc.GetSub("x").Msg)
}

// output from go run main.go:
// processing y...
// processing x...
// request for x closed
// sub x
// request for y closed
// sub y
// sub x
// sub x
// sub x
// sub x
// sub y
// sub y
// sub y
// sub y
// sub y
// sub y
// sub y
// sub y
// sub y
// sub x
// sub x
// sub x
// sub x
// sub x
// processing x...
// request for x closed
// sub x

// sub x = one GetSub method invocation
// processing = one slow call to external api
// this allows multiple invocations of the resolver to wait for the first request (per id)'s
// response then return the same result to each individual goroutine
