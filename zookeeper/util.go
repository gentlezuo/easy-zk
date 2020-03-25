package zookeeper

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"unicode/utf8"
)

// 发送请求前校验path
func validatePath(path string, isSequential bool) error {
	if path == "" {
		return ErrInvalidPath
	}

	if path[0] != '/' {
		return ErrInvalidPath
	}

	n := len(path)
	if n == 1 {
		return nil
	}

	if !isSequential && path[n-1] == '/' {
		return ErrInvalidPath
	}

	for i, w := 1, 0; i < n; i += w {
		r, width := utf8.DecodeRuneInString(path[i:])
		switch {
		case r == '\u0000':
			return ErrInvalidPath
		case r == '/':
			last, _ := utf8.DecodeLastRuneInString(path[:i])
			if last == '/' {
				return ErrInvalidPath
			}
		case r == '.':
			last, lastWidth := utf8.DecodeLastRuneInString(path[:i])

			// Check for double dot
			if last == '.' {
				last, _ = utf8.DecodeLastRuneInString(path[:i-lastWidth])
			}

			if last == '/' {
				if i+1 == n {
					return ErrInvalidPath
				}

				next, _ := utf8.DecodeRuneInString(path[i+w:])
				if next == '/' {
					return ErrInvalidPath
				}
			}
		case r >= '\u0000' && r <= '\u001f',
			r >= '\u007f' && r <= '\u009f',
			r >= '\uf000' && r <= '\uf8ff',
			r >= '\ufff0' && r < '\uffff':
			return ErrInvalidPath
		}
		w = width
	}
	return nil
}

func FormatServers(servers []string) []string {
	for i := range servers {
		if !strings.Contains(servers[i], ":") {
			servers[i] = servers[i] + ":" + strconv.Itoa(ZKDefaultPort)
		}
	}
	return servers
}

func WorldACL(perms int32) []ACL {
	return []ACL{{perms, "world", "anyone"}}
}

func DigestACL(perms int32, user, password string) []ACL {
	userPass := []byte(fmt.Sprintf("%s:%s", user, password))
	h := sha1.New()
	if n, err := h.Write(userPass); err != nil || n != len(userPass) {
		panic("SHA1 failed")
	}
	digest := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return []ACL{{perms, "digest", fmt.Sprintf("%s:%s", user, digest)}}
}

func stringShuffle(s []string) {
	for i := len(s) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}
