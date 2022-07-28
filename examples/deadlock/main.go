package main

import (
	"fmt"
)

func main() {
	intStream := make(chan int)
	unreadStream := make(chan interface{})

	go func() {
		defer close(unreadStream)
		for i := range intStream {
			fmt.Println(i)
			if i > 9 {
				unreadStream <- struct{}{}
			}
		}
	}()
	
	for i := 0; i < 100; i++ {
		select {
		case intStream <- i:
		}
	}

	close(intStream)
}


