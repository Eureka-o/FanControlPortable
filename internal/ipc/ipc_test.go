package ipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDisconnectFailsPendingRequestImmediately(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := NewClient(nil)
	client.conn = clientConn
	client.connected = true
	client.generation = 1
	go client.readLoop(clientConn, bufio.NewReader(clientConn), 1)

	go func() {
		reader := bufio.NewReader(serverConn)
		_, _ = reader.ReadBytes('\n')
		_ = serverConn.Close()
	}()

	started := time.Now()
	_, err := client.SendRequestWithTimeout(ReqPing, nil, 5*time.Second)
	if err == nil {
		t.Fatal("request succeeded after connection closed")
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("disconnect took %v to fail pending request", elapsed)
	}
}

func TestSendRequestReportsActualGeneration(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := NewClient(nil)
	client.conn = clientConn
	client.connected = true
	client.generation = 7
	go client.readLoop(clientConn, bufio.NewReader(clientConn), 7)

	go func() {
		defer serverConn.Close()
		line, err := bufio.NewReader(serverConn).ReadBytes('\n')
		if err != nil {
			return
		}
		var request Request
		if json.Unmarshal(line, &request) != nil {
			return
		}
		payload, err := json.Marshal(Response{RequestID: request.RequestID, IsResponse: true, Success: true})
		if err == nil {
			_, _ = serverConn.Write(append(payload, '\n'))
		}
	}()

	response, generation, err := client.SendRequestWithTimeoutGeneration(ReqPing, nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if response == nil || !response.Success {
		t.Fatalf("unexpected response: %+v", response)
	}
	if generation != 7 {
		t.Fatalf("request generation = %d, want 7", generation)
	}
	client.Close()
}

func TestStaleReadLoopCannotDisconnectReplacement(t *testing.T) {
	oldClient, oldServer := net.Pipe()
	newClient, newServer := net.Pipe()
	defer newServer.Close()

	client := NewClient(nil)
	client.conn = newClient
	client.connected = true
	client.generation = 2

	done := make(chan struct{})
	go func() {
		client.readLoop(oldClient, bufio.NewReader(oldClient), 1)
		close(done)
	}()
	_ = oldServer.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stale read loop did not exit")
	}

	if !client.IsConnected() {
		t.Fatal("stale read loop marked replacement connection disconnected")
	}
	client.Close()
}

func TestCloseDoesNotFailReplacementGenerationPending(t *testing.T) {
	oldClient, oldServer := net.Pipe()
	defer oldServer.Close()
	newClient, newServer := net.Pipe()
	defer newClient.Close()
	defer newServer.Close()

	client := NewClient(nil)
	client.conn = oldClient
	client.connected = true
	client.generation = 1

	// Hold pendingMutex so Close has already retired generation 1 before it can
	// fail requests. This makes the reconnect window deterministic.
	client.pendingMutex.Lock()
	closeDone := make(chan struct{})
	go func() {
		client.Close()
		close(closeDone)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		client.connMutex.RLock()
		retired := !client.connected && client.generation == 2
		client.connMutex.RUnlock()
		if retired || time.Now().After(deadline) {
			if !retired {
				client.pendingMutex.Unlock()
				t.Fatal("Close did not retire the old connection")
			}
			break
		}
		time.Sleep(time.Millisecond)
	}

	client.connMutex.Lock()
	client.conn = newClient
	client.connected = true
	client.generation = 3
	client.connMutex.Unlock()
	replacementPending := make(chan requestResult, 1)
	client.pending["replacement"] = pendingRequest{generation: 3, result: replacementPending}
	client.pendingMutex.Unlock()

	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("Close did not finish")
	}
	select {
	case result := <-replacementPending:
		t.Fatalf("old Close failed replacement request: %v", result.err)
	default:
	}
}

