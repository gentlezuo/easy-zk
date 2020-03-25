package zookeeper

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	//ErrNoServer = errors.New("zk: could not connect to a server")

	ErrInvalidPath = errors.New("zk: invalid path")
)

const (
	bufferSize = 1536 * 1024
	//eventChanSize   = 7
	sendChanSize = 16
	//protectedPrefix = "_c_"
)

var DefaultLogger Logger = defaultLogger{}

//scheme xxx:xxx
type authCreds struct {
	scheme string
	auth   []byte
}

/*type Event struct {
	Type   EventType
	State  State
	Path   string // For non-session events, the path of the watched node.
	Err    error
	Server string // For connection events
}*/

type Dialer func(network string, address string, timeout time.Duration) (net.Conn, error)

type Conn struct {
	lastZxid                  int64
	sessionID                 int64
	state                     State //conn的状态
	xid                       uint32
	sessionTimeoutMillisecond int32
	passwd                    []byte
	dialer                    Dialer
	serverMutex               sync.Mutex
	server                    string //zk服务地址
	conn                      net.Conn
	//eventChan      chan Event
	//eventCallback  EventCallBack
	shouldQuit     chan struct{}
	pingInterval   time.Duration
	recvTimeout    time.Duration
	connectTimeout time.Duration
	maxBufferSize  int
	logger         Logger
	creds          []authCreds
	credsMu        sync.Mutex // protects server

	sendChan     chan *request
	requestsLock sync.Mutex
	requests     map[int32]*request // Xid -> pending request

	// Debug (for recurring re-auth hang)
	debugCloseRecvLoop bool
	debugReauthDone    chan struct{}
	logInfo            bool
	buf                []byte

	// Debug (used by unit tests)
	reconnectLatch chan struct{}

	closeChan chan struct{} // channel to tell send loop stop
}

type request struct {
	xid        int32
	opcode     int32
	pkt        interface{}
	recvStruct interface{}
	recvChan   chan response

	recvFunc func(*request, *responseHeader, error)
}

type response struct {
	zxid int64
	err  error
}

func Connect(servers []string, sessionTimeout time.Duration) (*Conn, error) {
	if len(servers) == 0 {
		return nil, errors.New("zk: server list must not be empty")
	}

	srvs := make([]string, len(servers))
	for i, address := range servers {
		if strings.Contains(address, ":") {
			srvs[i] = address
		} else {
			srvs[i] = address + ":" + strconv.Itoa(ZKDefaultPort)
		}
	}

	//打乱
	srvs_str := strings.Join(srvs, ",")

	//ec := make(chan Event, eventChanSize)
	conn := &Conn{
		//lastZxid:                  0,
		//sessionID:                 0,
		state: StateDisconnected,
		//xid:                       0,
		//sessionTimeoutMillisecond: 0,
		passwd:      emptyPassword,
		dialer:      net.DialTimeout,
		serverMutex: sync.Mutex{},
		//server:                    "",
		conn: nil,
		//eventChan:      ec,
		//eventCallback:  nil,
		shouldQuit: make(chan struct{}),
		//pingInterval:   0,
		//recvTimeout:    0,
		server:         srvs_str,
		connectTimeout: 1 * time.Second,
		//maxBufferSize:  0,
		logger:  DefaultLogger,
		logInfo: true,
		//closeChan:      nil,
		sendChan: make(chan *request, sendChanSize),
		requests: make(map[int32]*request),
		buf:      make([]byte, bufferSize),
	}
	conn.setTimeouts(int32(sessionTimeout / time.Millisecond))

	go func() {
		conn.loop()
		conn.flushRequests(ErrClosing)
		//close(conn.eventChan)
	}()
	return conn, nil
}

