package main

import (
	"errors"
	"fmt"
	"github.com/tgregory/gnatsd/server"
	"golang.org/x/net/websocket"
	"net"
	"net/http"
	"runtime"
	"sync"
)

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
		addr:  WebsocketAddr(fmt.Sprintf("%v%v", laddr, "/nats")),
		snc:   &sync.RWMutex{},
		queue: make(chan net.Conn, 32),
		err:   make(chan error),
	}
	fmt.Println(lst.Addr())
	mux := http.NewServeMux()
	mux.Handle("/nats", websocket.Handler(func(ws *websocket.Conn) {
		if nil != ws {
			lst.snc.RLock()
			defer lst.snc.Unlock()
			if !lst.closed {
				lst.queue <- ws
			}
		}
	}))
	mux.Handle("/", http.FileServer(http.Dir("./")))
	lst.srv = &http.Server{
		Handler: mux,
		Addr:    laddr,
	}
	go func() {
		fmt.Println("Starting connections accept.")
		e := lst.srv.ListenAndServe()
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
		if nil == con {
			return nil, ListenerClosed
		}
		return con, nil
	case err := <-wl.err:
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
		Host:     "localhost",
		Port:     8888,
		HTTPPort: 20000,
	}, WebsocketListenerCreator{})
	srv.Start()
	for {
		runtime.Gosched()
	}
}
