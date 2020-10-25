package interop

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

var ErrInterrupted = errors.New("interrupted")

type HandlerFunc func(method string, result string)

type Interop struct {
	conn   net.Conn
	mu     sync.Mutex
	buffer []byte
}

func New(conn net.Conn) *Interop {
	return &Interop{
		conn:   conn,
		buffer: make([]byte, 65535),
	}
}

func (i *Interop) Call(method string, params ...string) (string, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.call(method, params...)
}

func (i *Interop) Subscribe(handler HandlerFunc) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	ch := make(chan interface{}, 1)
	go i.produce(ch)

	for {
		select {
		case v, ok := <-ch:
			if !ok {
				return ErrInterrupted
			}
			if err, ok := v.(error); ok {
				return err
			}
			res, ok := v.(string)
			if !ok {
				continue
			}

			tokens := strings.SplitN(res, " ", 2)
			method := tokens[0]
			result := ""
			if len(tokens) >= 2 {
				result = tokens[1]
			}
			handler(method, result)
		case <-ticker.C:
			// Send keepalive to peer
			if _, err := i.call("keepalive"); err != nil {
				return err
			}
		}
	}
}

func (i *Interop) produce(ch chan<- interface{}) {
	defer close(ch)
	for {
		n, err := i.conn.Read(i.buffer)
		if err != nil {
			ch <- err
			return
		}
		ch <- string(i.buffer[:n])
	}
}

func (i *Interop) call(method string, params ...string) (string, error) {
	cmd := method
	if len(params) > 0 {
		cmd = fmt.Sprintf("%s %s", method, strings.Join(params, " "))
	}

	if _, err := i.conn.Write([]byte(cmd)); err != nil {
		return "", err
	}
	n, err := i.conn.Read(i.buffer)
	if err != nil {
		return "", err
	}

	resp := string(i.buffer[:n])
	if strings.HasPrefix(resp, "error") {
		message := strings.TrimSpace(resp[5:])
		return "", errors.New(message)
	} else if strings.HasPrefix(resp, "ok") {
		result := strings.TrimSpace(resp[2:])
		return result, nil
	}
	return "", fmt.Errorf("unknown response: %s", resp)
}
