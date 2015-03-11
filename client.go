package main

import (
	"./shared"
	"errors"
	"fmt"
	"github.com/gopherjs/websocket"
	"github.com/tgregory/nats"
	"honnef.co/go/js/dom"
	"math/rand"
	"net"
	"time"
)

var names = [5]string{
	"Willy",
	"Greg",
	"Margaret",
	"Sam",
	"George",
}

var lastNames = [5]string{
	"Johnson",
	"Ivanov",
	"Simpson",
	"Davidson",
	"Thatcher",
}

var (
	TimedOut = errors.New("Dial timed out.")
)

type WebsocketTimeoutDialer struct {
}

func (wtd WebsocketTimeoutDialer) DialTimeout(host string, timeout time.Duration) (net.Conn, error) {
	result := make(chan net.Conn, 1)
	err := make(chan error, 1)
	tout := time.After(timeout)
	go func() {
		conn, e := websocket.Dial(host)
		if nil != e {
			err <- e
		} else {
			result <- conn
		}
	}()
	select {
	case res := <-result:
		return res, nil
	case e := <-err:
		return nil, e
	case <-tout:
		return nil, TimedOut
	}
}

func main() {
	conn, err := nats.ConnectDialer("ws://localhost/nats", WebsocketTimeoutDialer{})
	if nil != err {
		panic(err)
	}
	ec, err := nats.NewEncodedConn(conn, "json")
	if nil != err {
		panic(err)
	}
	defer ec.Close()
	recvCh := make(chan *shared.Message)
	ec.BindRecvChan("messages", recvCh)

	sendCh := make(chan *shared.Message, 10)
	ec.BindSendChan("messages", sendCh)
	d := dom.GetWindow().Document()
	author := fmt.Sprintf("%v %v", names[rand.Int31n(5)], lastNames[rand.Int31n(5)])
	d.GetElementByID("#user").(*dom.BasicElement).SetInnerHTML(author)
	messageInput := d.GetElementByID("#message").(*dom.BasicElement)
	d.GetElementByID("#send").(*dom.BasicElement).AddEventListener("click", false, func(evt dom.Event) {
		data := messageInput.TextContent()
		timestamp := time.Now()
		messageInput.SetTextContent("")
		go func() {
			sendCh <- &shared.Message{
				Timestamp: timestamp,
				Author:    author,
				Data:      data,
			}
		}()
	})
	messageBoard := d.GetElementByID("#message-board").(*dom.BasicElement)
	for {
		mes := <-recvCh
		mesEl := d.CreateElement("tr")
		mesEl.SetInnerHTML(fmt.Sprintf("<td>%v</td><td>%v</td><td>%v</td>", mes.Timestamp, mes.Author, mes.Data))
		messageBoard.AppendChild(mesEl)
	}
}
