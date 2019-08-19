package lock

import (
	"bytes"
	"fmt"
	"log"
	"time"

	consulApi "github.com/hashicorp/consul/api"
)

type Lock struct {
	ReadersKey string
	WritersKey string
	Client     *consulApi.Client
}

func NewLock(key string) *Lock {
	c, _ := consulApi.NewClient(consulApi.DefaultConfig())

	return &Lock{
		ReadersKey: fmt.Sprintf("lock/%s/readers", key),
		WritersKey: fmt.Sprintf("lock/%s/writers", key),
		Client:     c,
	}
}

// initialize the lock keys in Consul. not concurrency safe
func (l *Lock) Init() error {
	kv := l.Client.KV()

	e, _, err := kv.Get(l.ReadersKey, nil)
	if err != nil {
		return err
	}
	if e != nil {
		return nil
	}
	_, err = kv.Put(&consulApi.KVPair{
		Key:   l.ReadersKey,
		Value: []byte{byte(0)},
	}, nil)
	if err != nil {
		return err
	}

	e, _, err = kv.Get(l.WritersKey, nil)
	if err != nil {
		return err
	}
	if e != nil {
		return nil
	}
	_, err = kv.Put(&consulApi.KVPair{
		Key:   l.WritersKey,
		Value: []byte{byte(0)},
	}, nil)
	if err != nil {
		return err
	}

	return nil
}

func (l *Lock) RLock() error {
	kv := l.Client.KV()

	// ++readers once readers != -1 and writers == 0
	var wait uint64

	// this still feels racey
	for {
		writersKV, _, err := kv.Get(l.WritersKey, &consulApi.QueryOptions{
			WaitIndex: wait,
		})
		if err != nil {
			return err
		}

		if writersKV == nil {
			return fmt.Errorf("Unexpected missing key %s", l.WritersKey)
		}

		// we can proceed once there are no active/pending writers
		// this is so we dont starve pending writers by piling on readers
		// constantly
		if !bytes.Equal(writersKV.Value, []byte{byte(0)}) {
			log.Printf("[RLock] Writers currently %d, try again at %d",
				writersKV.Value, writersKV.ModifyIndex)
			wait = writersKV.ModifyIndex
			continue
		}

		break
	}

	wait = 0 // reset
	for {
		readersKV, _, err := kv.Get(l.ReadersKey, &consulApi.QueryOptions{
			WaitIndex: wait,
		})
		if err != nil {
			return err
		}

		if readersKV == nil {
			return fmt.Errorf("Unexpected missing key %s", l.ReadersKey)
		}

		// someone else is still writing, so wait for the next change and check again
		if bytes.Equal(readersKV.Value, []byte{byte(0xff)}) {
			log.Printf("[RLock] Readers currently %d, try again at %d",
				readersKV.Value, readersKV.ModifyIndex)
			wait = readersKV.ModifyIndex
			continue
		}

		newCount := readersKV.Value[0] + 1
		log.Printf("Setting %s to %d (%d)", l.ReadersKey,
			int(newCount), readersKV.ModifyIndex)

		newReaders := consulApi.KVPair{
			Key:         l.ReadersKey,
			ModifyIndex: readersKV.ModifyIndex,
			Value:       []byte{newCount},
		}

		ok, _, err := kv.CAS(&newReaders, nil)
		if err != nil {
			return err
		}

		if ok {
			break
		}

		// didnt get it, so start over from the beginning
		// not updating wait here, so the next GET will have an older WaitIndex
		// which will return right away
		log.Println("Failed to CAS lock, retry")
	}

	return nil
}

func (l *Lock) RUnlock() error {
	kv := l.Client.KV()

	// --readers
	var wait uint64
	for {
		readersKV, _, err := kv.Get(l.ReadersKey, &consulApi.QueryOptions{
			WaitIndex: wait,
		})
		if err != nil {
			return err
		}

		if readersKV == nil {
			return fmt.Errorf("Unexpected missing key %s", l.ReadersKey)
		}

		newCount := readersKV.Value[0] - 1
		log.Printf("Setting %s to %d (%d)", l.ReadersKey,
			int(newCount), readersKV.ModifyIndex)

		newReaders := consulApi.KVPair{
			Key:         l.ReadersKey,
			ModifyIndex: readersKV.ModifyIndex,
			Value:       []byte{newCount},
		}

		ok, _, err := kv.CAS(&newReaders, nil)
		if err != nil {
			return err
		}

		if ok {
			break
		}

		// didnt get it, so start over from the beginning
		// not updating wait here, so the next GET will have an older WaitIndex
		// which will return right away
		log.Println("Failed to CAS lock, retry")
	}

	return nil
}

