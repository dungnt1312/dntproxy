package openai

import "time"

func currentTimeUnix() int64 {
	return time.Now().Unix()
}
