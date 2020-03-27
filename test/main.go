package main

import (
	"fmt"
	"github.com/gentlezuo/easy-zk/zookeeper"
	"strings"
	"time"
)

func main() {
	/*s:="好"
	fmt.Println(len(s))
	fmt.Print(len([]rune(s)))
	strconv.Itoa(4)*/
	s:="aaa"
	fmt.Println(len(strings.Split(s,",")))
	s="bbb,"
	fmt.Println(len(strings.Split(s,",")))
	time.Sleep(time.Hour)

	conn, err := zookeeper.Connect([]string{"182.92.99.111:2181"}, time.Second*5)
	if err != nil {
		fmt.Println(err)
		return
	}
	data, _, err := conn.Get("/test")
	if err != nil {
		fmt.Println(err)
		return
	}
	_,err=conn.Create("/test_time",[]byte("aaa"),zookeeper.ModeEphemeral,zookeeper.WorldACL(zookeeper.PermissionAll))
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(data))
	time.Sleep(time.Hour)
}
