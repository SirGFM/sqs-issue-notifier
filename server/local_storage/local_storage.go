/*
Package local_storage implements a storage-agnostic API for storing
arbitrary data locally.

Instead of trying to send packages directly to a SQS, they should first be
stored locally in this layer. This way, even if the SQS is unresponsive,
the packages will be stored and sent at a later time.

Although this package accepts data of any type (as it works with bytes), it
ensures the integrity of the stored data.

A local storage must be initialized by calling "New*()" (currently, only a
file system is implemented through "NewFS()"). Then, reading of new data
may be done in a goroutine by waiting for a signal, while the main
goroutine stores new data.

If the local storage isn't empty on boot, the next local storage will be
properly signaled on start.

Example:

	store := local_storage.NewFS("some-dir")

	go func() {
		for store.Wait() {
			data, err = store.Get()
			if err != nil {
				// handle err
				continue
			}

			bytes := data.Bytes()
			// do something with this data
			bytes.Remove()
		}
	} ()

	err := store.Store([]byte("some-data"))
	if err != nil {
		// handle err
	}

	// ...

	store.Close()
*/
package local_storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	flock "github.com/theckman/go-flock"
	"fmt"
	"io/fs"
	"log"
	"os"
	"sync"
	"path/filepath"
	"time"
)

// Store defines the API to manage data in a local storage.
type Store interface {
	// Store data in the local storage.
	Store(data []byte) error

	// Get a node from the local storage. This node won't be retrieved
	// again until it's either Close()'d or Remove()'d.
	Get() (Data, error)

	// Wait blocks until anything was stored in the local storage. Returns
	// ErrStoreClosed if the Store was closed, and ErrTimedOut if no
	// message was received in a timely manner. A 'nil' return indicates
	// that there is something ready to be retrieved.
	Wait() error

	// Close this store.
	Close() error
}

// Data defines the API to read and erase data retrieved from the local
// storage.
type Data interface {
	// The contents of this object.
	Bytes() []byte

	// Remove this object from the local storage.
	Remove() error

	// Close this object, allowing it to be retrieved again.
	Close() error
}

// notifier handles events and synchronization between the store and nodes.
type notifier struct {
	// Notify the waiting goroutine that something was added. Although
	// simpler, using a channel for waking the other thread requires a
	// receiver (which may not exist).
	cond *sync.Cond

	// Timer used to signal that a Wait should timeout.
	timer *time.Timer

	// Number of known queued messages.
	queued int

	// Signals that the store should continue running.
	run bool

	// Forcefully wakeup a Waiting goroutine.
	forceWake bool
}

// fsStore store data in the file system.
type fsStore struct {
	// The directory were data is stored.
	dir string

	// The directory were lock files are kept.
	lock_dir string

	// Handles waiting and walking the store.
	wait *notifier
}

// The format of the time used in file names.
const time_format = "2006-01-02-15-04-05-"

func (f fsStore) Store(data []byte) error {
	// Store the data as the file "<time>-<hash>".
	now := time.Now().Format(time_format)

	hash := sha256.Sum256(data)
	hash_hex := hex.EncodeToString(hash[:])

	filename := now + hash_hex

	// Lock the file to ensure that even if two identical events were
	// received at the same time, only one would be stored.
	lock := flock.New(filepath.Join(f.lock_dir, filename))
	if locked, err := lock.TryLock(); err != nil {
		log.Printf("local_storage/Store: TryLock failed: %+v\n", err)
		return ErrStoreLockFailed
	} else if !locked {
		return ErrDuplicatedStore
	}
	// TODO: (*Flock)Unlock() simply unlocks the flock, but does not erase
	// the lock file. Keep the lock file around until the service is
	// restarted or until the file is consumed.
	defer lock.Unlock()

	file := filepath.Join(f.dir, filename)

	// Alternatively, check that the file does not exist. Otherwise, the
	// same event may have arrived duplicated (but after the first message
	// was properly handled).
	if _, err := os.Stat(file); !errors.Is(err, fs.ErrNotExist) {
		return ErrDuplicatedStore
	}

	err := os.WriteFile(file, data, 0600)
	if err != nil {
		log.Printf("local_storage/Store: Write failed: %+v\n", err)
		return ErrStoreFailed
	}

	f.wait.cond.L.Lock()
	f.wait.queued++
	f.wait.cond.L.Unlock()
	f.wait.cond.Signal()
	return nil
}

