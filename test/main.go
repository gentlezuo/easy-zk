package main

import (
	"fmt"
	"github.com/gentlezuo/easy-zk/zookeeper"
	"time"
)

func main() {
	/*s:="好"
	fmt.Println(len(s))
	fmt.Print(len([]rune(s)))
	strconv.Itoa(4)*/

	conn, err := zookeeper.Connect([]string{"182.92.99.111:2181"}, time.Second*1000)
	if err != nil {
		fmt.Println(err)
		return
	}
	data, _, err := conn.Get("/test")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(data))

}
