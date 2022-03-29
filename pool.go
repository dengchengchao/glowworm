package pool

import (
	"context"
	"errors"
	"net"
	"pool/logger"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ClosedError = errors.New("pool is closed")

	BadConnError = errors.New("get a bad conn")
)

//Options  options for init conn pool
type Options struct {
	Dialer func() (net.Conn, error)
	//PoolSize max conn num
	PoolSize int

	//MinIdleSize min idle conn size
	MinIdleSize int

	//MaxConnAge   the max time the connection can alive
	MaxConnAge time.Duration

	//IdleTimeout  the max time the connection can be idle
	IdleTimeout time.Duration
}

type State struct {
	//WaitTimeMs Total waiting milliseconds time of Get()
	WaitTimeMs int64

	//IdleNum  Total number of idle conn
	IdleNum int

	//ConnNum Total number of conn
	ConnNum int

	//WaiterNum Total number of waiting free conn in Get()
	WaiterNum int

	//DialErrNum Total number of error to dial new conn
	DialErrNum int32

	//KeeperNum Total number of keep idle conn num request
	KeeperNum int

	//MaxAgeClosedNum Total number of connections closed due to age count
	MaxAgeClosedNum int32

	//MaxIdleClosedNum Total number of connections closed du to idle count
	MaxIdleClosedNum int32
}

type Pooler interface {
	Get(ctx context.Context) (*Conn, error)

	Put(*Conn, error)
}

type ConnPool struct {

	//pool state atomic
	waitTime   int64
	maxAgeNum  int32
	maxIdleNum int32

	opt *Options

	//with lock
	poolMu    sync.Mutex
	idleConns []*Conn
	connNums  int
	waiterNum int
	waitReqs  chan *Conn

	//atomic
	dialErrNum int32
	closed     int32
	checkNum   int32

	idleKeeper chan struct{}
	closeChan  chan struct{}
}

func NewConnPool(opt *Options) Pooler {
	p := &ConnPool{
		opt:        opt,
		idleConns:  make([]*Conn, 0, opt.PoolSize),
		idleKeeper: make(chan struct{}, opt.PoolSize),
		closeChan:  make(chan struct{}, 1),
		waitReqs:   make(chan *Conn),
	}
	p.newIdleConn()
	go p.keepIdleConn()
	return p
}

func (p *ConnPool) Get(ctx context.Context) (*Conn, error) {
	if p.isClosed() {
		return nil, ClosedError
	}

	for {
		p.poolMu.Lock()
		select {
		default:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		//idle connections
		if len(p.idleConns) > 0 {
			cn, valid := p.freeIdleConnLocked()
			p.poolMu.Unlock()
			if !valid {
				_ = cn.Close()
				continue
			}
			return cn, nil
		}

		//new connection
		if p.connNums < p.opt.PoolSize {
			p.connNums++
			p.poolMu.Unlock()
			return p.createNewConn()
		}

		//wait for idle conn
		p.waiterNum++
		p.poolMu.Unlock()
		conn, err := p.waitIdleConn(ctx)
		if conn == nil {
			continue
		}
		return conn, err

	}
}

func (p *ConnPool) freeIdleConnLocked() (*Conn, bool) {
	conn := p.idleConns[0]
	p.idleConns = p.idleConns[1:]
	if !p.isValidConn(conn) {
		p.connNums--
		if len(p.idleConns) < p.opt.MinIdleSize {
			p.connNums++
			p.keepIdleConn()
		}
		return nil, false
	}
	return conn, true
}

func (p *ConnPool) createNewConn() (*Conn, error) {
	conn, err := p.newConn()
	if err != nil {
		p.poolMu.Lock()
		p.connNums--
		p.poolMu.Unlock()
		p.notifyWaitReqLocked(nil)
		return nil, err
	}
	return conn, nil
}

func (p *ConnPool) waitIdleConn(ctx context.Context) (*Conn, error) {
	waitStart := time.Now()
	select {
	case <-ctx.Done():
		p.poolMu.Lock()
		p.waiterNum--
		p.poolMu.Unlock()
		select {
		default:
		case cn := <-p.waitReqs:
			p.poolMu.Lock()
			if cn != nil {
				p.addNewIdleConnLocked(cn)
			}
			p.poolMu.Unlock()
		}
		return nil, ctx.Err()
	case conn := <-p.waitReqs:
		atomic.AddInt64(&p.waitTime, int64(time.Since(waitStart)/time.Millisecond))
		return conn, nil
	}
}

func (p *ConnPool) isValidConn(conn *Conn) bool {
	if conn.IsBadConn() {
		return false
	}

	tn := time.Now()
	if tn.Sub(conn.LastActiveTime()) > p.opt.IdleTimeout {
		atomic.AddInt32(&p.maxIdleNum, 1)
		return false
	}

	if tn.Sub(conn.createTime) > p.opt.MaxConnAge {
		atomic.AddInt32(&p.maxAgeNum, 1)
	}

	return true
}

func (p *ConnPool) notifyWaitReqLocked(conn *Conn) bool {
	if p.waiterNum > 0 {
		p.waitReqs <- conn
		p.waiterNum--
		return true
	}
	return false
}

func (p *ConnPool) Put(conn *Conn, err error) {

}

func (p *ConnPool) addNewIdleConnLocked(conn *Conn) {
	if p.notifyWaitReqLocked(conn) {
		return
	}

	if p.connNums < p.opt.PoolSize && len(p.idleConns) < p.opt.MinIdleSize {
		p.idleConns = append(p.idleConns, conn)
		return
	}
	p.connNums--
	_ = conn.Close()
}

func (p *ConnPool) newIdleConn() {
	for i := 0; i < p.opt.MinIdleSize; i++ {
		p.poolMu.Lock()
		p.connNums++
		p.poolMu.Unlock()
		p.keepIdleConn()
	}
}

func (p *ConnPool) newConn() (*Conn, error) {
	if p.isClosed() {
		return nil, ClosedError
	}
	if atomic.LoadInt32(&p.dialErrNum) >= int32(p.opt.PoolSize) {
		return nil, BadConnError
	}

	cn, err := p.opt.Dialer()
	if err != nil {
		atomic.AddInt32(&p.dialErrNum, 1)
		logger.Error("dial conn err %s, now dial err num %d ", err, atomic.LoadInt32(&p.dialErrNum))
		if atomic.LoadInt32(&p.dialErrNum) == int32(p.opt.PoolSize) {
			logger.Info("start tryDial")
			go p.tryDial()
		}
		return nil, err
	}

	return NewConn(cn), nil
}

func (p *ConnPool) tryDial() {
	for {
		if p.isClosed() {
			return
		}

		time.Sleep(1 * time.Second)
		conn, err := p.opt.Dialer()
		if err == nil {
			atomic.StoreInt32(&p.dialErrNum, 0)
			logger.Info("success connect %s , stop tryDial", conn.RemoteAddr())
			return
		}
	}
}

func (p *ConnPool) keepIdleConn() {
	logger.Debug("receive keep idle request")
	p.idleKeeper <- struct{}{}
}

func (p *ConnPool) isClosed() bool {
	return atomic.LoadInt32(&p.closed) == 1
}
