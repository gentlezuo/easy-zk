package zookeeper

import (
	"encoding/binary"
	"errors"
	"log"
	"reflect"
	"runtime"
	"strings"
)

var (
	ErrUnhandledFieldType = errors.New("zk: unhandled field type")
	ErrPtrExpected        = errors.New("zk: encode/decode expect a non-nil pointer to struct")
	ErrShortBuffer        = errors.New("zk: buffer too small")
)

//使用[scheme:id:permissions]来表示acl权限
//permission 是rwcdaa
//scheme是world: 它下面只有一个id, 叫anyone, world:anyone代表任何人，zookeeper中对所有人有权限的结点就是属于world:anyone的
//auth: 它不需要id, 只要是通过authentication的user都有权限（zookeeper支持通过kerberos来进行authencation, 也支持username/password形式的authentication)
//digest: 它对应的id为username:BASE64(SHA1(password))，它需要先通过username:password形式的authentication
//ip: 它对应的id为客户机的IP地址，设置的时候可以设置一个ip段，比如ip:192.168.1.0/16, 表示匹配前16个bit的IP段
//super: 在这种scheme情况下，对应的id拥有超级权限，可以做任何事情(cdrwa)
type ACL struct {
	Permission int32
	Scheme     string
	ID         string
}

//Node的Stat
type Stat struct {
	Czxid          int64 // cxid：创造node
	Mzxid          int64 // zxid：最后更改node
	Ctime          int64 // ctime：创造
	Mtime          int64 // mtime：更改
	Version        int32
	Cversion       int32
	Aversion       int32
	EphemeralOwner int64
	DataLength     int32
	NumChildren    int32
	Pzxid          int64
}

type Logger interface {
	Printf(string, ...interface{})
}

type defaultLogger struct{}

func (defaultLogger) Printf(format string, in ...interface{}) {
	log.Printf(format, in...)
}

type decoder interface {
	Decode(buf []byte) (int, error)
}

type encoder interface {
	Encode(buf []byte) (int, error)
}

type requestHeader struct {
	Xid    int32
	Opcode int32
}

type responseHeader struct {
	Xid  int32
	Zxid int64
	Err  ErrCode
}

/*type multiHeader struct {
	Type int32
	Done bool
	Err  ErrCode
}
*/
type auth struct {
	Type   int32
	Scheme string
	Auth   []byte
}

// Generic request structs

type pathRequest struct {
	Path string
}

type PathVersionRequest struct {
	Path    string
	Version int32
}

/*type pathWatchRequest struct {
	Path  string
	Watch bool
}*/

type pathResponse struct {
	Path string
}

type statResponse struct {
	Stat Stat
}

//

type CheckVersionRequest PathVersionRequest
type closeRequest struct{}
type closeResponse struct{}

type connectRequest struct {
	ProtocolVersion int32
	LastZxidSeen    int64
	TimeOut         int32
	SessionID       int64
	Passwd          []byte
}

type connectResponse struct {
	ProtocolVersion int32
	TimeOut         int32
	SessionID       int64
	Passwd          []byte
}

type CreateRequest struct {
	Path  string
	Data  []byte
	Acl   []ACL
	Flags int32
}

type createResponse pathResponse
type DeleteRequest PathVersionRequest
type deleteResponse struct{}
type errorResponse struct {
	Err int32
}

type existsRequest struct {
	Path  string
	Watch bool
}

//type existsRequest pathWatchRequest
type existsResponse statResponse
type getAclRequest pathRequest

type getAclResponse struct {
	Acl  []ACL
	Stat Stat
}

type getChildrenRequest pathRequest

type getChildrenResponse struct {
	Children []string
}

//type getChildren2Request pathWatchRequest
type getChildren2Request struct {
	Path  string
	Watch bool
}

type getChildren2Response struct {
	Children []string
	Stat     Stat
}

//type getDataRequest pathWatchRequest
type getDataRequest struct {
	Path  string
	Watch bool
}

type getDataResponse struct {
	Data []byte
	Stat Stat
}

type getMaxChildrenRequest pathRequest

type getMaxChildrenResponse struct {
	Max int32
}

type getSaslRequest struct {
	Token []byte
}

type pingRequest struct{}
type pingResponse struct{}

type setAclRequest struct {
	Path    string
	Acl     []ACL
	Version int32
}

type setAclResponse statResponse

type SetDataRequest struct {
	Path    string
	Data    []byte
	Version int32
}

type setDataResponse statResponse

type setMaxChildren struct {
	Path string
	Max  int32
}

type setSaslRequest struct {
	Token string
}

type setSaslResponse struct {
	Token string
}



type syncRequest pathRequest
type syncResponse pathResponse

type setAuthRequest auth
type setAuthResponse struct{}

//将buf中的数据解码为st，st通常是response类型
func decodePacket(buf []byte, st interface{}) (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(runtime.Error); ok && strings.HasPrefix(e.Error(), "runtime error: slice bounds out of range") {
				err = ErrShortBuffer
			} else {
				panic(r)
			}
		}
	}()

	v := reflect.ValueOf(st)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return 0, ErrPtrExpected
	}
	return decodePacketValue(buf, v)
}

