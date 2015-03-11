package shared

import (
	"time"
)

type Message struct {
	Author    string
	Timestamp time.Time
	Data      string
}
