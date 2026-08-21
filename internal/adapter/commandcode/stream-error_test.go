package commandcode

import (
	"bufio"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMapCommandCodeStreamErrorStatus(t *testing.T) {
	gateway := "Invalid error response format: Gateway request failed"
	if got := mapCommandCodeStreamErrorStatus(gateway, nil); got != http.StatusBadGateway {
		t.Fatalf("gateway status = %d", got)
	}
	code := 429
	if got := mapCommandCodeStreamErrorStatus("quota", &code); got != 429 {
		t.Fatalf("event status = %d", got)
	}
	if got := mapCommandCodeStreamErrorStatus("You have insufficient credits", nil); got != http.StatusPaymentRequired {
		t.Fatalf("credits status = %d", got)
	}
}

func TestReadUntilFirstSSE_ErrorBeforeContent(t *testing.T) {
	body := strings.NewReader("{\"type\":\"reasoning\",\"text\":\"thinking\"}\n{\"type\":\"error\",\"error\":{\"message\":\"Invalid error response format: Gateway request failed\"}}\n")
	br := bufio.NewReader(body)
	state := newStreamState("m", time.Now().Unix())
	prelude, done, status, err := readUntilFirstSSE(br, state)
	if prelude != "" || !done {
		t.Fatalf("prelude=%q done=%v", prelude, done)
	}
	if status != http.StatusBadGateway {
		t.Fatalf("status = %d", status)
	}
	if err == nil || !strings.Contains(err.Error(), "Gateway request failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestReadUntilFirstSSE_FirstText(t *testing.T) {
	body := strings.NewReader("{\"type\":\"text-delta\",\"text\":\"hi\"}\n{\"type\":\"finish\",\"finishReason\":\"stop\"}\n")
	br := bufio.NewReader(body)
	state := newStreamState("m", time.Now().Unix())
	prelude, done, status, err := readUntilFirstSSE(br, state)
	if err != nil || status != 0 || done {
		t.Fatalf("status=%d done=%v err=%v", status, done, err)
	}
	if !strings.Contains(prelude, `"content":"hi"`) {
		t.Fatalf("prelude = %s", prelude)
	}
}
