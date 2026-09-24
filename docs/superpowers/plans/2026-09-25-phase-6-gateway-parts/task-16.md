### Task 16: Fake Anthropic upstream

**A zero-spend upstream that speaks just enough of the Messages API for LiteLLM and Claude Code to talk to it.** Local mode (task 17), the LiteLLM probe (task 18) and the hand-run end-to-end proof (task 23) all need an upstream that answers `POST /v1/messages` like Anthropic does, streams like Anthropic does, and costs nothing. This binary is that upstream. It is not Claude, and it is **not Bedrock**: local mode runs two copies, `-label global` on `:4900` and `-label au` on `:4901`, and the `au` copy speaks the Anthropic format too. It therefore proves that the gatekeeper routed a request to the `au/<id>` group and that the spend was attributed; it proves nothing about LiteLLM's Anthropic-to-Bedrock translation, which is task 24's P04.

**The event sequence is from training knowledge, UNVERIFIED.** The order written here, `message_start`, `content_block_start`, `ping`…, `content_block_delta`, `content_block_stop`, `message_delta`, `message_stop`, with `data:` payloads whose `type` repeats the event name, is how the Messages streaming format is remembered, not a document read for this plan. Task 23 runs unmodified Claude Code against it; if the CLI rejects a stream, the fault is here and the fix is here.

**Two properties exist for task 18, and neither is decoration.**

1. **It answers `POST /` as well as `POST /v1/messages`, and it records every request, including ones it answers `404`.** Task 18's P07 points LiteLLM at this fake twice, once with `api_base` ending in `/v1/messages` and once with a bare host, and reads back which path arrived. That is the Panacea `405` question (the skeleton's pitfall table, row one) answered by measurement: a bare host that LiteLLM posts to verbatim shows up here as `/`, a full path that LiteLLM appends to shows up as `/v1/messages/v1/messages`. A fake that recorded only the requests it served could not tell those apart.
2. **Pings arrive at the gap, flushed, one at a time.** Task 18's P09 measures whether LiteLLM relays a ping the moment it arrives. That measurement is only as good as the upstream: a fake that buffered its own stream would make LiteLLM look guilty. The test for this file proves the fake's own events arrive spaced by `-ping-every`, not together at the end.

**It stores no body and no header value.** `GET /_captured` returns the last 50 requests' method, path, query parameter **names**, header **names**, top-level body **keys**, body length and body SHA-256. The keys are there for task 18's P08 and P10 (did `metadata` survive LiteLLM; did `drop_params` drop the field it was asked to), and they are the JSON object's field names, never a value. A prompt, a token, an `x-litellm-tags` value: none of these can be read back, and a canary test proves it. Global constraint 3 applies to test tools too, because local mode's containers write logs a developer may paste into a ticket.

**It logs one line, at start.** No per-request log line, so there is nothing to scrub.

**Files:**
- `control-plane/cmd/fakeanthropic/main.go` — flags, the handler, the SSE writer, the capture ring (new)
- `control-plane/cmd/fakeanthropic/main_test.go` — the event-order golden, the ping gap, the label, the capture canary, path recording, `count_tokens`, the ring bound, the flag table (new)

