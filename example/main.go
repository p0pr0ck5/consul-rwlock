package main

import (
  "fmt"
  "log"
  "time"

  "github.com/p0pr0ck5/consul-rwlock"
)

func main() {
  fmt.Println("hello world")

  lock := lock.NewLock("foo")

  lock.Init()

  go func() {
    log.Println("lock 1")
    lock.Lock()
    log.Println("lock 1 acq")
    time.Sleep(time.Second * 3)
    log.Println("unlock 1")
    lock.Unlock()
  }()

  time.Sleep(time.Millisecond * 100)

  go func() {
    log.Println("rlock 1")
    lock.RLock()
    log.Println("rlock 1 acq")
    time.Sleep(time.Second * 2)
    log.Println("runlock 1")
    lock.RUnlock()
  }()

  go func() {
    log.Println("rlock 2")
    lock.RLock()
    log.Println("rlock 2 acq")
    time.Sleep(time.Second * 2)
    log.Println("runlock 2")
    lock.RUnlock()
  }()

  go func() {
    log.Println("rlock 3")
    lock.RLock()
    log.Println("rlock 3 acq")
    time.Sleep(time.Second * 2)
    log.Println("runlock 3")
    lock.RUnlock()
  }()

  time.Sleep(time.Millisecond * 1500)

  go func() {
    log.Println("rlock 4")
    lock.RLock()
    log.Println("rlock 3 acq")
    time.Sleep(time.Second * 2)
    log.Println("runlock 3")
    lock.RUnlock()
  }()

  time.Sleep(time.Second * 10)
}