func TestLateCloseGenerationDoesNotCloseReplacement(t *testing.T) {
	newClient, newServer := net.Pipe()
	defer newServer.Close()

	client := NewClient(nil)
	client.conn = newClient
	client.connected = true
	client.generation = 2
	replacementPending := make(chan requestResult, 1)
	client.pending["replacement"] = pendingRequest{generation: 2, result: replacementPending}

	if client.CloseGeneration(1) {
		t.Fatal("late generation close retired the replacement connection")
	}
	if !client.IsConnected() || client.ConnectionGeneration() != 2 {
		t.Fatal("replacement connection was changed by a late generation close")
	}
	select {
	case result := <-replacementPending:
		t.Fatalf("late generation close failed replacement request: %v", result.err)
	default:
	}
	client.Close()
}

func TestConcurrentLateCloseGenerationDoesNotCloseReplacement(t *testing.T) {
	newClient, newServer := net.Pipe()
	defer newServer.Close()

	client := NewClient(nil)
	client.conn = newClient
	client.connected = true
	client.generation = 2
	replacementPending := make(chan requestResult, 1)
	client.pending["replacement"] = pendingRequest{generation: 2, result: replacementPending}

	start := make(chan struct{})
	results := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			results <- client.CloseGeneration(1)
		}()
	}
	close(start)
	for i := 0; i < 2; i++ {
		if <-results {
			t.Fatal("late generation close retired the replacement connection")
		}
	}
	if !client.IsConnected() || client.ConnectionGeneration() != 2 {
		t.Fatal("replacement connection was changed by concurrent late closes")
	}
	select {
	case result := <-replacementPending:
		t.Fatalf("concurrent late closes failed replacement request: %v", result.err)
	default:
	}
	client.Close()
}

