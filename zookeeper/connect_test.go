package zookeeper

import (
	"fmt"
	"testing"
	"time"
)

const ZK_ADDRESS="182.92.99.111:2181"

func TestConnect(t *testing.T) {
	addr := []string{ZK_ADDRESS}
	conn, err := Connect(addr, time.Second*2)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(conn.server)

	defer conn.Close()
}

func TestConn_Create(t *testing.T) {
	addr := []string{ZK_ADDRESS}
	conn, err := Connect(addr, time.Second*2)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()

	/*p, err := conn.Create("/a", []byte("aaa"), ModeEphemeralSequence, WorldACL(PermissionAll))
	if err != nil {
		t.Error(err)
	}
	if !strings.HasPrefix(p, "/a") {
		t.Error("创建临时序列节点失败")
	}
	p2, _ := conn.Create("/b", []byte("bbb"), ModeEphemeral, WorldACL(PermissionAll))
	if "/b" != p2 {
		t.Error("创建临时无序列节点失败")
	}
	p3, _ := conn.Create("/c", []byte("ccc"), ModePersistent, WorldACL(PermissionAll))
	if "/c" != p3 {
		t.Error("创建永久无序列节点失败")
	}
	p4, _ := conn.Create("/d", []byte("ddd"), ModePersistentSequence, WorldACL(PermissionAll))
	if !strings.HasPrefix(p4, "/d") {
		t.Error("创建永久序列节点失败")
	}*/
	_, err = conn.Create("/b", []byte("aaa"), 10, WorldACL(PermissionAll))
	if err != nil {
		t.Error(err)
	}
	/*if !strings.HasPrefix(p, "/a") {
		t.Error("创建临时序列节点失败")
	}
	p2, _ := conn.Create("/b", []byte("bbb"), ModeEphemeral, WorldACL(PermissionAll))
	if "/b" != p2 {
		t.Error("创建临时无序列节点失败")
	}
	p3, _ := conn.Create("/c", []byte("ccc"), ModePersistent, WorldACL(PermissionAll))
	if "/c" != p3 {
		t.Error("创建永久无序列节点失败")
	}
	p4, _ := conn.Create("/d", []byte("ddd"), ModePersistentSequence, WorldACL(PermissionAll))
	if !strings.HasPrefix(p4, "/d") {
		t.Error("创建永久序列节点失败")
	}*/
	time.Sleep(time.Second * 2)

}

func TestConn_Get(t *testing.T) {
	addr := []string{ZK_ADDRESS}
	conn, err := Connect(addr, time.Second*2)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()

	data, stat, err := conn.Get("/c")
	if err != nil {
		t.Error("get /c 失败", err)
	}
	fmt.Println(ZkStateStringFormat(stat))
	fmt.Println(string(data))
	data, stat, err = conn.Get("/test/a")
	if err != nil {
		t.Error("get /test/a 失败", err)
	}
	fmt.Println(ZkStateStringFormat(stat))
	fmt.Println(string(data))
}

func TestConn_Exists(t *testing.T) {
	addr := []string{ZK_ADDRESS}
	conn, err := Connect(addr, time.Second*2)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()

	conn.Create("/test_exist", []byte("aaa"), ModeEphemeral, WorldACL(PermissionAll))
	res, stat, err := conn.Exists("/test_exist")
	if err != nil || !res {
		t.Error(err)
	}
	fmt.Println(ZkStateStringFormat(stat))
	fmt.Println(res)

	res, _, err = conn.Exists("/test_exiaaaaaaaaa")
	if err != nil || res {
		t.Error(err)
	}
	fmt.Println(res)
}

func TestConn_Children(t *testing.T) {
	addr := []string{ZK_ADDRESS}
	conn, err := Connect(addr, time.Second*2)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()

	path := "/test"
	children, _, err := conn.Children(path)
	if err != nil {
		t.Error("get children 失败", err)
	}
	fmt.Println(children)
}

