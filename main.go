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

func (c *client) GetRequest(cusId string) string {
	fmt.Printf("processing %s...\n", cusId)
	time.Sleep(1 * time.Second)
	return fmt.Sprintf("sub %s", cusId)
}

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
			// when response arrives, close channel and send response to all the waiting goroutines
			res := &Response{l}
			fmt.Printf("request for %s closed\n", cusId)
			close(reqs)
			s.requests.Delete(cusId)
			for ch := range reqs {
				ch <- res
			}
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
