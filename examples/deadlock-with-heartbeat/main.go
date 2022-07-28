package main

import (
	"fmt"
	"os"
	"time"

	"github.com/blokje5/heartbeat"
)

func main() {
	cfg := heartbeat.NewHeartBeatConfig(100 * time.Millisecond, 200 * time.Millisecond)
	beat := heartbeat.NewHeartbeat(cfg)
	monitor := heartbeat.NewHeartBeatMonitor(beat, cfg)

	intStream := make(chan int)
	unreadStream := make(chan interface{})

	go func() {
		if err := monitor.Start(); err != nil {
			return
		}

		defer close(unreadStream)
		for i := range intStream {
			select {
				case <-beat.PulseInterval():
					// notify on heartbeat interval
					if err := beat.SendPulse(); err != nil {
						return
					}
				default:
					// do work
					fmt.Println(i)
					if i > 9 {
						unreadStream <- struct{}{}
					}
			}

		}
	}()
	
	for i := 0; i < 100; i++ {
		select {
		case intStream <- i:
		case healthy := <-monitor.HealthChan():
			fmt.Printf("health status %t\n", healthy)
			if !healthy {
				fmt.Println("monitor is unhealthy")
				os.Exit(1)
			}
		}
	}

	close(intStream)
}