func TestConn_Delete(t *testing.T) {
	addr := []string{ZK_ADDRESS}
	conn, err := Connect(addr, time.Second*2)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()

	_, err = conn.Create("/test_del", []byte("asd"), ModeEphemeral, WorldACL(PermissionAll))
	if err != nil {
		t.Error("create /test_del 失败", err)
	}
	_, stat, _ := conn.Get("/test_del")
	err = conn.Delete("/test_del", stat.Version)
	if err != nil {
		t.Error("del 失败", err)
	}
	//time.Sleep(time.Second*10)
}

func TestConn_Close(t *testing.T) {
	addr := []string{ZK_ADDRESS}
	conn, err := Connect(addr, time.Second*2)
	if err != nil {
		t.Error(err)
	}
	conn.Close()
}

func TestConn_GetACL(t *testing.T) {
	addr := []string{ZK_ADDRESS}
	conn, err := Connect(addr, time.Second*2)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()

	acls, _, err := conn.GetACL("/test")
	if err != nil {
		t.Error("get acl from test 失败", err)
	}
	fmt.Println(acls)

}

func TestConn_AddAuth(t *testing.T) {
	addr := []string{ZK_ADDRESS}
	conn, err := Connect(addr, time.Second*2)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()

	//添加认证
	acls := DigestACL(PermissionAll, "zzx", "passwd")
	_, err = conn.Create("/addauth_test", []byte("adf"), ModeEphemeral, acls)
	if err != nil {
		t.Error("create 失败")
	}
	err = conn.AddAuth("digest", []byte("zzx:passwd"))
	if err != nil {
		t.Error("add auth 失败", err)
	}

	data, _, err := conn.GetACL("/addauth_test")
	if err != nil {
		t.Error("get acls 失败", err)
	}
	fmt.Println(data)

	//另一个conn，无auth
	conn1, err := Connect(addr, time.Second*2)
	if err != nil {
		t.Error(err)
	}
	defer conn1.Close()
	_, _, err = conn1.Get("/addauth_test")
	if err == nil {
		t.Error("add auth 失败")
	}
	fmt.Println(err)

	//另一个conn，有auth
	conn2, _ := Connect(addr, time.Second*2)
	err = conn2.AddAuth("digest", []byte("zzx:passwd"))
	if err != nil {
		t.Error("conn2 add auth 失败", err)
	}
	defer conn2.Close()
	_, _, err = conn2.Get("/addauth_test")
	if err != nil {
		t.Error("add auth 失败", err)
	}

	time.Sleep(time.Second * 1)
}

func TestConn_SessionID(t *testing.T) {
	addr := []string{ZK_ADDRESS}
	conn, err := Connect(addr, time.Second*2)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()
	sid := conn.SessionID()
	fmt.Println(sid)
}

func TestConn_Server(t *testing.T) {
	addr := []string{ZK_ADDRESS}
	conn, err := Connect(addr, time.Second*2)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()
	fmt.Println(conn.Server())
}

func ZkStateString(s *Stat) string {
	return fmt.Sprintf("Czxid:%d, Mzxid: %d, Ctime: %d, Mtime: %d, Version: %d, Cversion: %d, Aversion: %d, EphemeralOwner: %d, DataLength: %d, NumChildren: %d, Pzxid: %d",
		s.Czxid, s.Mzxid, s.Ctime, s.Mtime, s.Version, s.Cversion, s.Aversion, s.EphemeralOwner, s.DataLength, s.NumChildren, s.Pzxid)
}

func ZkStateStringFormat(s *Stat) string {
	return fmt.Sprintf("Czxid:%d\nMzxid: %d\nCtime: %d\nMtime: %d\nVersion: %d\nCversion: %d\nAversion: %d\nEphemeralOwner: %d\nDataLength: %d\nNumChildren: %d\nPzxid: %d\n",
		s.Czxid, s.Mzxid, s.Ctime, s.Mtime, s.Version, s.Cversion, s.Aversion, s.EphemeralOwner, s.DataLength, s.NumChildren, s.Pzxid)
}
