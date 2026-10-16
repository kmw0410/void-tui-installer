package installer

import "testing"

func TestParseEvents(t *testing.T) {
	for _, line := range []string{`{"version":1,"type":"stage","name":"partitioning","status":"running"}`, `{"version":1,"type":"result","status":"failed"}`} {
		if _, err := ParseEvent([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	for _, line := range []string{`not json`, `{"version":2,"type":"result","status":"success"}`, `{"version":1,"type":"result","status":"success","password":"secret"}`, `{"version":1,"type":"stage","name":"unknown","status":"running"}`, `{"version":1,"type":"result","status":"success"} {}`} {
		if _, err := ParseEvent([]byte(line)); err == nil {
			t.Fatal("invalid protocol accepted")
		}
	}
}
func TestProtocolRequiresVerification(t *testing.T) {
	p := protocol{}
	if p.accept(Event{Type: "result", Status: "success"}) == nil {
		t.Fatal("premature success accepted")
	}
	p = protocol{}
	for _, s := range Stages {
		for _, status := range []string{"running", "completed"} {
			if e := p.accept(Event{Type: "stage", Name: s, Status: status}); e != nil {
				t.Fatal(e)
			}
		}
	}
	if e := p.accept(Event{Type: "result", Status: "success"}); e != nil {
		t.Fatal(e)
	}
	if p.accept(Event{Type: "log"}) == nil {
		t.Fatal("post-result event accepted")
	}
}