func (c *Conn) loop() {
	for {
		//连接
		if err := c.connect(); err != nil {
			return
		}
		//验证
		err := c.authenticate()

		switch {
		//处理错误
		case err == ErrSessionExpired:
			c.logger.Printf("authentication failed: %s", err)

		case err != nil && c.conn != nil:
			c.logger.Printf("authentication failed: %s", err)
			c.conn.Close()
		case err == nil:
			c.logger.Printf("authenticated: id=%d, timeout=%d", c.SessionID(), c.sessionTimeoutMillisecond)

			//发送停止loop
			c.closeChan = make(chan struct{})
			//重新验证
			reauthChan := make(chan struct{})
			//todo
			var wg sync.WaitGroup

			//重新验证
			wg.Add(1)
			go func() {
				<-reauthChan
				/*if c.debugCloseRecvLoop {
					close(c.debugReauthDone)
				}*/
				err := c.sendLoop()
				if err != nil || c.logInfo {
					c.logger.Printf("send loop terminated: err=%v", err)
				}
				c.conn.Close()
				wg.Done()
			}()

			wg.Add(1)
			go func() {
				var err error
				/*if c.debugCloseRecvLoop {
					err = errors.New("DEBUG: close recv loop")
				} else {*/
				err = c.recvLoop(c.conn)
				//}
				if err != io.EOF || c.logInfo {
					c.logger.Printf("recv loop terminated: err=%v", err)
				}
				if err == nil {
					panic("zk: recvLoop should never return nil error")
				}
				close(c.closeChan) // tell send loop to exit
				wg.Done()
			}()

			//c.resendZkAuth(reauthChan)
			//c.sendSetWatches()
			wg.Wait()
		}
		c.setState(StateDisconnected)

		select {
		case <-c.shouldQuit:
			c.flushRequests(ErrClosing)
			return
		default:
		}

		if err != ErrSessionExpired {
			err = ErrConnectionClosed
		}
		c.flushRequests(err)

		if c.reconnectLatch != nil {
			select {
			case <-c.shouldQuit:
				return
			case <-c.reconnectLatch:
			}
		}
	}
}

func (c *Conn) Close() {
	close(c.shouldQuit)
	select {
	case <-c.queueRequest(opClose, &closeRequest{}, &closeResponse{}, nil):
	case <-time.After(time.Second):
	}
}

func (c *Conn) authenticate() error {
	buf := make([]byte, 256)
	//编码
	cr := &connectRequest{
		ProtocolVersion: protocolVersion,
		LastZxidSeen:    c.lastZxid,
		TimeOut:         c.sessionTimeoutMillisecond,
		SessionID:       c.SessionID(),
		Passwd:          c.passwd,
	}
	n, err := encodePacket(buf[4:], cr)

	if err != nil {
		return err
	}

	binary.BigEndian.PutUint32(buf[:4], uint32(n))

	if err := c.conn.SetWriteDeadline(time.Now().Add(c.recvTimeout * 10)); err != nil {
		return err
	}
	//发送
	_, err = c.conn.Write(buf[:n+4])
	if err != nil {
		return err
	}
	if err := c.conn.SetWriteDeadline(time.Time{}); err != nil {
		return err
	}

	// Receive and decode a connect response.
	if err := c.conn.SetReadDeadline(time.Now().Add(c.recvTimeout * 10)); err != nil {
		return err
	}
	_, err = io.ReadFull(c.conn, buf[:4])
	if err != nil {
		return err
	}
	if err := c.conn.SetReadDeadline(time.Time{}); err != nil {
		return err
	}

	blen := int(binary.BigEndian.Uint32(buf[:4]))
	if cap(buf) < blen {
		buf = make([]byte, blen)
	}

	_, err = io.ReadFull(c.conn, buf[:blen])
	if err != nil {
		return err
	}

	r := connectResponse{}
	//解码获得connectResponse
	_, err = decodePacket(buf[:blen], &r)
	if err != nil {
		return err
	}
	if r.SessionID == 0 {
		atomic.StoreInt64(&c.sessionID, int64(0))
		c.passwd = emptyPassword
		c.lastZxid = 0
		c.setState(StateExpired)
		return ErrSessionExpired
	}

	atomic.StoreInt64(&c.sessionID, r.SessionID)
	c.setTimeouts(r.TimeOut)
	c.passwd = r.Passwd
	c.setState(StateHasSession)

	return nil
}

func (c *Conn) resendZkAuth(reauthReadyChan chan struct{}) {
	//是否取消函数
	shouldCancel := func() bool {
		select {
		case <-c.shouldQuit:
			return true
		case <-c.closeChan:
			return true
		default:
			return false
		}
	}

	c.credsMu.Lock()
	defer c.credsMu.Unlock()

	defer close(reauthReadyChan)

	if c.logInfo {
		c.logger.Printf("re-submitting `%d` credentials after reconnect", len(c.creds))
	}

	for _, cred := range c.creds {
		if shouldCancel() {
			return
		}
		resChan, err := c.sendRequest(
			opSetAuth,
			&setAuthRequest{Type: 0,
				Scheme: cred.scheme,
				Auth:   cred.auth,
			},
			&setAuthResponse{},
			nil)

		if err != nil {
			c.logger.Printf("call to sendRequest failed during credential resubmit: %s", err)
			// FIXME(prozlach): lets ignore errors for now
			continue
		}

		var res response
		//结果处理
		select {
		case res = <-resChan:
		case <-c.closeChan:
			c.logger.Printf("recv closed, cancel re-submitting credentials")
			return
		case <-c.shouldQuit:
			c.logger.Printf("should quit, cancel re-submitting credentials")
			return
		}
		if res.err != nil {
			c.logger.Printf("credential re-submit failed: %s", res.err)
			// FIXME(prozlach): lets ignore errors for now
			continue
		}
	}
}