func decodePacketValue(buf []byte, v reflect.Value) (int, error) {
	//todo 改逻辑
	rv := v
	kind := v.Kind()
	if kind == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
		kind = v.Kind()
	}
	n := 0
	switch kind {
	case reflect.Struct:
		//处理multi
		if de, ok := rv.Interface().(decoder); ok {
			return de.Decode(buf)
		} else if de, ok := v.Interface().(decoder); ok {
			return de.Decode(buf)
		} else {
			for i := 0; i < v.NumField(); i++ {
				field := v.Field(i)
				n2, err := decodePacketValue(buf[n:], field)
				n += n2
				if err != nil {
					return n, err
				}
			}
		}
	case reflect.Bool:
		v.SetBool(buf[n] != 0)
		n++
	case reflect.Int32:
		v.SetInt(int64(binary.BigEndian.Uint32(buf[n : n+4])))
		n += 4
	case reflect.Int64:
		v.SetInt(int64(binary.BigEndian.Uint64(buf[n : n+8])))
		n += 8
	case reflect.String:
		//string【len|str】
		ln := int(binary.BigEndian.Uint32(buf[n : n+4]))
		v.SetString(string(buf[n+4 : n+4+ln]))
		n += (4 + ln)
	case reflect.Slice:
		switch v.Type().Elem().Kind() {
		default:
			count := int(binary.BigEndian.Uint32(buf[n : n+4]))
			n += 4
			values := reflect.MakeSlice(v.Type(), count, count)
			v.Set(values)
			for i := 0; i < count; i++ {
				n2, err := decodePacketValue(buf[n:], values.Index(i))
				n += n2
				if err != nil {
					return n, err
				}
			}
		case reflect.Uint8:
			//[len|num]
			ln := int(int32(binary.BigEndian.Uint32(buf[n : n+4])))
			if ln < 0 {
				n += 4
				v.SetBytes(nil)
			} else {
				bytes := make([]byte, ln)
				copy(bytes, buf[n+4:n+4+ln])
				v.SetBytes(bytes)
				n += (4 + ln)
			}

		}
	default:
		return n, ErrUnhandledFieldType
	}
	return n, nil
}

func encodePacket(buf []byte, st interface{}) (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(runtime.Error); ok && strings.HasPrefix(e.Error(), "runtime error: slice bounds out of range") {
				err = ErrShortBuffer
			} else {
				panic(r)
			}
		}
	}()
	v := reflect.ValueOf(st)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return 0, ErrPtrExpected
	}
	return encodePacketValue(buf, v)
}

func encodePacketValue(buf []byte, v reflect.Value) (int, error) {
	rv := v
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	n := 0
	switch v.Kind() {
	default:
		return n, ErrUnhandledFieldType
	case reflect.Struct:
		if en, ok := rv.Interface().(encoder); ok {
			return en.Encode(buf)
		} else if en, ok := v.Interface().(encoder); ok {
			return en.Encode(buf)
		} else {
			for i := 0; i < v.NumField(); i++ {
				field := v.Field(i)
				n2, err := encodePacketValue(buf[n:], field)
				n += n2
				if err != nil {
					return n, err
				}
			}
		}
	case reflect.Bool:
		if v.Bool() {
			buf[n] = 1
		} else {
			buf[n] = 0
		}
		n++
	case reflect.Int32:
		binary.BigEndian.PutUint32(buf[n:n+4], uint32(v.Int()))
		n += 4
	case reflect.Int64:
		binary.BigEndian.PutUint64(buf[n:n+8], uint64(v.Int()))
		n += 8
	case reflect.String:
		str := v.String()
		binary.BigEndian.PutUint32(buf[n:n+4], uint32(len(str)))
		copy(buf[n+4:n+4+len(str)], []byte(str))
		n += 4 + len(str)
	case reflect.Slice:
		switch v.Type().Elem().Kind() {
		default:
			count := v.Len()
			startN := n
			n += 4
			for i := 0; i < count; i++ {
				n2, err := encodePacketValue(buf[n:], v.Index(i))
				n += n2
				if err != nil {
					return n, err
				}
			}
			binary.BigEndian.PutUint32(buf[startN:startN+4], uint32(count))
		case reflect.Uint8:
			if v.IsNil() {
				binary.BigEndian.PutUint32(buf[n:n+4], uint32(0xffffffff))
				n += 4
			} else {
				bytes := v.Bytes()
				binary.BigEndian.PutUint32(buf[n:n+4], uint32(len(bytes)))
				copy(buf[n+4:n+4+len(bytes)], bytes)
				n += 4 + len(bytes)
			}
		}
	}
	return n, nil
}

//根据操作码返回操作的request结构体
func requestStructForOp(op int32) interface{} {
	switch op {
	case opClose:
		return &closeRequest{}
	case opCreate:
		return &CreateRequest{}
	case opDelete:
		return &DeleteRequest{}
	case opExists:
		return &existsRequest{}
	case opGetAcl:
		return &getAclRequest{}
	case opGetChildren:
		return &getChildrenRequest{}
	case opGetChildren2:
		return &getChildren2Request{}
	case opGetData:
		return &getDataRequest{}
	case opPing:
		return &pingRequest{}
	case opSetAcl:
		return &setAclRequest{}
	case opSetData:
		return &SetDataRequest{}
	case opSync:
		return &syncRequest{}
	case opSetAuth:
		return &setAuthRequest{}
	case opCheck:
		return &CheckVersionRequest{}
	}
	return nil
}