**Interfaces:**
- Produces:
  - the binary `fakeanthropic` (the skeleton's shared names: `:4900`, and a second instance on `:4901` labelled `au` in local mode)
  - flags `-addr` (default `:4900`), `-label` (`global` or `au`, default `global`), `-ping-every` (default `1s`; `0` sends no ping)
  - `POST /v1/messages` and `POST /` → a Messages reply, streamed when the body says `"stream": true`
  - `POST /v1/messages/count_tokens` → `{"input_tokens": n}`
  - `GET /_captured` → `{"requests": [record…]}`, oldest first, at most 50
  - every other method and path → `404` in Anthropic's error shape, **recorded** before it is refused
  - the response header `x-fake-route: <label>` on every reply
  - unexported in `package main`: `config`, `parseConfig`, `server`, `newServer`, `(*server).Handler`, `capture`, `record`, `message`, `textBlock`, `usage`, `event`, `writeError`, `writeJSON`, `inputTokens`, `newID`, and the constants `capturedMax`, `pingsPerStream`, `maxBody`, `outputTokens`
- Consumes: the standard library only. No task dependency (wave 1).
- Consumed by: task 17 (`images/gateway/Dockerfile` builds it; `compose.yaml` runs two copies), task 18 (`probe.sh` reads `/_captured` from inside the compose network), task 23 (Claude Code's replies read *fake global reply* and *fake au reply*).

**The wire shapes, complete.** Everything the fake emits, so task 18's `probe.sh` and task 23's checklist can be written without reading the Go:

```text
POST /v1/messages   {"model":"<m>","stream":true,...}

HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
X-Fake-Route: <label>

event: message_start
data: {"message":{"id":"msg_fake_<24 hex>","type":"message","role":"assistant","model":"<m>","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":<n>,"output_tokens":1}},"type":"message_start"}

event: content_block_start
data: {"content_block":{"type":"text","text":""},"index":0,"type":"content_block_start"}

event: ping                       (x3, each after -ping-every; none when it is 0)
data: {"type":"ping"}

event: content_block_delta
data: {"delta":{"text":"fake <label> reply","type":"text_delta"},"index":0,"type":"content_block_delta"}

event: content_block_stop
data: {"index":0,"type":"content_block_stop"}

event: message_delta
data: {"delta":{"stop_reason":"end_turn","stop_sequence":null},"type":"message_delta","usage":{"output_tokens":3}}

event: message_stop
data: {"type":"message_stop"}
```

```text
POST /v1/messages   {"model":"<m>",...}            (no "stream", or "stream": false)

200  application/json
{"id":"msg_fake_<24 hex>","type":"message","role":"assistant","model":"<m>",
 "content":[{"type":"text","text":"fake <label> reply"}],
 "stop_reason":"end_turn","stop_sequence":null,
 "usage":{"input_tokens":<n>,"output_tokens":3}}

POST /v1/messages/count_tokens
200  {"input_tokens":<n>}

<n> = max(1, len(request body in bytes) / 4). Deterministic, so a spend figure is reproducible.

Any error                                             (Anthropic's error shape)
<status> {"type":"error","error":{"type":"<t>","message":"<fixed text, never input>"}}
  400 invalid_request_error   body is not JSON, or has no "model"
  404 not_found_error         any other method or path
  413 request_too_large       body over 8 MiB

GET /_captured
200 {"requests":[
  {"seq":1,"at":"2026-09-25T01:02:03.456Z","method":"POST","path":"/v1/messages",
   "queryKeys":["beta"],"headerNames":["Anthropic-Version","Authorization","Content-Type"],
   "bodyKeys":["max_tokens","messages","model","stream"],"bodyBytes":131,
   "bodySha256":"<64 hex>"}
]}
```

`seq` counts every recorded request since start and never resets, so a probe records the highest `seq` before it acts and reads only the records after it. `GET /_captured` is not itself recorded. `headerNames` are Go's canonical form (`X-Litellm-Tags`); every list is sorted.

**Echoing the model id is what lets LiteLLM price the call.** LiteLLM computes spend from the model it believes it called and the `usage` in the reply (**UNVERIFIED** which of the request's or the reply's model it prices by; task 18's P12 reads a non-zero spend back either way). The fake returns the request's `model` verbatim, so whatever LiteLLM sent (`claude-opus-4-8`, or a prefixed form) comes back.

**The fake needs no authentication.** It is reachable only on the compose network, it holds nothing, and LiteLLM's local config (task 02) sends it whatever key that config names. It records that an `Authorization` or `X-Api-Key` header arrived, by name, which is how task 18 sees that LiteLLM sends the upstream key and not the session's virtual key.

- [ ] **Step 1: Write the failing tests**

Create `control-plane/cmd/fakeanthropic/main_test.go`. It is `package main`, an internal test, because the handler is built by the unexported `newServer`; every package-level name it adds starts with `fake` or is a test function, so it cannot collide with `main.go`.

```go
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

const fakeStreamBody = `{"model":"claude-opus-4-8","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"hi"}]}`

const fakePlainBody = `{"model":"claude-opus-4-8","max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`

func fakeStart(t *testing.T, label string, gap time.Duration) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(newServer(config{Addr: ":0", Label: label, PingEvery: gap}).Handler())
	t.Cleanup(srv.Close)
	return srv
}

func fakePost(t *testing.T, url, body string, hdr map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}
```

```go
type fakeEvent struct {
	Name string
	Data map[string]any
	At   time.Time
}

// fakeReadEvents reads the stream as it arrives and stamps each event with
// the moment its terminating blank line was read. A server that buffers its
// stream shows up here as events whose stamps are all the same.
func fakeReadEvents(t *testing.T, r io.Reader) []fakeEvent {
	t.Helper()
	var out []fakeEvent
	var cur fakeEvent
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			cur.Name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &cur.Data); err != nil {
				t.Fatalf("event %q: data is not JSON: %v", cur.Name, err)
			}
		case line == "":
			cur.At = time.Now()
			out = append(out, cur)
			cur = fakeEvent{}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func fakeFirst(t *testing.T, ev []fakeEvent, name string) fakeEvent {
	t.Helper()
	for _, e := range ev {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("no %s event in the stream", name)
	return fakeEvent{}
}

// Kills: a missing, extra or reordered event, and an event whose data "type"
// disagrees with its "event:" line (clients switch on either).
func TestStreamEventOrderGolden(t *testing.T) {
	srv := fakeStart(t, "global", 5*time.Millisecond)
	resp := fakePost(t, srv.URL+"/v1/messages?beta=true", fakeStreamBody, nil)
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type %q, want text/event-stream", ct)
	}
	var names []string
	for _, e := range fakeReadEvents(t, resp.Body) {
		if e.Data["type"] != e.Name {
			t.Errorf("event %q carries data type %v", e.Name, e.Data["type"])
		}
		names = append(names, e.Name)
	}
	want := "message_start content_block_start ping ping ping content_block_delta content_block_stop message_delta message_stop"
	if got := strings.Join(names, " "); got != want {
		t.Fatalf("events:\n got %s\nwant %s", got, want)
	}
}

// Kills: a fake that never flushes (every stamp equal) and a fake that sends
// its pings without waiting (first ping at once). Either would make task 18's
// P09 blame LiteLLM for buffering the fake did itself.
func TestPingsArriveAtTheGapNotAtTheEnd(t *testing.T) {
	const gap = 150 * time.Millisecond
	srv := fakeStart(t, "global", gap)
	ev := fakeReadEvents(t, fakePost(t, srv.URL+"/v1/messages", fakeStreamBody, nil).Body)
	start, ping, stop := fakeFirst(t, ev, "content_block_start"), fakeFirst(t, ev, "ping"), fakeFirst(t, ev, "message_stop")
	if d := ping.At.Sub(start.At); d < gap*8/10 {
		t.Fatalf("first ping arrived %v after content_block_start; want about %v", d, gap)
	}
	if d := stop.At.Sub(ping.At); d < 2*gap*8/10 {
		t.Fatalf("message_stop arrived %v after the first ping; want at least %v: the stream is buffered", d, 2*gap)
	}
}

func TestZeroGapSendsNoPing(t *testing.T) {
	srv := fakeStart(t, "global", 0)
	for _, e := range fakeReadEvents(t, fakePost(t, srv.URL+"/v1/messages", fakeStreamBody, nil).Body) {
		if e.Name == "ping" {
			t.Fatal("-ping-every 0 sent a ping")
		}
	}
}
```

```go
// Kills: a hard-coded label (the local AU group would read "fake global
// reply" and task 23's step 3 could not see the route change), and a reply
// that does not echo the model (LiteLLM could not price it).
func TestLabelReachesBodyAndHeader(t *testing.T) {
	srv := fakeStart(t, "au", time.Millisecond)
	resp := fakePost(t, srv.URL+"/v1/messages", fakePlainBody, nil)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("x-fake-route") != "au" {
		t.Fatalf("status %d, x-fake-route %q; want 200 and au", resp.StatusCode, resp.Header.Get("x-fake-route"))
	}
	var m message
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if len(m.Content) != 1 || m.Content[0].Text != "fake au reply" {
		t.Fatalf("content %+v, want one text block reading fake au reply", m.Content)
	}
	if m.Model != "claude-opus-4-8" || m.Usage.InputTokens < 1 || m.Usage.OutputTokens != outputTokens {
		t.Fatalf("model %q usage %+v; want the request's model and non-zero usage", m.Model, m.Usage)
	}
	stream := fakePost(t, srv.URL+"/v1/messages", fakeStreamBody, nil)
	if stream.Header.Get("x-fake-route") != "au" {
		t.Fatalf("stream x-fake-route %q", stream.Header.Get("x-fake-route"))
	}
	delta := fakeFirst(t, fakeReadEvents(t, stream.Body), "content_block_delta")
	if got := delta.Data["delta"].(map[string]any)["text"]; got != "fake au reply" {
		t.Fatalf("stream delta %v, want fake au reply", got)
	}
}

// Kills: storing the body, storing a header value, and recording /_captured
// itself. The canaries are a prompt, a session token and an attribution tag.
func TestCapturedHoldsNoBodyAndNoHeaderValue(t *testing.T) {
	const prompt, token, tag = "CANARY-prompt-5b1e9d", "sk-pa-CANARY-token-77c2", "tag-CANARY-3a0f"
	srv := fakeStart(t, "global", time.Millisecond)
	body := `{"model":"claude-opus-4-8","max_tokens":8,"metadata":{"tags":["` + tag + `"]},"messages":[{"role":"user","content":"` + prompt + `"}]}`
	fakePost(t, srv.URL+"/v1/messages?beta=true", body, map[string]string{"Authorization": "Bearer " + token, "x-litellm-tags": tag})
	resp, err := http.Get(srv.URL + "/_captured")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	for _, canary := range []string{prompt, token, tag, "CANARY"} {
		if strings.Contains(string(raw), canary) {
			t.Fatalf("/_captured contains %q:\n%s", canary, raw)
		}
	}
	var got struct {
		Requests []record `json:"requests"`
	}
	if err := json.Unmarshal(raw, &got); err != nil || len(got.Requests) != 1 {
		t.Fatalf("want exactly one record (the POST, not the GET), got %s (%v)", raw, err)
	}
	r, sum := got.Requests[0], sha256.Sum256([]byte(body))
	if r.Path != "/v1/messages" || r.BodySHA256 != hex.EncodeToString(sum[:]) || r.BodyBytes != len(body) {
		t.Fatalf("record %+v does not identify the request", r)
	}
	if !slices.Contains(r.HeaderNames, "Authorization") || !slices.Contains(r.HeaderNames, "X-Litellm-Tags") {
		t.Fatalf("header names %v", r.HeaderNames)
	}
	if fmt.Sprint(r.BodyKeys) != "[max_tokens messages metadata model]" || fmt.Sprint(r.QueryKeys) != "[beta]" {
		t.Fatalf("body keys %v, query keys %v", r.BodyKeys, r.QueryKeys)
	}
}
```

```go
func fakeCaptured(t *testing.T, url string) []record {
	t.Helper()
	resp, err := http.Get(url + "/_captured")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got struct {
		Requests []record `json:"requests"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	return got.Requests
}

// Kills: a fake that answers only /v1/messages (a bare-host api_base would
// look like a LiteLLM failure) and one that records only what it served
// (task 18's P07 could not see a doubled path).
func TestRootIsAnsweredAndEveryPathIsRecorded(t *testing.T) {
	srv := fakeStart(t, "global", time.Millisecond)
	if r := fakePost(t, srv.URL+"/", fakePlainBody, nil); r.StatusCode != http.StatusOK {
		t.Fatalf("POST / = %d, want 200", r.StatusCode)
	}
	if r := fakePost(t, srv.URL+"/v1/messages/v1/messages", fakePlainBody, nil); r.StatusCode != http.StatusNotFound {
		t.Fatalf("doubled path = %d, want 404", r.StatusCode)
	}
	var paths []string
	for _, r := range fakeCaptured(t, srv.URL) {
		paths = append(paths, r.Path)
	}
	if fmt.Sprint(paths) != "[/ /v1/messages/v1/messages]" {
		t.Fatalf("recorded paths %v", paths)
	}
}

func TestCountTokens(t *testing.T) {
	srv := fakeStart(t, "global", time.Millisecond)
	resp := fakePost(t, srv.URL+"/v1/messages/count_tokens", fakePlainBody, nil)
	var got map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil || len(got) != 1 || got["input_tokens"] != inputTokens([]byte(fakePlainBody)) {
		t.Fatalf("count_tokens = %v (%v), want only input_tokens = %d", got, err, inputTokens([]byte(fakePlainBody)))
	}
}

// Kills: an unbounded ring (a long local session grows the fake without
// limit) and a seq that restarts when the ring wraps (a probe filtering on
// "after seq N" would read old records).
func TestCapturedKeepsTheLastFifty(t *testing.T) {
	srv := fakeStart(t, "global", time.Millisecond)
	for i := 0; i < capturedMax+10; i++ {
		fakePost(t, srv.URL+"/v1/messages/count_tokens", fakePlainBody, nil)
	}
	got := fakeCaptured(t, srv.URL)
	if len(got) != capturedMax || got[0].Seq != 11 || got[len(got)-1].Seq != capturedMax+10 {
		t.Fatalf("len %d, first seq %d, last seq %d; want %d, 11, %d", len(got), got[0].Seq, got[len(got)-1].Seq, capturedMax, capturedMax+10)
	}
}

func TestParseConfig(t *testing.T) {
	c, err := parseConfig(nil)
	if err != nil || c.Addr != ":4900" || c.Label != "global" || c.PingEvery != time.Second {
		t.Fatalf("defaults %+v (%v)", c, err)
	}
	for _, args := range [][]string{{"-label", "bedrock"}, {"-label", ""}, {"-ping-every", "-1s"}, {"-nope"}} {
		if _, err := parseConfig(args); err == nil {
			t.Errorf("parseConfig(%q) accepted", args)
		}
	}
}
```

The label table refuses `bedrock` on purpose: nothing about this binary is Bedrock, and a label that said so would be read as evidence it is.

- [ ] **Step 2: Run the tests to verify they fail**

```bash
go test ./control-plane/cmd/fakeanthropic/... -count=1
```

Expected: a build failure naming the missing symbols, starting `undefined: newServer` (and `config`, `message`, `record`, `outputTokens`, `inputTokens`, `capturedMax`, `parseConfig`). Any test running at this point means a stray `main.go` exists.

- [ ] **Step 3: Write `control-plane/cmd/fakeanthropic/main.go`**

The file in five blocks, in order. First the package comment, imports, constants and flags:

```go
// Command fakeanthropic is a zero-spend upstream that speaks enough of the
// Anthropic Messages format for LiteLLM and Claude Code to talk to it. It is
// for tests and local mode (phase 6 tasks 17, 18, 23). It is not Claude and
// not Bedrock: local mode's "au" instance is this program with -label au,
// which proves routing and attribution and nothing about LiteLLM's Bedrock
// translation.
//
// The event sequence is written from training knowledge of the Messages
// streaming format and is UNVERIFIED until task 23 runs Claude Code at it.
//
// It never stores or logs a request body or a header value. /_captured holds
// the method, path, query and header NAMES, top-level body KEYS, the body's
// length and its SHA-256: enough for task 18 to see which path LiteLLM called
// and which fields survived, and not enough to become a prompt store.
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"
)

const (
	capturedMax    = 50      // records kept by /_captured
	pingsPerStream = 3       // pings between content_block_start and the delta
	maxBody        = 8 << 20 // request bodies above this are refused, unread
	outputTokens   = 3       // "fake <label> reply"
)

type config struct {
	Addr      string
	Label     string
	PingEvery time.Duration
}

func parseConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("fakeanthropic", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var c config
	fs.StringVar(&c.Addr, "addr", ":4900", "listen address")
	fs.StringVar(&c.Label, "label", "global", "route label: global or au")
	fs.DurationVar(&c.PingEvery, "ping-every", time.Second, "gap before each ping; 0 sends none")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if c.Label != "global" && c.Label != "au" {
		return config{}, fmt.Errorf("-label must be global or au, got %q", c.Label)
	}
	if c.PingEvery < 0 {
		return config{}, fmt.Errorf("-ping-every must not be negative, got %s", c.PingEvery)
	}
	return c, nil
}
```

Then the wire types and the capture ring. `record`'s JSON keys are the ones task 18's `probe.sh` reads; renaming one breaks the probe, not this file's tests.

```go
type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type textBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type message struct {
	ID           string      `json:"id"`
	Type         string      `json:"type"`
	Role         string      `json:"role"`
	Model        string      `json:"model"`
	Content      []textBlock `json:"content"`
	StopReason   *string     `json:"stop_reason"`
	StopSequence *string     `json:"stop_sequence"`
	Usage        usage       `json:"usage"`
}

// record is one request as /_captured reports it: names, keys, a length and
// a hash. There is deliberately no field that can hold a value.
type record struct {
	Seq         int       `json:"seq"`
	At          time.Time `json:"at"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	QueryKeys   []string  `json:"queryKeys"`
	HeaderNames []string  `json:"headerNames"`
	BodyKeys    []string  `json:"bodyKeys"`
	BodyBytes   int       `json:"bodyBytes"`
	BodySHA256  string    `json:"bodySha256"`
}

type capture struct {
	mu   sync.Mutex
	seq  int
	ring []record
}

func (c *capture) add(r record) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	r.Seq = c.seq
	c.ring = append(c.ring, r)
	if len(c.ring) > capturedMax {
		c.ring = slices.Clone(c.ring[len(c.ring)-capturedMax:])
	}
}

func (c *capture) list() []record {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.ring)
}

type server struct {
	cfg config
	cap *capture
}

func newServer(c config) *server { return &server{cfg: c, cap: &capture{}} }

// Handler routes on the path only; the query (?beta=true) is recorded by name
// and otherwise ignored. POST / is a Messages endpoint on purpose: it is what
// a bare-host api_base posted verbatim arrives as (task 18, P07).
func (s *server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/messages", s.messages)
	mux.HandleFunc("POST /{$}", s.messages)
	mux.HandleFunc("POST /v1/messages/count_tokens", s.countTokens)
	mux.HandleFunc("GET /_captured", s.captured)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found_error", "fakeanthropic serves POST /v1/messages, POST /, POST /v1/messages/count_tokens and GET /_captured")
	})
	return s.record(mux)
}
```

`slices.Clone` on the trimmed ring is what keeps it bounded: re-slicing alone would keep the old backing array reachable and let `append` grow it for the life of the process.

Then the recorder and the two JSON endpoints. The recorder reads the body once, bounded, records it by hash, and hands the handler a fresh reader over the same bytes. It records **before** routing, so a `404` is recorded too.

```go
func (s *server) record(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Fake-Route", s.cfg.Label)
		if r.URL.Path == "/_captured" {
			next.ServeHTTP(w, r)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
		if err != nil || len(body) > maxBody {
			writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body unreadable or over 8 MiB")
			return
		}
		sum := sha256.Sum256(body)
		s.cap.add(record{
			At:          time.Now().UTC(),
			Method:      r.Method,
			Path:        r.URL.Path,
			QueryKeys:   slices.Sorted(maps.Keys(r.URL.Query())),
			HeaderNames: slices.Sorted(maps.Keys(r.Header)),
			BodyKeys:    bodyKeys(body),
			BodyBytes:   len(body),
			BodySHA256:  hex.EncodeToString(sum[:]),
		})
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

// bodyKeys is the sorted top-level field names of a JSON object, or nil.
// Only names: a value never leaves this function.
func bodyKeys(body []byte) []string {
	var top map[string]json.RawMessage
	if json.Unmarshal(body, &top) != nil {
		return nil
	}
	return slices.Sorted(maps.Keys(top))
}

func inputTokens(body []byte) int { return max(1, len(body)/4) }

func (s *server) countTokens(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	writeJSON(w, http.StatusOK, map[string]int{"input_tokens": inputTokens(body)})
}

func (s *server) captured(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string][]record{"requests": s.cap.list()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError is Anthropic's error shape. The message is always a fixed
// string: nothing from the request is ever echoed.
func writeError(w http.ResponseWriter, status int, typ, msg string) {
	writeJSON(w, status, map[string]any{"type": "error", "error": map[string]string{"type": typ, "message": msg}})
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "msg_fake_" + hex.EncodeToString(b)
}
```

`max` is the Go 1.21 built-in, inside the module's `go 1.23` directive. `maps.Keys` returning an iterator, and `slices.Sorted` over it, are Go 1.23; `r.Header` and `r.URL.Query()` are both `map[string][]string` underneath, which is what `maps.Keys`'s `~map[K]V` constraint accepts.

Then the Messages handler and the stream. Every event is written and flushed on its own; the pings wait on the request's context, so a client that hangs up ends the handler instead of leaving it asleep.

```go
type event struct {
	name string
	data map[string]any
}

func (s *server) messages(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	if json.Unmarshal(body, &req) != nil || req.Model == "" {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "body must be a JSON object with a model")
		return
	}
	text, in, end := "fake "+s.cfg.Label+" reply", inputTokens(body), "end_turn"
	if !req.Stream {
		writeJSON(w, http.StatusOK, message{
			ID: newID(), Type: "message", Role: "assistant", Model: req.Model,
			Content:    []textBlock{{Type: "text", Text: text}},
			StopReason: &end, Usage: usage{InputTokens: in, OutputTokens: outputTokens},
		})
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "api_error", "streaming unsupported by this server")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	send := func(e event) bool {
		b, err := json.Marshal(e.data)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.name, b); err != nil {
			return false
		}
		fl.Flush()
		return true
	}
	start := message{ID: newID(), Type: "message", Role: "assistant", Model: req.Model,
		Content: []textBlock{}, Usage: usage{InputTokens: in, OutputTokens: 1}}
	head := []event{
		{"message_start", map[string]any{"type": "message_start", "message": start}},
		{"content_block_start", map[string]any{"type": "content_block_start", "index": 0,
			"content_block": textBlock{Type: "text", Text: ""}}},
	}
	tail := []event{
		{"content_block_delta", map[string]any{"type": "content_block_delta", "index": 0,
			"delta": map[string]any{"type": "text_delta", "text": text}}},
		{"content_block_stop", map[string]any{"type": "content_block_stop", "index": 0}},
		{"message_delta", map[string]any{"type": "message_delta",
			"delta": map[string]any{"stop_reason": end, "stop_sequence": nil},
			"usage": map[string]any{"output_tokens": outputTokens}}},
		{"message_stop", map[string]any{"type": "message_stop"}},
	}
	for _, e := range head {
		if !send(e) {
			return
		}
	}
	for i := 0; s.cfg.PingEvery > 0 && i < pingsPerStream; i++ {
		t := time.NewTimer(s.cfg.PingEvery)
		select {
		case <-r.Context().Done():
			t.Stop()
			return
		case <-t.C:
		}
		if !send(event{"ping", map[string]any{"type": "ping"}}) {
			return
		}
	}
	for _, e := range tail {
		if !send(e) {
			return
		}
	}
}
```

`json.Marshal` of a `map[string]any` sorts keys, which is why the wire block above lists `"message"` before `"type"`. Nothing that parses JSON depends on key order.

Last, `main`. One log line at start and nothing per request. No `WriteTimeout`: a stream with `-ping-every 2m` must be allowed to run past any fixed deadline, which is the property task 18's P09 relies on.

```go
func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "fakeanthropic:", err)
		os.Exit(2)
	}
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           newServer(cfg).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Info("fakeanthropic listening", "addr", cfg.Addr, "label", cfg.Label, "pingEvery", cfg.PingEvery.String())
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("fakeanthropic stopped", "err", err.Error())
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
go test ./control-plane/cmd/fakeanthropic/... -race -count=1 -v
```

Expected: nine `--- PASS` lines (`TestStreamEventOrderGolden`, `TestPingsArriveAtTheGapNotAtTheEnd`, `TestZeroGapSendsNoPing`, `TestLabelReachesBodyAndHeader`, `TestCapturedHoldsNoBodyAndNoHeaderValue`, `TestRootIsAnsweredAndEveryPathIsRecorded`, `TestCountTokens`, `TestCapturedKeepsTheLastFifty`, `TestParseConfig`) and `ok  github.com/kpmg-au-emu/panacea-app/control-plane/cmd/fakeanthropic`. The ping test takes about half a second. `-race` needs cgo; on a Windows machine without a C compiler, drop the flag and say so in the commit.

**Prove the ping test is not vacuous.** Comment out `fl.Flush()` in `send` and re-run `TestPingsArriveAtTheGapNotAtTheEnd`: it must fail with `first ping arrived <a few µs> after content_block_start`, a figure far under 120 ms (Go's response writer holds small writes until a flush or the handler's return, and the whole stream is under 1 KiB). Restore the line. If the test still passes with the flush removed, the client is not reading incrementally and the test proves nothing.

- [ ] **Step 5: Run the binary by hand**

Build, then run the built file: a `go run` in the background leaves its child running when the shell is stopped.

```bash
go build -o /tmp/fakeanthropic ./control-plane/cmd/fakeanthropic
/tmp/fakeanthropic -addr 127.0.0.1:4900 -label au -ping-every 1s &
curl -sN -X POST 'http://127.0.0.1:4900/v1/messages?beta=true' \
  -H 'content-type: application/json' \
  -d '{"model":"claude-opus-4-8","max_tokens":16,"stream":true,"messages":[{"role":"user","content":"hi"}]}'
