package main

import (
	"errors"
	"fmt"
	"github.com/gopherjs/websocket"
	"github.com/tgregory/nats"
	"github.com/tgregory/nats-websocket/shared"
	"honnef.co/go/js/dom"
	//"io"
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

type ConnMonitor struct {
	net.Conn
}

type WebsocketTimeoutDialer struct {
}

func (wtd WebsocketTimeoutDialer) DialTimeout(host string, timeout time.Duration) (net.Conn, error) {
	host = "ws://localhost:8888/nats"
	println("Preparing to dial: ", host)
	result := make(chan net.Conn, 1)
	err := make(chan error, 1)
	tout := time.After(timeout)
	go func() {
		println("Dialing.")
		conn, e := websocket.Dial(host)
		if nil != e {
			err <- e
		} else {
			result <- &shared.MonitoredConn{conn}
		}
	}()
	select {
	case res := <-result:
		println("Dial success.")
		return res, nil
	case e := <-err:
		println("Dial error.")
		println(e.Error())
		return nil, e
	case <-tout:
		println("Dial timeout.")
		return nil, TimedOut
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())
	sendCh := make(chan *shared.Message, 1)
	recvCh := make(chan *shared.Message, 1)
	author := fmt.Sprintf("%v %v", names[rand.Intn(5)], lastNames[rand.Intn(5)])
	go func() {
		conn, err := nats.ConnectDialer("ws://localhost:8888", WebsocketTimeoutDialer{})
		if nil != err {
			fmt.Printf("Nats connect failed: %v\n", err)
			return
		}
		//defer conn.Close()
		ec, err := nats.NewEncodedConn(conn, "json")
		if nil != err {
			fmt.Println(err.Error())
			return
		}
		ec.Flush()
		ec.BindRecvChan("messages", recvCh)
		ec.BindSendChan("messages", sendCh)
		ec.Subscribe("join", func(author string) {
			fmt.Println(author + " joined")
		})
		ec.Publish("join", author)
		ec.Flush()
	}()
	d := dom.GetWindow().Document()
	d.GetElementByID("user").(*dom.HTMLDivElement).SetInnerHTML(author)
	messageInput := d.GetElementByID("message").(*dom.HTMLInputElement)
	d.GetElementByID("send").(*dom.HTMLButtonElement).AddEventListener("click", false, func(evt dom.Event) {
		fmt.Println("Clicked.")
		data := messageInput.Value
		timestamp := time.Now()
		messageInput.Value = ""
		go func() {
			msg := &shared.Message{
				Timestamp: timestamp,
				Author:    author,
				Data:      data,
			}
			sendCh <- msg
		}()
	})
	messageBoard := d.GetElementByID("message-board").(*dom.HTMLTableElement)
	//go func() {
	for {
		mes := <-recvCh
		mesEl := d.CreateElement("tr")
		mesEl.SetInnerHTML(fmt.Sprintf("<td style='width: 4em;'>%v</td><td style='width: 10em;'>%v</td><td>%v</td>", mes.Timestamp.Format(time.Kitchen), mes.Author, mes.Data))
		messageBoard.AppendChild(mesEl)
	}
	//}()
}