func (l *Lock) Lock() error {
	kv := l.Client.KV()

	// ++writers
	// readers -> -1 once readers == 0

	// increase the number of pending/active writers on the lock
	for {
		writersKV, _, err := kv.Get(l.WritersKey, nil /* QueryOptions */)
		if err != nil {
			return err
		}

		if writersKV == nil {
			return fmt.Errorf("Unexpected missing key %s", l.WritersKey)
		}

		newCount := writersKV.Value[0] + 1
		log.Printf("Setting %s to %d (%d)", l.WritersKey,
			int(newCount), writersKV.ModifyIndex)

		newWriters := consulApi.KVPair{
			Key:         l.WritersKey,
			ModifyIndex: writersKV.ModifyIndex,
			Value:       []byte{newCount},
		}

		// CAS loop to increment writers count
		ok, _ /* WriteMeta */, err := kv.CAS(&newWriters, nil /* WriteOptions */)
		if err != nil {
			return err
		}

		if ok {
			break
		}

		log.Println("Failed to CAS lock, retry")
		time.Sleep(time.Millisecond * 500)
	}

	// set readers to -1 once readers is 0
	var wait uint64
	for {
		readersKV, _, err := kv.Get(l.ReadersKey, &consulApi.QueryOptions{
			WaitIndex: wait,
		})
		if err != nil {
			return err
		}

		if readersKV == nil {
			return fmt.Errorf("Unexpected missing key %s", l.ReadersKey)
		}

		// someone else is still reading, so wait for the next change and check again
		if !bytes.Equal(readersKV.Value, []byte{byte(0)}) {
			log.Printf("[Lock] Readers currently %d, try again at %d",
				readersKV.Value, readersKV.ModifyIndex)
			wait = readersKV.ModifyIndex
			continue
		}

		log.Printf("Setting %s to %d (%d)", l.ReadersKey, -1, readersKV.ModifyIndex)

		newReaders := consulApi.KVPair{
			Key:         l.ReadersKey,
			ModifyIndex: readersKV.ModifyIndex,
			Value:       []byte{byte(0xff)},
		}

		ok, _, err := kv.CAS(&newReaders, nil)
		if err != nil {
			return err
		}

		if ok {
			break
		}

		// didnt get it, so start over from the beginning
		// not updating wait here, so the next GET will have an older WaitIndex
		// which will return right away
		log.Println("Failed to CAS lock, retry")
	}

	return nil
}

// Unlock releases the write lock on this Lock
func (l *Lock) Unlock() error {
	kv := l.Client.KV()

	// --writers
	// readers -> 0

	// decrease the number of pending/active writers on the lock
	for {
		writersKV, _, err := kv.Get(l.WritersKey, nil /* QueryOptions */)
		if err != nil {
			return err
		}

		if writersKV == nil {
			return fmt.Errorf("Unexpected missing key %s", l.WritersKey)
		}

		newCount := writersKV.Value[0] - 1
		log.Printf("Setting %s to %d (%d)", l.WritersKey,
			int(newCount), writersKV.ModifyIndex)

		newWriters := consulApi.KVPair{
			Key:         l.WritersKey,
			ModifyIndex: writersKV.ModifyIndex,
			Value:       []byte{newCount},
		}

		ok, _ /* WriteMeta */, err := kv.CAS(&newWriters, nil /* WriteOptions */)
		if err != nil {
			return err
		}

		if ok {
			break
		}

		log.Println("Failed to CAS lock, retry")
		time.Sleep(time.Millisecond * 500)
	}

	// set the number of active readers to 0
	// here any pending readers are blocking on this value currently being 0xff
	// so we can put directly here without needing to CAS
	_, err := kv.Put(&consulApi.KVPair{
		Key:   l.ReadersKey,
		Value: []byte{byte(0)},
	}, nil)
	if err != nil {
		return err
	}

	return nil
}
