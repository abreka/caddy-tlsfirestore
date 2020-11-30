package storagefirestore

import (
	"cloud.google.com/go/firestore"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

var errAlreadyLocked = errors.New("certificate is already locked")

// Lock acquires a distributed lock for the given key or blocks until it gets one.
func (s *Storage) Lock(ctx context.Context, key string) error {
	for {
		err := s.attemptLock(ctx, key)

		if err == nil {
			// TODO: go func for context cancel?
			return nil
		}

		if err != errAlreadyLocked {
			return err
		}

		if didAbort := s.sleepOrAbort(ctx, s.randSleepTime()); didAbort {
			return ctx.Err() // Pattern used by certmagic/filestorage.go
		}
	}
}


func (s *Storage) Unlock(key string) error {
	// According to certmagic, Unlock is called after log, even in
	// case of error or timeout of critical section.
	if found := s.unlockLocal(key); !found {
		return fmt.Errorf("lock %s was not found", key)
	}

	// TODO: add nonce and only update if matched?
	_, err := s.keyToRef(key).Update(context.Background(), []firestore.Update{
		{Path: "locked", Value: false},
		{Path: "lockedAt", Value: UTCNow()},
	})

	if err != nil {
		return fmt.Errorf("unable to unlock %s: %w", key, err)
	}


	return nil
}

// attemptLock executes a transaction to acquire a lock on a particular key.
//
// It does this by setting the `locked` and `lockedAt` fields of the particular
// document in firestore in a transaction.
//
// It does not block on errAlreadyLocked failure.
func (s *Storage) attemptLock(ctx context.Context, key string) error {
	if s.hasLockLocal(key) {
		return nil  // We already locally have the lock
	}

	ref := s.keyToRef(key)
	err := s.client.RunTransaction(ctx, func(ctx context.Context, t *firestore.Transaction) error {
		doc, err := t.Get(ref)

		if err != nil {
			if IsDocNotFound(err) {
				// No document yet exists for the key. Create it with the
				// lock already set.
				now := UTCNow()
				return t.Create(ref, &Record{
					Locked:    true,
					LockedAt:  now,
					CreatedAt: now,
					UpdatedAt: now,
				})
			} else {
				return err
			}
		}

		// The document does already exist.

		// Read it
		var cert Record
		err = doc.DataTo(&cert)
		if err != nil {
			return err
		}

		if cert.Locked && !s.isStale(cert.LockedAt) {
			// valid lock found; poll again soon.
			// otherwise, overwrite it.
			return errAlreadyLocked
		}

		return t.Update(ref, []firestore.Update{
			{Path: "locked", Value: true},
			{Path: "lockedAt", Value: UTCNow()},
		})
	})

	if err != nil {
		if err == errAlreadyLocked {
			return err
		}
		return fmt.Errorf("unable to lock %s: %w", key, err)
	}

	go s.keepLockFresh(s.lockLocal(ctx, key), key)

	return nil
}

// keepLockFresh maintains lockedAt to prevent active lock expiration
//
// > To prevent deadlocks, all implementations should put a reasonable
// > expiration on the lock in case Unlock is unable to be called
// > due to some sort of network failure or system crash.
func (s *Storage) keepLockFresh(ctx context.Context, key string) {
	interval := time.Duration(s.FreshnessSeconds) * time.Second
	timer := time.NewTimer(interval)

	for {
		select {
		case <-timer.C:
			err := s.updateFreshness(ctx, key)
			if err != nil {
				// TODO: log
				return
			}
			timer.Reset(interval)
		case <-ctx.Done():
			// Lock relinquished.
			if !timer.Stop() {
				<- timer.C
			}
			return
		}
	}
}

func (s *Storage) updateFreshness(ctx context.Context, key string) error {
	ref := s.keyToRef(key)

	_, err := ref.Update(ctx, []firestore.Update{
		{Path: "lockedAt", Value: UTCNow()},
	})

	return err
}

func (s *Storage) isStale(mTime time.Time) bool {
	// Twice the freshness seconds to allow a latency grace period.
	staleTime := mTime.Add(time.Second * time.Duration(s.FreshnessSeconds) * 2)
	return UTCNow().After(staleTime)
}

func (s *Storage) keyToRef(key string) *firestore.DocumentRef {
	return s.client.Collection(s.Collection).Doc(firestoreSafeKey(key))
}

func firestoreSafeKey(key string) string {
	// Keys are forward slash separated with no leading slash.
	// This doesn't work with firebase since paths are forward
	// slash separated, too.
	return strings.ReplaceAll(key, "/", "\\")
}

func (s *Storage) sleepOrAbort(ctx context.Context, duration time.Duration) (didAbort bool) {
	timer := time.NewTimer(duration)

	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C  // Drain
		}
		return true
	case <-timer.C:
		return false
	}
}

func (s *Storage) randSleepTime() time.Duration {
	delta := float64(s.MaxPollSeconds - s.MinPollSeconds)
	randTime := time.Microsecond * time.Duration(delta * rand.Float64() * float64(time.Millisecond))
	return time.Second * time.Duration(s.MinPollSeconds) + randTime
}

func (s *Storage) hasLockLocal(key string) bool {
	s.m.Lock()
	defer s.m.Unlock()
	_, found := s.locks[key]
	return found
}

func (s *Storage) lockLocal(parent context.Context, key string) context.Context {
	s.m.Lock()
	withCancel, cancel := context.WithCancel(parent)
	s.locks[key] = cancel
	s.m.Unlock()

	return withCancel
}

func (s *Storage) unlockLocal(key string) bool {
	s.m.Lock()
	defer s.m.Unlock()
	if cancel, found := s.locks[key]; found {
		cancel()
		delete(s.locks, key)
		return true
	}
	return false
}

