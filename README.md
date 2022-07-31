# go-heartbeat

Channels are a powerful primitive in golang. They allow for sharing state between multiple goroutines without needing to use lower level synchronization primitives. However, channels are blocking by default and this can lead to situations where it is hard to distinguish between a goroutine that is waiting for a return on a blocking channel call or a goroutine that stopped functioning (e.g. due to a panic).

go-heartbeat is a package that makes implementing a heartbeat, a pulse for your goroutines, simple.

Say you have a goroutine that is monitoring a channel, but there is a chance it starts blocking on a channel that isn't being read or for some other reason isn't actually making progress. We will create a contrived example to show how this could happen (see examples/deadlock/main.go):

```golang
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
```

When we run this example you will see the following console output:

```console
$ go run examples/deadlock/main.go
0
1
2
3
4
5
6
7
8
9
10
fatal error: all goroutines are asleep - deadlock!
```

Go is smart enough to figure out that a deadlock is created and will panic, however that might not be desirable as that will crash your program.

Instead what we want is to make sure we can restart the goroutine when it becomes unhealthy. Hence the need for a mechanism to monitor the health of a goroutine.

Let's take the previous example and add a heartbeat + a heartbeat monitor (see examples/deadlock-with-heartbeat/main.go):

```golang
cfg := heartbeat.NewConfig(100 * time.Millisecond, 200 * time.Millisecond)
beat := heartbeat.New(cfg)
monitor := heartbeat.NewMonitor(beat, cfg)

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
```

When we run this example you will see the following console output:

```console
$ go run examples/deadlock-with-heartbeat/main.go
0
1
2
3
4
5
6
7
8
9
10
health status false
monitor is unhealthy
exit status 1
```

The HeartbeatMonitor gives us some control and let's us decide how to deal with a deadlock. In this case we simply call `os.Exit(1)` but because we gain control we can do whatever we need. We can even recreate the goroutine and ensure our program eventually prints all 100 integers.
