package cherryNats

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	cerror "github.com/cherry-game/cherry/error"
	clog "github.com/cherry-game/cherry/logger"
	"github.com/nats-io/nats.go"
)

const (
	REQ_ID = "reqID"
	CON_ID = "conID"
)

type (
	Connect struct {
		*nats.Conn                      //
		options                         //
		id         int                  //
		seq        uint64               //
		waiters    sync.Map             // map[string]chan *nats.Msg
		subs       []*nats.Subscription //
		reply      string               // request reply subject
	}

	options struct {
		address       string
		maxReconnects int
		user          string
		password      string
		isStats       bool
	}
	OptionFunc func(o *options)
)

func NewConnect(id int, replySubject string, opts ...OptionFunc) *Connect {
	conn := &Connect{
		id:    id,
		reply: fmt.Sprintf("%s.%d", replySubject, id),
	}

	if len(opts) > 0 {
		for _, opt := range opts {
			opt(&conn.options)
		}
	}

	return conn
}

func (p *Connect) Connect() {
	if p.Conn != nil {
		return
	}

	for {
		conn, err := nats.Connect(p.address, p.natsOptions()...)
		if err != nil {
			clog.Warnf("[id = %d] Nats connect fail! retrying in 3 seconds. err = %s", p.id, err)
			time.Sleep(3 * time.Second)
			continue
		}
		p.Conn = conn
		p.initReplySubscribe()

		if p.isStats {
			go p.statistics()
		}

		break
	}
}

func (p *Connect) Subs() []*nats.Subscription {
	return p.subs
}

func (p *Connect) Close() {
	if p.IsConnected() {
		for _, sub := range p.subs {
			sub.Unsubscribe()
		}

		p.Conn.Close()
	}
}

func (p *Connect) statistics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		for _, sub := range p.subs {
			if dropped, err := sub.Dropped(); err != nil {
				clog.Errorf("Dropped messages. [subject = %s, dropped = %d, err = %v]",
					sub.Subject,
					dropped,
					err,
				)
			}
		}

		stats := p.Stats()
		clog.Debugf("[Statistics] InMsgs = %d, OutMsgs = %d, InBytes = %d, OutBytes = %d, Reconnects = %d",
			stats.InMsgs,
			stats.OutMsgs,
			stats.InBytes,
			stats.OutBytes,
			stats.Reconnects,
		)
	}
}

func (p *Connect) GetID() int {
	return p.id
}

func (p *Connect) initReplySubscribe() {
	err := p.Subscribe(p.reply, func(msg *nats.Msg) {
		reqID := msg.Header.Get(REQ_ID)
		if reqID == "" {
			clog.Infof("header = %v, subject = %v", msg.Header, msg.Subject)
			return
		}

		if chMsg, ok := p.waiters.LoadAndDelete(reqID); ok {
			ch := chMsg.(chan *nats.Msg)
			select {
			case ch <- msg:
			default:
			}
			close(ch)
		} else {
			clog.Warnf("Waiter not found for reqID = %s", reqID) // ← 添加这行
		}
	})
	if err != nil {
		clog.Warnf(" err = %v", err)
		return
	}
}

func (p *Connect) Request(subject string, data []byte, tod ...time.Duration) ([]byte, error) {
	timeout := GetTimeout(tod...)
	natsMsg, err := p.Conn.Request(subject, data, timeout)
	if err != nil {
		return nil, err
	}

	return natsMsg.Data, nil
}

func (p *Connect) RequestSync(subject string, data []byte, tod ...time.Duration) ([]byte, time.Time, error) {
	timeout := GetTimeout(tod...)

	reqID := strconv.FormatUint(atomic.AddUint64(&p.seq, 1), 10)
	ch := make(chan *nats.Msg, 1)
	p.waiters.Store(reqID, ch)

	// get msg from pool
	msg := GetMsg()
	msg.Subject = subject
	msg.Reply = p.reply
	msg.Header.Set(REQ_ID, reqID)
	msg.Header.Set(CON_ID, strconv.FormatInt(int64(p.id), 10))
	msg.Data = data
	err := p.PublishMsg(msg)
	// release msg
	ReleaseMsg(msg)

	if err != nil {
		p.waiters.Delete(reqID)
		close(ch)
		return nil, time.Time{}, err
	}

	// 用 NewTimer + Stop，避免 time.After 在“成功先返回”时把 Timer 挂在运行时堆里直到 timeout。
	// 超时与 reply 回调可能并发争用同一个 waiter：两边都必须用 LoadAndDelete 抢所有权，
	// 只有抢到的一方才能 close(ch)，否则会 double-close panic。
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case resp, ok := <-ch:
		replyAt := time.Now()
		if !ok || resp == nil {
			return nil, time.Time{}, cerror.ClusterRequestTimeout
		}
		// replyAt is the local timestamp at which the reply woke the waiter.
		// Callers use it to distinguish NATS request/reply time from post-reply work.
		return resp.Data, replyAt, nil
	case <-timer.C:
		if _, loaded := p.waiters.LoadAndDelete(reqID); loaded {
			close(ch)
		}
		clog.Warnf("NatsResSync timeout id = %d, reqID = %s", p.id, reqID)
		return nil, time.Time{}, cerror.ClusterRequestTimeout
	}
}

func (p *Connect) Subscribe(subject string, cb nats.MsgHandler) error {
	sub, err := p.Conn.Subscribe(subject, cb)
	if err != nil {
		return err
	}

	if sub != nil {
		p.subs = append(p.subs, sub)
	}

	return nil
}

func (p *Connect) QueueSubscribe(subject, queue string, cb nats.MsgHandler) error {
	sub, err := p.Conn.QueueSubscribe(subject, queue, cb)
	if err != nil {
		return err
	}

	if sub != nil {
		p.subs = append(p.subs, sub)
	}

	return nil
}

func (p *options) natsOptions() []nats.Option {
	var opts []nats.Option

	if reconnectDelay > 0 {
		opts = append(opts, nats.ReconnectWait(reconnectDelay))
	}

	if p.maxReconnects > 0 {
		opts = append(opts, nats.MaxReconnects(p.maxReconnects))
	}

	opts = append(opts, nats.DisconnectErrHandler(func(conn *nats.Conn, err error) {
		if err != nil {
			clog.Warnf("Disconnect error. [error = %v]", err)
		}
	}))

	opts = append(opts, nats.ReconnectHandler(func(nc *nats.Conn) {
		clog.Warnf("Reconnected [%s]", nc.ConnectedUrl())
	}))

	opts = append(opts, nats.ClosedHandler(func(nc *nats.Conn) {
		if nc.LastError() != nil {
			clog.Infof("error = %v", nc.LastError())
		}
	}))

	opts = append(opts, nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
		clog.Warnf("IsConnect = %v. %s on connection for subscription on %q",
			nc.IsConnected(),
			err.Error(),
			sub.Subject,
		)
	}))

	if p.user != "" {
		opts = append(opts, nats.UserInfo(p.user, p.password))
	}

	return opts
}

func (p *options) Address() string {
	return p.address
}

func (p *options) MaxReconnects() int {
	return p.maxReconnects
}

func WithAddress(address string) OptionFunc {
	return func(opts *options) {
		opts.address = address
	}
}

func WithParams(maxReconnects int) OptionFunc {
	return func(opts *options) {
		opts.maxReconnects = maxReconnects
	}
}

func WithAuth(user, password string) OptionFunc {
	return func(opts *options) {
		opts.user = user
		opts.password = password
	}
}

func WithIsStats(isStats bool) OptionFunc {
	return func(opts *options) {
		opts.isStats = isStats
	}
}
