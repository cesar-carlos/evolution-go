package nats_producer

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

type publication struct{ subject, payload string }

// protocolPeer exercises the real nats.go connection over loopback without an
// external broker. Only CONNECT, PUB and PING/PONG are needed by the producer.
func protocolPeer(t *testing.T, stallFlush bool) (string, <-chan publication) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	messages := make(chan publication, 100)
	var mu sync.Mutex
	var workers sync.WaitGroup
	connections := map[net.Conn]bool{}
	stopped := false
	workers.Add(1)
	go func() {
		defer workers.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			if stopped {
				mu.Unlock()
				conn.Close()
				return
			}
			connections[conn] = true
			workers.Add(1)
			mu.Unlock()
			go func() {
				defer workers.Done()
				defer conn.Close()
				defer func() { mu.Lock(); delete(connections, conn); mu.Unlock() }()
				if _, err := fmt.Fprint(conn, "INFO {\"server_id\":\"test\",\"max_payload\":1048576}\r\n"); err != nil {
					return
				}
				reader := bufio.NewReader(conn)
				pings := 0
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					fields := strings.Fields(line)
					if len(fields) == 0 {
						continue
					}
					switch fields[0] {
					case "PING":
						pings++
						if !stallFlush || pings == 1 {
							if _, err := fmt.Fprint(conn, "PONG\r\n"); err != nil {
								return
							}
						}
					case "PUB":
						if len(fields) < 3 {
							return
						}
						size, err := strconv.Atoi(fields[len(fields)-1])
						if err != nil || size < 0 || size > 1048576 {
							return
						}
						data := make([]byte, size+2)
						if _, err := io.ReadFull(reader, data); err != nil {
							return
						}
						select {
						case messages <- publication{fields[1], string(data[:size])}:
						default:
							return
						}
					}
				}
			}()
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		mu.Lock()
		stopped = true
		conns := make([]net.Conn, 0, len(connections))
		for c := range connections {
			conns = append(conns, c)
		}
		mu.Unlock()
		for _, c := range conns {
			c.Close()
		}
		workers.Wait()
	})
	return "nats://" + listener.Addr().String(), messages
}

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for test peer")
	}
	var zero T
	return zero
}

func TestProducerModesAndPayload(t *testing.T) {
	url, received := protocolPeer(t, false)
	p, err := newNatsProducer(url, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	payload := `{"event":"ButtonClick","data":{"buttonId":"selection"}}`
	for _, mode := range []string{"enabled", "true", "global"} {
		if err := p.Produce("instance.buttonclick", []byte(payload), mode, "instance"); err != nil {
			t.Fatal(err)
		}
		if got := receive(t, received); got != (publication{"instance.buttonclick", payload}) {
			t.Fatalf("payload/subject changed: %+v", got)
		}
	}
	for _, mode := range []string{"", "false", "disabled", "unknown"} {
		if err := p.Produce("ignored", nil, mode, ""); err != nil {
			t.Fatal(err)
		}
	}
	p.natsGlobalEnabled = false
	if err := p.Produce("ignored", nil, "global", ""); err != nil {
		t.Fatal(err)
	}
	if err := p.conn.FlushTimeout(time.Second); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-received:
		t.Fatalf("disabled publication: %+v", got)
	default:
	}
	if err := p.Produce("", nil, "enabled", ""); !errors.Is(err, nats.ErrBadSubject) {
		t.Fatalf("publish error not propagated: %v", err)
	}
	p.Close()
	p.Close()
	if err := p.Produce("buttonclick", nil, "true", ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed connection reported success: %v", err)
	}
}

type dialFunc func(string, string) (net.Conn, error)

func (f dialFunc) Dial(network, address string) (net.Conn, error) { return f(network, address) }

func TestEmptyURLDoesNotDial(t *testing.T) {
	p, err := newNatsProducer("", true, nats.SetCustomDialer(dialFunc(func(string, string) (net.Conn, error) {
		t.Error("empty URL attempted connection")
		return nil, errors.New("unexpected dial")
	})))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	if err = p.Produce("buttonclick", nil, "global", ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing connection reported success: %v", err)
	}
}

