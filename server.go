package main

import (
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/tgregory/gnatsd/logger"
	"github.com/tgregory/gnatsd/server"
	//"github.com/tgregory/nats-websocket/shared"
	//"golang.org/x/net/websocket"
	"io"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	Closed = errors.New("Socket closed.")
)

type WebsocketConn struct {
	*websocket.Conn
	r io.Reader
}

func (wc *WebsocketConn) Read(b []byte) (n int, err error) {
	var tpe int
	var r io.Reader
	for nil == wc.r && nil == err {
		tpe, r, err = wc.NextReader()
		if nil != err {
			break
		}
		if tpe == websocket.BinaryMessage {
			wc.r = r
		}
	}
	if nil != err {
		fmt.Println(err)
		return 0, err
	}
	n, err = wc.r.Read(b)
	if io.EOF == err {
		err = nil
		wc.r = nil
	}
	if n != 0 {
		fmt.Println("Read: ", string(b[:n]))
	}
	return n, err
}

func (wc *WebsocketConn) SetDeadline(t time.Time) error {
	if err := wc.SetReadDeadline(t); nil != err {
		return err
	}
	return wc.SetWriteDeadline(t)
}

func (wc *WebsocketConn) Write(b []byte) (n int, err error) {
	n, err = len(b), wc.WriteMessage(websocket.BinaryMessage, b)
	fmt.Println("Written: ", string(b[:n]))
	return n, err
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func makeGzipHandler(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			fn(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		gzr := gzipResponseWriter{Writer: gz, ResponseWriter: w}
		fn(gzr, r)
	}
}

var (
	ListenerClosed = errors.New("Listener closed.")
)

type WebsocketAddr string

func (wa WebsocketAddr) Network() string {
	return "ws"
}

func (wa WebsocketAddr) String() string {
	return string(wa)
}

type websocketListener struct {
	addr   WebsocketAddr
	queue  chan net.Conn
	snc    *sync.RWMutex
	closed bool
	srv    *http.Server
	err    chan error
}

func Listen(laddr string) (*websocketListener, error) {
	fmt.Println("Creating websocket listener.")
	lst := &websocketListener{
		addr:  WebsocketAddr(laddr), //WebsocketAddr(fmt.Sprintf("%v%v", laddr, "/nats")),
		snc:   &sync.RWMutex{},
		queue: make(chan net.Conn, 1),
		err:   make(chan error),
	}
	fmt.Println(lst.Addr())
	mux := http.NewServeMux()
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  10,
		WriteBufferSize: 10,
	}
	/*mux.Handle("/nats", websocket.Handler(func(ws *websocket.Conn) {
		if nil != ws {
			lst.snc.RLock()
			defer lst.snc.RUnlock()
			if !lst.closed {
				lst.queue <- &shared.MonitoredConn{ws}
			}
		}
	}))*/
	mux.HandleFunc("/nats", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if nil == err {
			lst.snc.RLock()
			defer lst.snc.RUnlock()
			if !lst.closed {
				lst.queue <- &WebsocketConn{ws, nil}
				//lst.queue <- &shared.MonitoredConn{ws.UnderlyingConn()}
			}
		}
	})
	mux.HandleFunc("/", makeGzipHandler(http.FileServer(http.Dir("./")).ServeHTTP))
	lst.srv = &http.Server{
		Handler: mux,
		Addr:    laddr,
	}
	go func() {
		fmt.Println("Starting connections accept.")
		e := lst.srv.ListenAndServe()
		//e := http.ListenAndServe(laddr, mux)
		fmt.Println(e)
		lst.err <- e
	}()
	fmt.Printf("%+v\n", lst)
	return lst, nil
}

func (wl *websocketListener) Accept() (net.Conn, error) {
	fmt.Println("Accepting.")
	select {
	case con := <-wl.queue:
		fmt.Printf("%+v\n", con)
		if nil == con {
			return nil, ListenerClosed
		}
		return con, nil
	case err := <-wl.err:
		fmt.Println(err)
		wl.Close()
		return nil, err
	}
}

func (wl *websocketListener) Close() error {
	wl.snc.Lock()
	defer wl.snc.Unlock()
	wl.closed = true
	return nil
}

func (wl *websocketListener) Addr() net.Addr {
	return wl.addr
}

type WebsocketListenerCreator struct{}

func (wlc WebsocketListenerCreator) CreateListener(laddr string) (net.Listener, error) {
	return Listen(laddr)
}

func main() {
	srv := server.NewCustom(&server.Options{
		Host:     "",
		Port:     8888,
		HTTPPort: 8887,
	}, WebsocketListenerCreator{})
	/*srv := server.NewCustom(&server.Options{
		Host: "",
		Port: 8888,
	}, nil)*/
	srv.SetLogger(logger.NewFileLogger("server.log", true, true, true, true), true, true)
	srv.Start()
	for {
		time.Sleep(1 * time.Minute)
		runtime.Gosched()
	}
}