curl -s http://127.0.0.1:4900/_captured
kill %1
```

Expected: the first two events at once, three `event: ping` blocks a second apart, then `content_block_delta` carrying `"text":"fake au reply"` and the rest together. `/_captured` shows one record with `"path":"/v1/messages"`, `"queryKeys":["beta"]` and no `hi`. On Windows, use `%TEMP%`'s path in place of `/tmp`, and add `--ssl-no-revoke` only if `curl` is pointed at an HTTPS URL, which it is not here.

- [ ] **Step 6: Vet, format, and the whole workspace**

```bash
gofmt -l control-plane/cmd/fakeanthropic
go vet ./control-plane/cmd/fakeanthropic/...
go test ./sandboxd/... ./control-plane/... -count=1
```

`gofmt -l` and `go vet` print nothing. The third line is the workspace's full-test form (phase 2's rule: `go test ./...` at the `go.work` root fails with `directory prefix . does not contain modules listed in go.work`).

- [ ] **Step 7: Commit**

```bash
git add control-plane/cmd/fakeanthropic/main.go control-plane/cmd/fakeanthropic/main_test.go
git commit -F /tmp/msg-task16.txt -- control-plane/cmd/fakeanthropic/main.go control-plane/cmd/fakeanthropic/main_test.go
```

with `/tmp/msg-task16.txt` holding:

```text
feat(gateway): fakeanthropic, a zero-spend Messages upstream

