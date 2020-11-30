package storagefirestore

import "time"

type Record struct {
	Raw       []byte    `firestore:"raw"`
	Locked    bool      `firestore:"locked"`
	LockedAt  time.Time `firestore:"lockedAt"`
	CreatedAt time.Time `firestore:"createdAt"`
	UpdatedAt time.Time `firestore:"updatedAt"`
}