func (f fsStore) Get() (Data, error) {
	var data Data

	// Walk over every file in f.dir returning the first valid Data.

	walk := func (path string, d fs.DirEntry, err error)  (ret_err error) {
		if d.IsDir() && path != f.dir {
			return fs.SkipDir
		} else if d.IsDir() {
			return err
		}

		// Try to lock the current file, so it may be used exclusively.
		filename := filepath.Base(path)
		lock := flock.New(filepath.Join(f.lock_dir, filename))
		if locked, err := lock.TryLock(); err != nil {
			log.Printf("local_storage/Get: TryLock failed: %+v\n", err)
			return ErrGetLockFailed
		} else if !locked {
			// This file is already being read. Continue walking.
			return nil
		}

		// SkipDir indicates a success, as no further processing is done.
		defer func() {
			if ret_err != fs.SkipDir {
				lock.Unlock()
			}
		} ()

		// Try to read the file and check its integrity.
		hash_offset := len(time_format)
		if len(filename) < hash_offset {
			// TODO: Remove the file?
			log.Printf("local_storage/Get: Invalid file: %s\n", path)
			return nil
		}
		hash_str := filename[hash_offset:]

		file_data, err := os.ReadFile(path)
		if err != nil {
			// TODO: Remove the file?
			log.Printf("local_storage/Get: Couldn't read file %s: %+v\n", path, err)
			return nil
		}

		hash := sha256.Sum256(file_data)
		hash_hex := hex.EncodeToString(hash[:])
		// This is only used for integrity (as in, data corruption), so no
		// need to use subtle.
		if hash_hex != hash_str {
			// TODO: Remove the file?
			log.Printf("local_storage/Get: Corrupted file: %s\n", path)
			return nil
		}

		// On success, return SkipDir to stop further processing and
		// assign the data captured by closure.
		data = fsData {
			data: file_data,
			file_path: path,
			lock: lock,
			wait: f.wait,
		}
		return fs.SkipDir
	}

	err := filepath.WalkDir(f.dir, walk)
	if err != nil {
		log.Printf("local_storage/Get: Couldn't read any file: %+v\n", err)
		return nil, ErrGetFailed
	} else if data == nil {
		return nil, ErrGetEmpty
	}

	return data, nil
}

func (f fsStore) Wait() error {
	f.wait.cond.L.Lock()
	for n := f.wait; n.queued == 0 && n.run && !n.forceWake; {
		n.cond.Wait()
	}

	var err error
	if f.wait.forceWake {
		err = ErrTimedOut
		f.wait.forceWake = false
	} else if !f.wait.run {
		err = ErrStoreClosed
	}

	f.wait.cond.L.Unlock()
	return err
}

func (f fsStore) Close() error {
	f.wait.cond.L.Lock()
	f.wait.run = false
	if f.wait.timer != nil {
		f.wait.timer.Stop()
	}
	f.wait.cond.L.Unlock()
	f.wait.cond.Signal()
	return nil
}

// fsData manages data read from the file system.
type fsData struct {
	// The file's contents.
	data []byte

	// The file's path.
	file_path string

	// The file's flock. It's always locked and must be released by either
	// calling Remove() or Close().
	lock *flock.Flock

	// Notifies the store that this data was removed.
	wait *notifier
}

func (fd fsData) Bytes() []byte {
	// Return a copy of the data to ensure that it won't be tampered.
	tmp := []byte{}
	return append(tmp, fd.data...)
}

func (fd fsData) Remove() error {
	err := os.Remove(fd.file_path)
	if err != nil {
		log.Printf("local_storage/Remove: Couldn't remove the data file: %+v\n", err)
		return ErrRemoveFailed
	}

	fd.lock.Unlock()
	err = os.Remove(fd.lock.Path())
	if err != nil {
		// No need to return this error, as it's useless for the rest of
		// the application.
		log.Printf("local_storage/Remove: Couldn't remove the lock file: %+v\n", err)
	}

	fd.wait.cond.L.Lock()
	if fd.wait.queued > 0 {
		fd.wait.queued--
	}
	fd.wait.cond.L.Unlock()

	return nil
}

func (fd fsData) Close() error {
	fd.lock.Unlock()
	return nil
}

// NewFS creates a new Store using the file system as the local storage.
// Files are written to dir, and the directory is checked every timeout
// (if the store isn't signaled). Set this to 0 to ignore the timeout.
func NewFS(dir string, timeout time.Duration) Store {
	s := fsStore {
		dir: dir,
		lock_dir: filepath.Join(dir, ".lock"),
		wait: &notifier{
			cond: sync.NewCond(&sync.Mutex{}),
			run: true,
		},
	}

	// Ensure that the lock dir exists and is empty.
	err := os.RemoveAll(s.lock_dir)
	if err != nil {
		panic(fmt.Sprintf("local_storage/NewFS: Failed to clean the lock dir: %+v", err))
	}
	err = os.MkdirAll(s.lock_dir, 0755)
	if err != nil {
		panic(fmt.Sprintf("local_storage/NewFS: Failed to create the lock dir: %+v", err))
	}

	// Pre-fill the wait channel with as many files as there are in the
	// directory.
	walk := func (path string, d fs.DirEntry, err error)  (ret_err error) {
		if d.IsDir() && path != s.dir {
			return fs.SkipDir
		} else if d.IsDir() {
			return err
		}

		// TODO: Clean up invalid files
		s.wait.queued++

		return nil
	}
	err = filepath.WalkDir(s.dir, walk)
	if err != nil {
		panic(fmt.Sprintf("local_storage/NewFS: Failed to initialize the local storage: %+v", err))
	}

	// Spawn a goroutine to wake up a Waiting goroutine (if any).
	if timeout != time.Duration(0) {
		s.wait.timer = time.NewTimer(timeout)

		go func(n *notifier) {
			for n.run {
				n.timer.Reset(timeout)
				<-n.timer.C

				n.cond.L.Lock()
				if n.queued == 0 {
					n.forceWake = true
				}
				n.cond.L.Unlock()
				n.cond.Signal()
			}
		} (s.wait)
	}

	return s
}
