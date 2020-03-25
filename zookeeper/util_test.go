package zookeeper

import (
	"fmt"
	"testing"
)

func TestValidatePath(t *testing.T) {
	tt := []struct {
		path  string
		seq   bool
		valid bool
	}{
		{"/this is / a valid/path", false, true},
		{"/", false, true},
		{"", false, false},
		{"not/valid", false, false},
		{"/ends/with/slash/", false, false},
		{"/sequential/", true, true},
		{"/test\u0000", false, false},
		{"/double//slash", false, false},
		{"/single/./period", false, false},
		{"/double/../period", false, false},
		{"/double/..ok/period", false, true},
		{"/double/alsook../period", false, true},
		{"/double/period/at/end/..", false, false},
		{"/name/with.period", false, true},
		{"/test\u0001", false, false},
		{"/test\u001f", false, false},
		{"/test\u0020", false, true}, // first allowable
		{"/test\u007e", false, true}, // last valid ascii
		{"/test\u007f", false, false},
		{"/test\u009f", false, false},
		{"/test\uf8ff", false, false},
		{"/test\uffef", false, true},
		{"/test\ufff0", false, false},
	}

	for _, tc := range tt {
		err := validatePath(tc.path, tc.seq)
		if (err != nil) == tc.valid {
			t.Errorf("failed to validate path %q", tc.path)
		}
	}
}

func TestFormatZKServers(t *testing.T) {
	src := []string{"192.156.23.9", "192.89.34.3:2181", "192.56.45.9:3333"}
	dst := []string{"192.156.23.9:2181", "192.89.34.3:2181", "192.56.45.9:3333"}
	for k, v := range FormatServers(src) {
		if dst[k] != v {
			t.Errorf("%v should equal %v", v, dst[k])
		}
	}
}

func TestStringShuffle(t *testing.T) {
	src := []string{"a", "b", "c", "d", "e"}
	stringShuffle(src)
	for _, v := range src {
		fmt.Print(v + " ")
	}
}

func ExampleHello() {
	fmt.Println("hello")
	// Output: hello
}
