package pool_test

import (
	"context"
	"net"
	"pool"
	"sync/atomic"
	"testing"
	"time"
)

var (
	PoolSize    = 5
	IdleSize    = 3
	MaxConnAge  = 10 * time.Second
	IdleTimeOut = 5 * time.Second

	//new conn times ,atomic
	newConnNum int32 = 0
)

type dummyConn struct {
	*net.TCPConn
}

func (d *dummyConn) Close() error {
	return nil
}

func dummyDialer() (net.Conn, error) {
	atomic.AddInt32(&newConnNum, 1)
	return &dummyConn{
		TCPConn: &net.TCPConn{},
	}, nil
}

func newPooler() pool.Pooler {
	opt := pool.Options{
		Dialer:      dummyDialer,
		PoolSize:    PoolSize,
		MinIdleSize: IdleSize,
		MaxConnAge:  MaxConnAge,
		IdleTimeout: IdleTimeOut,
	}

	return pool.NewConnPool(&opt)
}

func TestGetConn(t *testing.T) {
	p := newPooler()
	defer p.Close()
	if p.ConnNum() != IdleSize {
		t.Errorf("total conn nums must equals %d ,but now %d", IdleSize, p.ConnNum())
	}

	//wait to conn
	time.Sleep(1 * time.Second)
	if p.IdleNum() != IdleSize {
		t.Errorf("total idle conn nums must equals %d ,but now %d", IdleSize, p.IdleNum())
	}

	//request one conn
	conn, err := p.Get(context.Background())
	if err != nil {
		t.Errorf("new conn err %s", err)
	}

	if p.IdleNum() != IdleSize-1 {
		t.Errorf("idle conn nums must equals %d ,but now %d", IdleSize-1, p.IdleNum())
	}

	if p.ConnNum() != IdleSize {
		t.Errorf("conn nums must equals %d ,but now %d", IdleSize, p.ConnNum())
	}

	p.Put(conn, nil)
	if p.IdleNum() != IdleSize {
		t.Errorf("idle conn nums must equals %d, but now %d", IdleSize, p.IdleNum())
	}

	//wait for idle time out
	time.Sleep(IdleTimeOut)

	newIdleNum := atomic.LoadInt32(&newConnNum)
	conn, err = p.Get(context.Background())
	if err != nil {
		t.Errorf("new conn err %s", err)
	}

	//wait to connection to server
	time.Sleep(1 * time.Second)
	if atomic.LoadInt32(&newConnNum) < newIdleNum+int32(IdleSize) {
		t.Errorf("after idle time out ,pool must new at least %d conns,but now %d", IdleSize, atomic.LoadInt32(&newConnNum)-newIdleNum)
	}

}