func (c *Conn) request(opcode int32, req interface{}, res interface{}, recvFunc func(*request, *responseHeader, error)) (int64, error) {
	recvChan := <-c.queueRequest(opcode, req, res, recvFunc)
	return recvChan.zxid, recvChan.err
}

func (c *Conn) queueRequest(opcode int32, req interface{}, res interface{}, recvFunc func(*request, *responseHeader, error)) <-chan response {
	rq := &request{
		xid:        c.nextXid(),
		opcode:     opcode,
		pkt:        req,
		recvStruct: res,
		recvChan:   make(chan response, 1),
		recvFunc:   recvFunc,
	}
	c.sendChan <- rq
	return rq.recvChan
}

func (c *Conn) AddAuth(scheme string, auth []byte) error {
	_, err := c.request(opSetAuth, &setAuthRequest{Type: 0, Scheme: scheme, Auth: auth}, &setAuthResponse{}, nil)

	if err != nil {
		return err
	}

	// Remember authdata so that it can be re-submitted on reconnect
	//
	// FIXME(prozlach): For now we treat "userfoo:passbar" and "userfoo:passbar2"
	// as two different entries, which will be re-submitted on reconnet. Some
	// research is needed on how ZK treats these cases and
	// then maybe switch to something like "map[username] = password" to allow
	// only single password for given user with users being unique.
	obj := authCreds{
		scheme: scheme,
		auth:   auth,
	}

	c.credsMu.Lock()
	c.creds = append(c.creds, obj)
	c.credsMu.Unlock()

	return nil
}

func (c *Conn) Get(path string) ([]byte, *Stat, error) {
	if err := validatePath(path, false); err != nil {
		return nil, nil, err
	}

	res := &getDataResponse{}
	_, err := c.request(opGetData, &getDataRequest{Path: path, Watch: false}, res, nil)
	return res.Data, &res.Stat, err
}

func (c *Conn) Children(path string) ([]string, *Stat, error) {
	if err := validatePath(path, false); err != nil {
		return nil, nil, err
	}

	res := &getChildren2Response{}
	_, err := c.request(opGetChildren2, &getChildren2Request{Path: path, Watch: false}, res, nil)
	return res.Children, &res.Stat, err
}

func (c *Conn) Set(path string, data []byte, version int32) (*Stat, error) {
	if err := validatePath(path, false); err != nil {
		return nil, err
	}

	res := &setDataResponse{}
	_, err := c.request(opSetData, &SetDataRequest{path, data, version}, res, nil)
	return &res.Stat, err
}

func (c *Conn) Create(path string, data []byte, flags int32, acl []ACL) (string, error) {
	if err := validatePath(path, flags == ModeEphemeralSequence || flags == ModePersistentSequence); err != nil {
		return "", err
	}

	res := &createResponse{}
	_, err := c.request(opCreate, &CreateRequest{path, data, acl, flags}, res, nil)
	return res.Path, err
}

func (c *Conn) Delete(path string, version int32) error {
	if err := validatePath(path, false); err != nil {
		return err
	}

	_, err := c.request(opDelete, &DeleteRequest{path, version}, &deleteResponse{}, nil)
	return err
}

func (c *Conn) Exists(path string) (bool, *Stat, error) {
	if err := validatePath(path, false); err != nil {
		return false, nil, err
	}

	res := &existsResponse{}
	_, err := c.request(opExists, &existsRequest{Path: path, Watch: false}, res, nil)
	exists := true
	//todo
	if err == ErrNoNode {
		exists = false
		err = nil
	}
	return exists, &res.Stat, err
}

func (c *Conn) GetACL(path string) ([]ACL, *Stat, error) {
	if err := validatePath(path, false); err != nil {
		return nil, nil, err
	}

	res := &getAclResponse{}
	_, err := c.request(opGetAcl, &getAclRequest{Path: path}, res, nil)
	return res.Acl, &res.Stat, err
}

