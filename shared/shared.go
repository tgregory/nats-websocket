package shared

import (
	"fmt"
	"net"
	"time"
)

type Message struct {
	Author    string
	Timestamp time.Time
	Data      string
}

type MonitoredConn struct {
	net.Conn
}

func (mc *MonitoredConn) Read(b []byte) (n int, err error) {
	n, err = mc.Conn.Read(b)
	if nil != err {
		fmt.Println(err)
	} else {
		if 0 != n {
			fmt.Println("Read: ", string(b[:n]))
		}
	}
	return n, err
}

func (mc *MonitoredConn) Write(b []byte) (n int, err error) {
	n, err = mc.Conn.Write(b)
	if nil != err {
		fmt.Println(err)
	} else {
		if 0 != n {
			fmt.Println("Written: ", string(b[:n]))
		}
	}
	return n, err
}
