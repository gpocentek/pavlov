package action

import "time"

type ActionContext struct {
	Rule      string
	File      string
	Line      string
	GroupBy   string
	Group     string
	Timestamp time.Time
	Vars      map[string]string
}

type Action interface {
	Act(*ActionContext)
	Validate() error
}