func (c *Conn) SetACL(path string, acl []ACL, version int32) (*Stat, error) {
	if err := validatePath(path, false); err != nil {
		return nil, err
	}

	res := &setAclResponse{}
	_, err := c.request(opSetAcl, &setAclRequest{Path: path, Acl: acl, Version: version}, res, nil)
	return &res.Stat, err
}

func (c *Conn) Sync(path string) (string, error) {
	if err := validatePath(path, false); err != nil {
		return "", err
	}

	res := &syncResponse{}
	_, err := c.request(opSync, &syncRequest{Path: path}, res, nil)
	return res.Path, err
}

// SessionID returns the current session id of the connection.
func (c *Conn) SessionID() int64 {
	return atomic.LoadInt64(&c.sessionID)
}

func (c *Conn) recvLoop(conn net.Conn) error {
	//设置bufsize
	sz := bufferSize
	if c.maxBufferSize > 0 && sz > c.maxBufferSize {
		sz = c.maxBufferSize
	}
	buf := make([]byte, sz)
	for {
		if err := conn.SetReadDeadline(time.Now().Add(c.recvTimeout)); err != nil {
			c.logger.Printf("failed to set connection deadline: %v", err)
		}
		//获取buf长度
		_, err := io.ReadFull(conn, buf[:4])
		if err != nil {
			return fmt.Errorf("failed to read from connection: %v", err)
		}

		blen := int(binary.BigEndian.Uint32(buf[:4]))
		if cap(buf) < blen {
			if c.maxBufferSize > 0 && blen > c.maxBufferSize {
				return fmt.Errorf("received packet from server with length %d, which exceeds max buffer size %d", blen, c.maxBufferSize)
			}
			buf = make([]byte, blen)
		}

		//从conn中获取数据
		_, err = io.ReadFull(conn, buf[:blen])
		if err != nil {
			return err
		}
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			return err
		}

		res := responseHeader{}
		//buf前16个字节解码为responseHeader
		_, err = decodePacket(buf[:16], &res)
		if err != nil {
			return err
		}

		//根据xid判断类型
		/*if res.Xid == -1 {
			res := &watcherEvent{}
			_, err := decodePacket(buf[16:blen], res)
			if err != nil {
				return err
			}
			ev := Event{
				Type:  res.Type,
				State: res.State,
				Path:  res.Path,
				Err:   nil,
			}
			c.sendEvent(ev)
			wTypes := make([]watchType, 0, 2)
			switch res.Type {
			case EventNodeCreated:
				wTypes = append(wTypes, watchTypeExist)
			case EventNodeDeleted, EventNodeDataChanged:
				wTypes = append(wTypes, watchTypeExist, watchTypeData, watchTypeChild)
			case EventNodeChildrenChanged:
				wTypes = append(wTypes, watchTypeChild)
			}
			c.watchersLock.Lock()
			for _, t := range wTypes {
				wpt := watchPathType{res.Path, t}
				if watchers, ok := c.watchers[wpt]; ok {
					for _, ch := range watchers {
						ch <- ev
						close(ch)
					}
					delete(c.watchers, wpt)
				}
			}
			c.watchersLock.Unlock()
		} else*/
		if res.Xid == -2 {
			// Ping response. Ignore.
		} else if res.Xid < 0 {
			c.logger.Printf("Xid < 0 (%d) but not ping or watcher event", res.Xid)
		} else {
			if res.Zxid > 0 {
				c.lastZxid = res.Zxid
			}

			c.requestsLock.Lock()
			req, ok := c.requests[res.Xid]
			if ok {
				delete(c.requests, res.Xid)
			}
			c.requestsLock.Unlock()

			if !ok {
				c.logger.Printf("Response for unknown request with xid %d", res.Xid)
			} else {
				if res.Err != 0 {
					err = res.Err.toError()
				} else {
					//将recv解码到recvStruct
					_, err = decodePacket(buf[16:blen], req.recvStruct)
				}
				if req.recvFunc != nil {
					req.recvFunc(req, &res, err)
				}
				//发送chan，已经获取到response
				req.recvChan <- response{res.Zxid, err}
				if req.opcode == opClose {
					return io.EOF
				}
			}
		}
	}
}
func (c *Conn) sendLoop() error {
	//一直ping着
	pingTicker := time.NewTicker(c.pingInterval)
	defer pingTicker.Stop()

	for {
		select {
		//发送逻辑，通过sendChan通信
		case req := <-c.sendChan:
			if err := c.sendData(req); err != nil {
				return err
			}
		case <-pingTicker.C:
			n, err := encodePacket(c.buf[4:], &requestHeader{Xid: -2, Opcode: opPing})
			if err != nil {
				panic("zk: opPing should  fail to serialize")
			}
			binary.BigEndian.PutUint32(c.buf[:4], uint32(n))

			if err := c.conn.SetWriteDeadline(time.Now().Add(c.recvTimeout)); err != nil {
				return err
			}
			_, err = c.conn.Write(c.buf[:n+4])
			if err != nil {
				c.conn.Close()
				return err
			}
			//why,永远不超时
			if err := c.conn.SetWriteDeadline(time.Time{}); err != nil {
				return err
			}
		case <-c.closeChan:
			return nil
		}
	}
}