func TestInitialFailureRecoversWithoutReplaying(t *testing.T) {
	url, received := protocolPeer(t, false)
	var calls atomic.Int32
	ready := make(chan struct{}, 1)
	gate := make(chan struct{})
	var release sync.Once
	t.Cleanup(func() { release.Do(func() { close(gate) }) })
	notify := func(*nats.Conn) {
		select {
		case ready <- struct{}{}:
		default:
		}
	}
	p, err := newNatsProducer(url, true, nats.ReconnectWait(0), nats.ReconnectJitter(0, 0), nats.ConnectHandler(notify), nats.ReconnectHandler(notify), nats.SetCustomDialer(dialFunc(func(network, address string) (net.Conn, error) {
		if calls.Add(1) == 1 {
			return nil, errors.New("initial outage")
		}
		<-gate
		return net.DialTimeout(network, address, time.Second)
	})))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Release the test dialer even when an assertion fails before recovery.
		release.Do(func() { close(gate) })
		p.Close()
	})
	if err = p.Produce("buttonclick", []byte("must-not-replay"), "global", ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("outage reported success: %v", err)
	}
	// Also cover a disconnect racing the producer's IsConnected check: the
	// underlying client must refuse buffering while reconnecting.
	if err = p.conn.Publish("buttonclick", []byte("racing-disconnect")); !errors.Is(err, nats.ErrReconnectBufExceeded) {
		t.Fatalf("reconnect buffer was not disabled: %v", err)
	}
	release.Do(func() { close(gate) })
	receive(t, ready)
	if err = p.Produce("buttonclick", []byte("recovered"), "global", ""); err != nil {
		t.Fatal(err)
	}
	if got := receive(t, received); got.payload != "recovered" {
		t.Fatalf("unexpected replay: %+v", got)
	}
	select {
	case got := <-received:
		t.Fatalf("duplicate publication: %+v", got)
	default:
	}
}

func TestReconnectAttemptsAreBounded(t *testing.T) {
	var attempts atomic.Int32
	closed := make(chan struct{}, 1)
	p, err := newNatsProducer("nats://127.0.0.1:4222", true, nats.MaxReconnects(1), nats.ReconnectWait(0), nats.ReconnectJitter(0, 0), nats.ClosedHandler(func(*nats.Conn) { closed <- struct{}{} }), nats.SetCustomDialer(dialFunc(func(string, string) (net.Conn, error) { attempts.Add(1); return nil, errors.New("offline") })))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	receive(t, closed)
	if attempts.Load() != 2 {
		t.Fatalf("expected initial dial plus one retry, got %d", attempts.Load())
	}
	if err = p.Produce("buttonclick", nil, "enabled", ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("exhausted connection reported success: %v", err)
	}
}

func TestFlushFailureIsUncertainAndNotRetried(t *testing.T) {
	url, received := protocolPeer(t, true)
	p, err := newNatsProducer(url, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	if err = p.Produce("buttonclick", []byte("one"), "enabled", ""); !errors.Is(err, nats.ErrTimeout) {
		t.Fatalf("flush failure lost: %v", err)
	}
	if got := receive(t, received); got.payload != "one" {
		t.Fatal(got)
	}
	select {
	case got := <-received:
		t.Fatalf("automatic replay: %+v", got)
	default:
	}
}

func TestConcurrentPublishAndClose(t *testing.T) {
	url, _ := protocolPeer(t, false)
	p, err := newNatsProducer(url, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < 12; i++ {
		workers.Add(1)
		go func() { defer workers.Done(); <-start; _ = p.Produce("buttonclick", []byte("race"), "enabled", "") }()
	}
	workers.Add(1)
	go func() { defer workers.Done(); <-start; p.Close() }()
	close(start)
	workers.Wait()
	if err = p.Produce("buttonclick", nil, "enabled", ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("closed producer accepted work: %v", err)
	}
}

func TestCloseStopsPendingReconnect(t *testing.T) {
	delayEntered := make(chan struct{}, 1)
	closed := make(chan struct{}, 1)
	p, err := newNatsProducer("nats://127.0.0.1:4222", true,
		nats.CustomReconnectDelay(func(int) time.Duration {
			select {
			case delayEntered <- struct{}{}:
			default:
			}
			return time.Hour
		}),
		nats.ClosedHandler(func(*nats.Conn) { closed <- struct{}{} }),
		nats.SetCustomDialer(dialFunc(func(string, string) (net.Conn, error) { return nil, errors.New("offline") })),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	receive(t, delayEntered)
	p.Close()
	receive(t, closed)
	if !p.conn.IsClosed() {
		t.Fatal("Close left reconnect active")
	}
}
