package storagefirestore

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

func IsDocNotFound(err error) bool {
	return status.Code(err) == codes.NotFound
}

func UTCNow() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}
