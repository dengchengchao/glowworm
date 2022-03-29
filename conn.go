package pool

import (
	"net"
	"sync/atomic"
	"time"
)

var atomicId int32 = 0

type Conn struct {

	//atomic
	lastActiveTime int64
	id             int32
	isBadConn      int32

	net.Conn
	createTime time.Time
}

func NewConn(conn net.Conn) *Conn {
	c := &Conn{
		Conn:           conn,
		createTime:     time.Now(),
		lastActiveTime: time.Now().Unix(),
		id:             atomic.AddInt32(&atomicId, 1),
		isBadConn:      1,
	}

	return c
}

func (c *Conn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	c.checkBadConn(err)
	c.refreshLastActiveTime()
	return n, err
}

func (c *Conn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.checkBadConn(err)
	c.refreshLastActiveTime()
	return n, err
}

func (c *Conn) checkBadConn(err error) {
	if ne, ok := err.(net.Error); ok && !ne.Temporary() {
		atomic.StoreInt32(&c.isBadConn, 1)
	}
}

func (c *Conn) refreshLastActiveTime() {
	atomic.StoreInt64(&c.lastActiveTime, time.Now().Unix())
}

func (c *Conn) LastActiveTime() time.Time {
	unix := atomic.LoadInt64(&c.lastActiveTime)
	return time.Unix(unix, 0)
}

func (c *Conn) IsBadConn() bool {
	return atomic.LoadInt32(&c.isBadConn) == 1
}

func (c *Conn) Id() int32 {
	return c.id
}
