package zookeeper

import (
	"errors"
	"fmt"
)

/**
存放使用的常量
*/
//节点的模式[（临时|永久）（有序号|没序号）]
const (
	ModePersistent         = 0
	ModeEphemeral          = 1
	ModePersistentSequence = 2
	ModeEphemeralSequence  = 3
)

const ZKDefaultPort = 2181
const protocolVersion = 0

const (
	opNotify       = 0
	opCreate       = 1
	opDelete       = 2
	opExists       = 3
	opGetData      = 4
	opSetData      = 5
	opGetAcl       = 6
	opSetAcl       = 7
	opGetChildren  = 8
	opSync         = 9
	opPing         = 11
	opGetChildren2 = 12
	opCheck        = 13
	opClose   = -11
	opSetAuth = 100
	opError = -1
	opWatcherEvent = -2
)

var (
	emptyPassword = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	opNames       = map[int32]string{
		opNotify:       "notify",
		opCreate:       "create",
		opDelete:       "delete",
		opExists:       "exists",
		opGetData:      "getData",
		opSetData:      "setData",
		opGetAcl:       "getACL",
		opSetAcl:       "setACL",
		opGetChildren:  "getChildren",
		opSync:         "sync",
		opPing:         "ping",
		opGetChildren2: "getChildren2",
		opCheck:        "check",

		opClose:   "close",
		opSetAuth: "setAuth",

	}
)


//zk服务节点模式[unknown|leader|follower|standalone]
type Mode uint8

const (
	ModeUnknown    Mode = iota
	ModeLeader     Mode = iota
	ModeFollower   Mode = iota
	ModeStandalone Mode = iota
)

var (
	modeNames = map[Mode]string{
		ModeUnknown:    "unknown",
		ModeLeader:     "leader",
		ModeFollower:   "follower",
		ModeStandalone: "standalone",
	}
)

func (m Mode) String() string {
	if name := modeNames[m]; name != "" {
		return name
	}
	return "unknown"
}

//acl permissions [r 1|w 2|create 4|del 8|admin 16 |all 31]
const (
	PermissionRead = 1 << iota
	PermissionWrite
	PermissionCreate
	PermissionDelete
	PermissionAdmin
	PermissionAll = 0x1f
)

//客户端连接zk时的状态
type State int32

const (
	StateUnknown           State = -1
	StateDisconnected      State = 0
	StateConnecting        State = 1
	StateAuthFailed        State = 4
	StateConnectedReadOnly State = 5
	StateSaslAuthenticated State = 6
	StateExpired           State = -112
	StateConnected         State = 100
	StateHasSession        State = 101
)

var (
	stateNames = map[State]string{
		StateUnknown:           "StateUnknown",
		StateDisconnected:      "StateDisconnected",
		StateConnecting:        "StateConnecting",
		StateAuthFailed:        "StateAuthFailed",
		StateConnectedReadOnly: "StateConnectedReadOnly",
		StateSaslAuthenticated: "StateSaslAuthenticated",
		StateExpired:           "StateExpired",
		StateConnected:         "StateConnected",
		StateHasSession:        "StateHasSession",
	}
)

func (s State) String() string {
	if name := stateNames[s]; name != "" {
		return name
	}
	return "unknown state"
}

//error
type ErrCode int32

const (
	errOk = 0
	errSystemError          = -1
	errRuntimeInconsistency = -2
	errDataInconsistency    = -3
	errConnectionLoss       = -4
	errMarshallingError     = -5
	errUnimplemented        = -6
	errOperationTimeout     = -7
	errBadArguments         = -8
	errInvalidState         = -9
	// API errors
	errAPIError                ErrCode = -100
	errNoNode                  ErrCode = -101 // *
	errNoAuth                  ErrCode = -102
	errBadVersion              ErrCode = -103 // *
	errNoChildrenForEphemerals ErrCode = -108
	errNodeExists              ErrCode = -110 // *
	errNotEmpty                ErrCode = -111
	errSessionExpired          ErrCode = -112
	errInvalidCallback         ErrCode = -113
	errInvalidAcl              ErrCode = -114
	errAuthFailed              ErrCode = -115
	errClosing                 ErrCode = -116
	errNothing                 ErrCode = -117
	errSessionMoved            ErrCode = -118
)

var (
	ErrUnknown                 = errors.New("zk: unknown error")
	ErrSessionExpired          = errors.New("zk: session has been expired by the server")
	ErrConnectionClosed        = errors.New("zk: connection closed")
	ErrAPIError                = errors.New("zk: api error")
	ErrNoNode                  = errors.New("zk: node does not exist")
	ErrNoAuth                  = errors.New("zk: not authenticated")
	ErrBadVersion              = errors.New("zk: version conflict")
	ErrNoChildrenForEphemerals = errors.New("zk: ephemeral nodes may not have children")
	ErrNodeExists              = errors.New("zk: node already exists")
	ErrNotEmpty                = errors.New("zk: node has children")
	ErrInvalidACL              = errors.New("zk: invalid ACL specified")
	ErrAuthFailed              = errors.New("zk: client authentication failed")
	ErrClosing                 = errors.New("zk: zookeeper is closing")
	ErrNothing                 = errors.New("zk: no server responses to process")
	ErrSessionMoved            = errors.New("zk: session moved to another server, so operation is ignored")
	ErrBadArguments = errors.New("invalid arguments")

	errCodeToError = map[ErrCode]error{
		0:                          nil,
		errAPIError:                ErrAPIError,
		errNoNode:                  ErrNoNode,
		errNoAuth:                  ErrNoAuth,
		errBadVersion:              ErrBadVersion,
		errNoChildrenForEphemerals: ErrNoChildrenForEphemerals,
		errNodeExists:              ErrNodeExists,
		errNotEmpty:                ErrNotEmpty,
		errSessionExpired:          ErrSessionExpired,
		errInvalidAcl:   ErrInvalidACL,
		errAuthFailed:   ErrAuthFailed,
		errClosing:      ErrClosing,
		errNothing:      ErrNothing,
		errSessionMoved: ErrSessionMoved,
		errBadArguments: ErrBadArguments,
	}
)

func (e ErrCode) toError() error {
	if err, ok := errCodeToError[e]; ok {
		return err
	}
	return fmt.Errorf("unknown error: %v", e)
}