func TestStaleReadLoopDropsEvents(t *testing.T) {
	oldClient, oldServer := net.Pipe()
	defer oldClient.Close()
	defer oldServer.Close()
	newClient, newServer := net.Pipe()
	defer newServer.Close()

	client := NewClient(nil)
	client.conn = newClient
	client.connected = true
	client.generation = 2
	events := make(chan Event, 1)
	client.SetEventHandler(func(event Event) { events <- event })

	done := make(chan struct{})
	go func() {
		client.readLoop(oldClient, bufio.NewReader(oldClient), 1)
		close(done)
	}()
	payload, err := json.Marshal(Event{IsEvent: true, Type: "stale"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := oldServer.Write(append(payload, '\n')); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stale read loop did not exit after receiving an event")
	}
	select {
	case event := <-events:
		t.Fatalf("stale read loop delivered event %q", event.Type)
	default:
	}
	client.Close()
}

func TestEventsRemainOrderedAcrossGenerations(t *testing.T) {
	oldClient, oldServer := net.Pipe()
	defer oldClient.Close()
	defer oldServer.Close()
	newClient, newServer := net.Pipe()
	defer newServer.Close()

	client := NewClient(nil)
	client.conn = oldClient
	client.connected = true
	client.generation = 1

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	handled := make(chan string, 2)
	client.SetEventHandler(func(event Event) {
		if event.Type == "old" {
			close(firstStarted)
			<-releaseFirst
		}
		handled <- event.Type
	})
	go client.readLoop(oldClient, bufio.NewReader(oldClient), 1)

	oldPayload, err := json.Marshal(Event{IsEvent: true, Type: "old"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := oldServer.Write(append(oldPayload, '\n')); err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("old-generation handler did not start")
	}

	client.connMutex.Lock()
	client.conn = newClient
	client.connected = true
	client.generation = 2
	client.connMutex.Unlock()
	go client.readLoop(newClient, bufio.NewReader(newClient), 2)

	newPayload, err := json.Marshal(Event{IsEvent: true, Type: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newServer.Write(append(newPayload, '\n')); err != nil {
		t.Fatal(err)
	}
	select {
	case eventType := <-handled:
		t.Fatalf("new-generation event overtook active old event: %s", eventType)
	default:
	}

	close(releaseFirst)
	for _, want := range []string{"old", "new"} {
		select {
		case got := <-handled:
			if got != want {
				t.Fatalf("event order = %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %q event", want)
		}
	}
	client.Close()
}

func TestEventHandlerMayCloseClient(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	client := NewClient(nil)
	client.conn = clientConn
	client.connected = true
	client.generation = 1

	handled := make(chan struct{})
	client.SetEventHandler(func(Event) {
		client.Close()
		close(handled)
	})
	go client.readLoop(clientConn, bufio.NewReader(clientConn), 1)
	payload, err := json.Marshal(Event{IsEvent: true, Type: "close"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := serverConn.Write(append(payload, '\n')); err != nil {
		t.Fatal(err)
	}
	select {
	case <-handled:
	case <-time.After(time.Second):
		t.Fatal("event handler deadlocked while closing its client")
	}
	if client.IsConnected() {
		t.Fatal("event handler did not close the client")
	}
}

func TestStopDoesNotHoldServerMutexWhileClientCloseWaits(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	server := NewServer(nil, nil)
	state := &clientState{
		conn:    clientConn,
		writeCh: make(chan []byte, clientWriteQueueSize),
		respCh:  make(chan []byte, clientResponseQueueSize),
		closed:  make(chan struct{}),
	}
	server.clients[clientConn] = state
	server.running.Store(true)

	closeStarted := make(chan struct{})
	releaseClose := make(chan struct{})
	closeDone := make(chan struct{})
	go func() {
		state.closeOnce.Do(func() {
			close(closeStarted)
			<-releaseClose
		})
		close(closeDone)
	}()
	<-closeStarted

	stopDone := make(chan struct{})
	go func() {
		server.Stop()
		close(stopDone)
	}()
	deadline := time.Now().Add(time.Second)
	for server.running.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	clientsCleared := make(chan struct{})
	go func() {
		for {
			server.mutex.Lock()
			empty := len(server.clients) == 0
			server.mutex.Unlock()
			if empty {
				close(clientsCleared)
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	var blocked bool
	select {
	case <-clientsCleared:
	case <-time.After(250 * time.Millisecond):
		blocked = true
	}
	close(releaseClose)
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("in-progress client close did not finish")
	}
	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("server stop did not finish")
	}
	if blocked {
		t.Fatal("server Stop held server mutex while waiting for client close")
	}
}

func TestNoiseDiagnosticCancelIsHandledWhileTargetWaits(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	targetStarted := make(chan struct{})
	releaseTarget := make(chan struct{})
	defer func() {
		select {
		case <-releaseTarget:
		default:
			close(releaseTarget)
		}
	}()
	cancelHandled := make(chan struct{})
	server := NewServer(func(req Request) Response {
		switch req.Type {
		case ReqSetNoiseDiagnosticTarget:
			close(targetStarted)
			<-releaseTarget
		case ReqCancelNoiseDiagnostic:
			close(cancelHandled)
		}
		return Response{Success: true}
	}, nil)
	server.running.Store(true)
	state := &clientState{
		conn:    serverConn,
		writeCh: make(chan []byte, clientWriteQueueSize),
		respCh:  make(chan []byte, clientResponseQueueSize),
		sem:     make(chan struct{}, maxConcurrentRequestsPerClient),
		closed:  make(chan struct{}),
	}
	go server.clientWriter(state)
	go server.handleClient(serverConn, state)

	writeRequest := func(req Request) error {
		payload, err := json.Marshal(req)
		if err != nil {
			return err
		}
		if _, err := clientConn.Write(append(payload, '\n')); err != nil {
			return err
		}
		return nil
	}
	if err := writeRequest(Request{RequestID: "target", Type: ReqSetNoiseDiagnosticTarget}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-targetStarted:
	case <-time.After(time.Second):
		t.Fatal("target request did not start")
	}
	writeDone := make(chan error, 1)
	go func() {
		writeDone <- writeRequest(Request{RequestID: "cancel", Type: ReqCancelNoiseDiagnostic})
	}()
	select {
	case <-cancelHandled:
	case <-time.After(time.Second):
		t.Fatal("cancel request waited behind target settling")
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}

	reader := bufio.NewReader(clientConn)
	readResponseID := func() string {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			t.Fatal(err)
		}
		var response Response
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatal(err)
		}
		return response.RequestID
	}
	if got := readResponseID(); got != "cancel" {
		t.Fatalf("first response = %q; want cancel", got)
	}
	close(releaseTarget)
	if got := readResponseID(); got != "target" {
		t.Fatalf("second response = %q; want target", got)
	}
	server.running.Store(false)
}

func TestSlowRequestDoesNotBlockOthers(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	slowStarted := make(chan struct{})
	releaseSlow := make(chan struct{})
	defer close(releaseSlow)
	server := NewServer(func(req Request) Response {
		if req.Type == ReqConnect {
			close(slowStarted)
			<-releaseSlow
		}
		return Response{Success: true}
	}, nil)
	server.running.Store(true)
	state := &clientState{
		conn:    serverConn,
		writeCh: make(chan []byte, clientWriteQueueSize),
		respCh:  make(chan []byte, clientResponseQueueSize),
		sem:     make(chan struct{}, maxConcurrentRequestsPerClient),
		closed:  make(chan struct{}),
	}
	go server.clientWriter(state)
	go server.handleClient(serverConn, state)

	writeRequest := func(req Request) error {
		payload, err := json.Marshal(req)
		if err != nil {
			return err
		}
		_, err = clientConn.Write(append(payload, '\n'))
		return err
	}
	if err := writeRequest(Request{RequestID: "slow", Type: ReqConnect}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-slowStarted:
	case <-time.After(time.Second):
		t.Fatal("slow request did not start")
	}
	writeDone := make(chan error, 1)
	go func() {
		writeDone <- writeRequest(Request{RequestID: "ping", Type: ReqPing})
	}()

	responseID := make(chan string, 1)
	go func() {
		line, err := bufio.NewReader(clientConn).ReadBytes('\n')
		if err != nil {
			responseID <- ""
			return
		}
		var response Response
		if json.Unmarshal(line, &response) != nil {
			responseID <- ""
			return
		}
		responseID <- response.RequestID
	}()
	select {
	case got := <-responseID:
		if got != "ping" {
			t.Fatalf("first response = %q; want ping", got)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("ping request waited behind slow request")
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
}

func TestHandlerPanicReturnsErrorAndConnectionRemainsUsable(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	server := NewServer(func(req Request) Response {
		if req.Type == ReqConnect {
			panic("boom")
		}
		return Response{Success: true}
	}, nil)
	server.running.Store(true)
	state := &clientState{
		conn:    serverConn,
		writeCh: make(chan []byte, clientWriteQueueSize),
		respCh:  make(chan []byte, clientResponseQueueSize),
		sem:     make(chan struct{}, maxConcurrentRequestsPerClient),
		closed:  make(chan struct{}),
	}
	go server.clientWriter(state)
	go server.handleClient(serverConn, state)

	writeRequest := func(req Request) {
		payload, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := clientConn.Write(append(payload, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	reader := bufio.NewReader(clientConn)
	readResponse := func() Response {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			t.Fatal(err)
		}
		var response Response
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatal(err)
		}
		return response
	}

	writeRequest(Request{RequestID: "panic", Type: ReqConnect})
	panicResponse := readResponse()
	if panicResponse.Success || panicResponse.ErrorCode != "internal_panic" {
		t.Fatalf("panic response = %+v", panicResponse)
	}
	writeRequest(Request{RequestID: "ping", Type: ReqPing})
	pingResponse := readResponse()
	if !pingResponse.Success || pingResponse.RequestID != "ping" {
		t.Fatalf("ping response after panic = %+v", pingResponse)
	}
}

func TestServerWriterClosesStalledClientAfterTimeout(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	server := NewServer(nil, nil)
	server.writeTimeout = 25 * time.Millisecond
	state := &clientState{
		conn:    serverConn,
		writeCh: make(chan []byte, clientWriteQueueSize),
		respCh:  make(chan []byte, clientResponseQueueSize),
		closed:  make(chan struct{}),
	}
	server.clients[serverConn] = state
	go server.clientWriter(state)
	state.writeCh <- []byte("blocked\n")

	select {
	case <-state.closed:
	case <-time.After(time.Second):
		t.Fatal("stalled client was not closed after write timeout")
	}
	if server.HasClients() {
		t.Fatal("stalled client remained registered")
	}
}

func TestServerWriterPrioritizesResponseOverQueuedEvents(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	server := NewServer(nil, nil)
	state := &clientState{
		conn:    serverConn,
		writeCh: make(chan []byte, clientWriteQueueSize),
		respCh:  make(chan []byte, clientResponseQueueSize),
		closed:  make(chan struct{}),
	}
	state.writeCh <- []byte("event\n")
	state.respCh <- []byte("response\n")
	go server.clientWriter(state)

	line, err := bufio.NewReader(clientConn).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if line != "response\n" {
		t.Fatalf("first write = %q; want response", line)
	}
	server.closeClient(state)
}

func TestCriticalEventWaitsBrieflyForQueueSpace(t *testing.T) {
	server := NewServer(nil, nil)
	state := &clientState{
		writeCh: make(chan []byte, 1),
		closed:  make(chan struct{}),
	}
	state.writeCh <- []byte("queued\n")
	server.clients[nil] = state

	done := make(chan struct{})
	go func() {
		server.BroadcastEvent(EventConfigUpdate, true)
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	<-state.writeCh

	select {
	case payload := <-state.writeCh:
		var event Event
		if err := json.Unmarshal(payload, &event); err != nil {
			t.Fatal(err)
		}
		if event.Type != EventConfigUpdate {
			t.Fatalf("event type = %q; want %q", event.Type, EventConfigUpdate)
		}
	case <-time.After(time.Second):
		t.Fatal("critical event was dropped while queue space became available")
	}
	<-done
}

func TestClientWriteTimeoutRetiresConnectionGeneration(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	client := NewClient(nil)
	client.conn = clientConn
	client.connected = true
	client.generation = 4

	type sendResult struct {
		generation uint64
		err        error
	}
	done := make(chan sendResult, 1)
	go func() {
		_, generation, err := client.SendRequestWithTimeoutGeneration(ReqPing, nil, 25*time.Millisecond)
		done <- sendResult{generation: generation, err: err}
	}()

	select {
	case result := <-done:
		if !errors.Is(result.err, ErrRequestTimeout) {
			t.Fatalf("write error = %v; want ErrRequestTimeout", result.err)
		}
		if result.generation != 4 {
			t.Fatalf("request generation = %d; want 4", result.generation)
		}
		if client.IsConnected() || client.ConnectionGeneration() != 5 {
			t.Fatal("timed-out connection generation was not retired")
		}
	case <-time.After(250 * time.Millisecond):
		_ = serverConn.Close()
		result := <-done
		t.Fatalf("client write ignored request timeout: %v", result.err)
	}
}

func TestResponseTimeoutReturnsRequestTimeoutError(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	client := NewClient(nil)
	client.conn = clientConn
	client.connected = true
	client.generation = 1
	go client.readLoop(clientConn, bufio.NewReader(clientConn), 1)
	go func() {
		_, _ = bufio.NewReader(serverConn).ReadBytes('\n')
	}()

	_, _, err := client.SendRequestWithTimeoutGeneration(ReqPing, nil, 25*time.Millisecond)
	if !errors.Is(err, ErrRequestTimeout) {
		t.Fatalf("response timeout error = %v; want ErrRequestTimeout", err)
	}
	client.Close()
}

func TestEventHandlerConcurrentUpdate(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()
	client := NewClient(nil)
	client.conn = clientConn
	client.connected = true
	client.generation = 1

	var handled atomic.Int64
	handler := func(Event) { handled.Add(1) }
	client.SetEventHandler(handler)
	readDone := make(chan struct{})
	go func() {
		client.readLoop(clientConn, bufio.NewReader(clientConn), 1)
		close(readDone)
	}()
	payload, err := json.Marshal(Event{IsEvent: true, Type: "concurrent"})
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		for i := 0; i < 5000; i++ {
			client.SetEventHandler(handler)
		}
	}()
	go func() {
		defer workers.Done()
		<-start
		for i := 0; i < 200; i++ {
			if _, err := serverConn.Write(append(payload, '\n')); err != nil {
				return
			}
		}
	}()
	close(start)
	workers.Wait()
	client.Close()
	select {
	case <-readDone:
	case <-time.After(time.Second):
		t.Fatal("read loop did not stop after close")
	}
	deadline := time.Now().Add(time.Second)
	for handled.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if handled.Load() == 0 {
		t.Fatal("current-generation events were not delivered")
	}
}