// Send error to all pending requests and clear request map
func (c *Conn) flushRequests(err error) {
	c.requestsLock.Lock()
	for _, req := range c.requests {
		req.recvChan <- response{-1, err}
	}
	c.requests = make(map[int32]*request)
	c.requestsLock.Unlock()
}

//返回state
func (c *Conn) State() State {
	return State(atomic.LoadInt32((*int32)(&c.state)))
}

//get sessionId
func (c *Conn) SessionId() int64 {
	return atomic.LoadInt64(&c.sessionID)
}

func (c *Conn) setState(state State) {
	atomic.StoreInt32((*int32)(&c.state), int32(state))
	//c.send
}

func (c *Conn) SetLogger(l Logger) {
	c.logger = l
}

func (c *Conn) setTimeouts(sessionTimeoutMs int32) {
	c.sessionTimeoutMillisecond = sessionTimeoutMs
	sessionTimeout := time.Duration(sessionTimeoutMs) * time.Millisecond
	c.recvTimeout = sessionTimeout * 2 / 3
	c.pingInterval = c.recvTimeout / 2
}

func (c *Conn) connect() error {
	for {
		//原子操作
		c.setState(StateConnecting)
		zkConn, err := c.dialer("tcp", c.server, c.connectTimeout)
		if err == nil {
			c.conn = zkConn
			c.setState(StateConnected)
			c.logger.Printf("Connected to %s", c.Server())
			return nil
		}
		c.logger.Printf("Failed to connect to %s: %+v", c.Server(), err)
	}
}

func (c *Conn) sendRequest(opcode int32, req interface{}, res interface{}, recvFunc func(*request, *responseHeader, error)) (<-chan response, error) {
	rq := &request{
		xid:        c.nextXid(),
		opcode:     opcode,
		pkt:        req,
		recvStruct: res,
		recvChan:   make(chan response, 1),
		recvFunc:   recvFunc,
	}
	if err := c.sendData(rq); err != nil {
		return nil, err
	}
	return rq.recvChan, nil

}

func (c *Conn) sendData(req *request) error {
	//格式[len|header|pkt]
	header := &requestHeader{req.xid, req.opcode}
	n, err := encodePacket(c.buf[4:], header)
	if err != nil {
		req.recvChan <- response{-1, err}
		return nil
	}

	n2, err := encodePacket(c.buf[4+n:], req.pkt)
	if err != nil {
		req.recvChan <- response{-1, err}
		return nil
	}

	n += n2
	binary.BigEndian.PutUint32(c.buf[:4], uint32(n))

	c.requestsLock.Lock()
	select {
	case <-c.closeChan:
		req.recvChan <- response{-1, ErrConnectionClosed}
		c.requestsLock.Unlock()
		return ErrConnectionClosed
	default:
	}

	c.requests[req.xid] = req
	c.requestsLock.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.recvTimeout)); err != nil {
		return err
	}

	_, err = c.conn.Write(c.buf[:n+4])
	if err != nil {
		req.recvChan <- response{-1, err}
		c.conn.Close()
		return err
	}

	if err := c.conn.SetWriteDeadline(time.Time{}); err != nil {
		return err
	}
	return nil
}

//安全地返回zk 地址
func (c *Conn) Server() string {
	c.serverMutex.Lock()
	defer c.serverMutex.Unlock()
	return c.server
}

//原子加一，越界重新开始计数
func (c *Conn) nextXid() int32 {
	return int32(atomic.AddUint32(&c.xid, 1) & 0x7fffffff)
}