Local mode, the LiteLLM probe and the end-to-end proof need an upstream that
answers POST /v1/messages and streams like Anthropic, and costs nothing. Two
copies run locally, labelled global and au; the au copy is Anthropic-format
too, so it proves routing and attribution and nothing about Bedrock.

It answers POST / as well and records every request, including the ones it
refuses, so task 18 can see which path LiteLLM called for a bare-host and a
full-path api_base: the Panacea 405, measured rather than remembered.

Pings are flushed one at a time at -ping-every, and a test proves they arrive
spaced, so a buffering finding in task 18 is LiteLLM's and not the fake's.

/_captured holds names, keys, a length and a SHA-256. No body, no header
value, no per-request log line; a canary test holds it to that.

The event sequence is from training knowledge, UNVERIFIED until task 23 runs
Claude Code against it.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
```

**What this task does NOT claim:**

- **That the stream is exactly Anthropic's.** The event names, their order and the payload fields are from training knowledge. Task 23 observes whether unmodified Claude Code accepts them; the real API may send fields (for example cache-token counts in `usage`) this fake omits.
- **Anything about Bedrock.** The `au` label is a name. LiteLLM's Anthropic-to-Bedrock translation, `drop_params` against a real Bedrock model and the `au.` inference profiles are task 24's P04.
- **That LiteLLM prices these calls correctly.** It echoes the model and returns fixed `usage`; whether LiteLLM's price table knows the id, and whether it prices by the request's or the reply's model, is task 18's P12.
- **Faithful error behaviour.** Anthropic's `429`, `529 overloaded_error` and mid-stream `error` events are not emulated. A test of Claude Code's retry behaviour needs another tool.
- **`count_tokens` accuracy.** `input_tokens` is a quarter of the body's byte length, a stable number and not a tokenizer.
- **Authentication.** The fake accepts any credential or none; it is reachable only on the compose network (task 17) and in `httptest`.
