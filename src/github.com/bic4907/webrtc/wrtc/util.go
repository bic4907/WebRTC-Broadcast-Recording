package wrtc

import (
	"fmt"
	"github.com/google/uuid"
	"time"
)

func log(id uuid.UUID, str string) {
	uid, _ := id.Value()
	t := time.Now().Format("2006-01-02 15:04:05")

	fmt.Println(fmt.Sprintf("[%s] %s %s", uid, t, str))
}

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
