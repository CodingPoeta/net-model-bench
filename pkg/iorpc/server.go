package iorpc

import (
	"fmt"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type Headers interface {
	Encode(io.Writer) (int, error)
	Decode([]byte) error
}

type Request struct {
	Service Service
	Headers Headers
	Body    Body
}

type Response struct {
	Headers Headers
	Body    Body
}

// HandlerFunc is a server handler function.
//
// clientAddr contains client address returned by Listener.Accept().
// Request and response types may be arbitrary.
// All the request and response types the HandlerFunc may use must be registered
// with RegisterType() before starting the server.
// There is no need in registering base Go types such as int, string, bool,
// float64, etc. or arrays, slices and maps containing base Go types.
//
// Hint: use Dispatcher for HandlerFunc construction.
type HandlerFunc func(clientAddr string, request Request) (response *Response, err error)

// Server implements RPC server.
//
// Default server settings are optimized for high load, so don't override
// them without valid reason.
type Server struct {
	// Address to listen to for incoming connections.
	//
	// The address format depends on the underlying transport provided
	// by Server.Listener. The following transports are provided
	// out of the box:
	//   * TCP - see NewTCPServer() and NewTCPClient().
	//   * TLS (aka SSL) - see NewTLSServer() and NewTLSClient().
	//   * Unix sockets - see NewUnixServer() and NewUnixClient().
	//
	// By default TCP transport is used.
	Addr string

	// Handler function for incoming requests.
	//
	// Server calls this function for each incoming request.
	// The function must process the request and return the corresponding response.
	//
	// Hint: use Dispatcher for HandlerFunc construction.
	Handler HandlerFunc

	// The maximum number of concurrent rpc calls the server may perform.
	// Default is DefaultConcurrency.
	Concurrency int

	// The maximum delay between response flushes to clients.
	//
	// Negative values lead to immediate requests' sending to the client
	// without their buffering. This minimizes rpc latency at the cost
	// of higher CPU and network usage.
	//
	// Default is DefaultFlushDelay.
	FlushDelay time.Duration

	// The maximum number of pending responses in the queue.
	// Default is DefaultPendingMessages.
	PendingResponses int

	// Size of send buffer per each underlying connection in bytes.
	// Default is DefaultBufferSize.
	SendBufferSize int

	// Size of recv buffer per each underlying connection in bytes.
	// Default is DefaultBufferSize.
	RecvBufferSize int

	// OnConnect is called whenever connection from client is accepted.
	// The callback can be used for authentication/authorization/encryption
	// and/or for custom transport wrapping.
	//
	// See also Listener, which can be used for sophisticated transport
	// implementation.
	OnConnect OnConnectFunc

	// The server obtains new client connections via Listener.Accept().
	//
	// Override the listener if you want custom underlying transport
	// and/or client authentication/authorization.
	// Don't forget overriding Client.Dial() callback accordingly.
	//
	// See also OnConnect for authentication/authorization purposes.
	//
	// * NewTLSClient() and NewTLSServer() can be used for encrypted rpc.
	// * NewUnixClient() and NewUnixServer() can be used for fast local
	//   inter-process rpc.
	//
	// By default it returns TCP connections accepted from Server.Addr.
	Listener Listener

	// LogError is used for error logging.
	//
	// By default the function set via SetErrorLogger() is used.
	LogError LoggerFunc

	// Connection statistics.
	//
	// The stats doesn't reset automatically. Feel free resetting it
	// any time you wish.
	Stats ConnStats

	// CloseBody is used to close body immediately after reading it.
	// This is useful for save memory when you don't need the body.
	CloseBody bool

	serverStopChan chan struct{}
	stopWg         sync.WaitGroup
}

// Start starts rpc server.
//
// All the request and response types the Handler may use must be registered
// with RegisterType() before starting the server.
// There is no need in registering base Go types such as int, string, bool,
// float64, etc. or arrays, slices and maps containing base Go types.
func (s *Server) Start() error {
	if s.LogError == nil {
		s.LogError = errorLogger
	}
	if s.Handler == nil {
		panic("gorpc.Server: Server.Handler cannot be nil")
	}

	if s.serverStopChan != nil {
		panic("gorpc.Server: server is already running. Stop it before starting it again")
	}
	s.serverStopChan = make(chan struct{})

	if s.Concurrency <= 0 {
		s.Concurrency = DefaultConcurrency
	}
	if s.FlushDelay == 0 {
		s.FlushDelay = DefaultFlushDelay
	}
	if s.PendingResponses <= 0 {
		s.PendingResponses = DefaultPendingMessages
	}
	if s.SendBufferSize <= 0 {
		s.SendBufferSize = DefaultBufferSize
	}
	if s.RecvBufferSize <= 0 {
		s.RecvBufferSize = DefaultBufferSize
	}

	if s.Listener == nil {
		s.Listener = &defaultListener{}
	}
	if s.Listener.ListenAddr() == nil {
		err := s.Listener.Init(s.Addr)
		if err != nil {
			err = fmt.Errorf("gorpc.Server: [%s]. Cannot listen to: [%s]", s.Addr, err)
			s.LogError("%s", err)
			return err
		}
	}

	workersCh := make(chan struct{}, s.Concurrency)
	s.stopWg.Add(1)
	go serverHandler(s, workersCh)
	return nil
}

// Stop stops rpc server. Stopped server can be started again.
func (s *Server) Stop() {
	if s.serverStopChan == nil {
		panic("gorpc.Server: server must be started before stopping it")
	}
	close(s.serverStopChan)
	s.stopWg.Wait()
	s.serverStopChan = nil
}

// Serve starts rpc server and blocks until it is stopped.
func (s *Server) Serve() error {
	if err := s.Start(); err != nil {
		return err
	}
	s.stopWg.Wait()
	return nil
}

func serverHandler(s *Server, workersCh chan struct{}) {
	defer s.stopWg.Done()

	var conn io.ReadWriteCloser
	var clientAddr string
	var err error
	var stopping atomic.Value

	for {
		acceptChan := make(chan struct{})
		go func() {
			if conn, clientAddr, err = s.Listener.Accept(); err != nil {
				if stopping.Load() == nil {
					s.LogError("gorpc.Server: [%s]. Cannot accept new connection: [%s]", s.Addr, err)
				}
			}
			close(acceptChan)
		}()

		select {
		case <-s.serverStopChan:
			stopping.Store(true)
			s.Listener.Close()
			<-acceptChan
			return
		case <-acceptChan:
			s.Stats.incAcceptCalls()
		}

		if err != nil {
			s.Stats.incAcceptErrors()
			select {
			case <-s.serverStopChan:
				return
			case <-time.After(time.Second):
			}
			continue
		}

		s.stopWg.Add(1)
		go serverHandleConnection(s, conn, clientAddr, workersCh)
	}
}

func serverHandleConnection(s *Server, conn io.ReadWriteCloser, clientAddr string, workersCh chan struct{}) {
	defer s.stopWg.Done()

	if s.OnConnect != nil {
		newConn, err := s.OnConnect(clientAddr, conn)
		if err != nil {
			s.LogError("gorpc.Server: [%s]->[%s]. OnConnect error: [%s]", clientAddr, s.Addr, err)
			conn.Close()
			return
		}
		conn = newConn
	}

	var enabledCompression bool
	var err error
	var stopping atomic.Value

	zChan := make(chan bool, 1)
	go func() {
		var buf [1]byte
		if _, err = conn.Read(buf[:]); err != nil {
			if stopping.Load() == nil {
				s.LogError("gorpc.Server: [%s]->[%s]. Error when reading handshake from client: [%s]", clientAddr, s.Addr, err)
			}
		}
		zChan <- (buf[0] != 0)
	}()
	select {
	case enabledCompression = <-zChan:
		if err != nil {
			conn.Close()
			return
		}
	case <-s.serverStopChan:
		stopping.Store(true)
		conn.Close()
		return
	case <-time.After(10 * time.Second):
		s.LogError("gorpc.Server: [%s]->[%s]. Cannot obtain handshake from client during 10s", clientAddr, s.Addr)
		conn.Close()
		return
	}

	responsesChan := make(chan *serverMessage, s.PendingResponses)
	stopChan := make(chan struct{})

	readerDone := make(chan struct{})
	go serverReader(s, conn, clientAddr, responsesChan, stopChan, readerDone, enabledCompression, workersCh)

	writerDone := make(chan struct{})
	go serverWriter(s, conn, clientAddr, responsesChan, stopChan, writerDone, enabledCompression)

	select {
	case <-readerDone:
		close(stopChan)
		conn.Close()
		<-writerDone
	case <-writerDone:
		close(stopChan)
		conn.Close()
		<-readerDone
	case <-s.serverStopChan:
		close(stopChan)
		conn.Close()
		<-readerDone
		<-writerDone
	}
}

type serverMessage struct {
	ID         uint64
	Request    *Request
	Response   *Response
	Error      string
	ClientAddr string
}

var serverMessagePool = &sync.Pool{
	New: func() interface{} {
		return &serverMessage{}
	},
}

func isClientDisconnect(err error) bool {
	return err == io.ErrUnexpectedEOF || err == io.EOF
}

func isServerStop(stopChan <-chan struct{}) bool {
	select {
	case <-stopChan:
		return true
	default:
		return false
	}
}

func serverReader(s *Server, r io.Reader, clientAddr string, responsesChan chan<- *serverMessage,
	stopChan <-chan struct{}, done chan<- struct{}, enabledCompression bool, workersCh chan struct{}) {

	defer func() {
		if r := recover(); r != nil {
			s.LogError("gorpc.Server: [%s]->[%s]. Panic when reading data from client: %v", clientAddr, s.Addr, r)
		}
		close(done)
	}()

	d := newMessageDecoder(r, &s.Stats, s.CloseBody)
	defer d.Close()

	var wr wireRequest
	for {
		if err := d.DecodeRequest(&wr); err != nil {
			if !isClientDisconnect(err) && !isServerStop(stopChan) {
				s.LogError("gorpc.Server: [%s]->[%s]. Cannot decode request: [%s]", clientAddr, s.Addr, err)
			}
			return
		}

		m := serverMessagePool.Get().(*serverMessage)
		m.ID = wr.ID
		m.Request = &Request{
			Service: Service(wr.Service),
			Headers: wr.Headers,
			Body:    wr.Body,
		}
		m.ClientAddr = clientAddr

		wr.ID = 0
		wr.Service = 0
		wr.Headers = nil
		wr.Body.Reset()

		select {
		case workersCh <- struct{}{}:
		default:
			select {
			case workersCh <- struct{}{}:
			case <-stopChan:
				return
			}
		}
		go serveRequest(s, responsesChan, stopChan, m, workersCh)
	}
}

func serveRequest(s *Server, responsesChan chan<- *serverMessage, stopChan <-chan struct{}, m *serverMessage, workersCh <-chan struct{}) {
	request := m.Request
	m.Request = nil
	clientAddr := m.ClientAddr
	m.ClientAddr = ""
	skipResponse := (m.ID == 0)

	if skipResponse {
		m.Response = nil
		m.Error = ""
		s.Stats.incRPCCalls()
		serverMessagePool.Put(m)
	}

	t := time.Now()
	response, err := callHandlerWithRecover(s.LogError, s.Handler, clientAddr, s.Addr, *request)
	s.Stats.incRPCTime(uint64(time.Since(t).Seconds() * 1000))

	if !skipResponse {
		m.Response = response
		m.Error = err

		// Select hack for better performance.
		// See https://github.com/valyala/gorpc/pull/1 for details.
		select {
		case responsesChan <- m:
		default:
			select {
			case responsesChan <- m:
			case <-stopChan:
			}
		}
	}

	<-workersCh
}

func callHandlerWithRecover(logErrorFunc LoggerFunc, handler HandlerFunc, clientAddr, serverAddr string, request Request) (response *Response, errStr string) {
	defer func() {
		if x := recover(); x != nil {
			stackTrace := make([]byte, 1<<20)
			n := runtime.Stack(stackTrace, false)
			errStr = fmt.Sprintf("Panic occured: %v\nStack trace: %s", x, stackTrace[:n])
			logErrorFunc("gorpc.Server: [%s]->[%s]. %s", clientAddr, serverAddr, errStr)
		}
	}()
	response, err := handler(clientAddr, request)
	if err != nil {
		return nil, err.Error()
	}
	return response, ""
}

func serverWriter(s *Server, w io.Writer, clientAddr string, responsesChan <-chan *serverMessage, stopChan <-chan struct{}, done chan<- struct{}, enabledCompression bool) {
	defer func() { close(done) }()

	e := newMessageEncoder(w, &s.Stats)
	defer e.Close()

	var wr wireResponse
	for {
		var m *serverMessage

		select {
		case m = <-responsesChan:
		default:
			// Give the last chance for ready goroutines filling responsesChan :)
			runtime.Gosched()

			select {
			case <-stopChan:
				return
			case m = <-responsesChan:
			}
		}

		wr.ID = m.ID
		wr.Error = m.Error
		if m.Response != nil {
			wr.Body = m.Response.Body
			wr.Headers = m.Response.Headers
		}

		m.Response = nil
		m.Error = ""
		serverMessagePool.Put(m)

		if err := e.EncodeResponse(wr); err != nil {
			s.LogError("gorpc.Server: [%s]->[%s]. Cannot send response to wire: [%s]", clientAddr, s.Addr, err)
			return
		}
		wr.Body.Reset()
		wr.Headers = nil

		s.Stats.incRPCCalls()
	}
}
