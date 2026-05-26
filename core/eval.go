package core

import (
	"errors"
	"io"
	"log"
	"strconv"
	"time"
)

func EvalAndRespond(cmd *RedisCmd, c io.ReadWriter) error {
	log.Println("command: ", cmd)
	switch cmd.Cmd {
	case "PING":
		return evalPING(cmd.Args, c)
	case "SET":
		return evalSET(cmd.Args, c)
	case "GET":
		return evalGET(cmd.Args, c)
	case "TTL":
		return evalTTL(cmd.Args, c)
	default:
		return evalPING(cmd.Args, c)
	}
}

func evalPING(args []string, c io.ReadWriter) error {
	var b []byte

	if len(args) >= 2 {
		return errors.New("ERR wrong number of arguments for 'ping' command")
	}

	if len(args) == 0 {
		b = Marshal(Value{Type: STRING, Str: "PONG"})
	} else {
		b = Marshal(Value{Type: STRING, Str: args[0]})
	}

	_, err := c.Write(b)
	return err
}

func evalSET(args []string, c io.ReadWriter) error {
	if len(args) <= 1 {
		return errors.New("ERR wrong number of arguments for 'set' command")
	}

	var key, value string
	var exDurationMs int64 = -1

	key, value = args[0], args[1]

	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "EX", "ex":
			i++
			if i == len(args) {
				return errors.New("ERR syntax error")
			}

			exDurationSec, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return errors.New("ERR value is not an integer or out of range")
			}

			exDurationMs = exDurationSec * 1000
		default:
			return errors.New("ERR Syntax error")
		}
	}

	Put(key, NewObj(value, exDurationMs))
	c.Write(Marshal(Value{Type: STRING, Str: "OK"}))
	return nil
}

func evalGET(args []string, c io.ReadWriter) error {
	if len(args) != 1 {
		return errors.New("ERR wrong number of arguments for 'get' command")
	}
	var key string = args[0]

	obj := Get(key)

	if obj == nil {
		c.Write(Marshal(Value{Type: NULL_BULK}))
		return nil
	}

	if obj.ExpiresAt != -1 && obj.ExpiresAt <= time.Now().UnixMilli() {
		c.Write(Marshal(Value{Type: NULL_BULK}))
		return nil
	}

	var bulk string
	switch v := obj.Value.(type) {
	case string:
		bulk = v
	case []byte:
		bulk = string(v)
	default:
		bulk = ""
	}

	c.Write(Marshal(Value{Type: BULK, Bulk: bulk}))

	return nil
}

func evalTTL(args []string, c io.ReadWriter) error {
	if len(args) != 1 {
		return errors.New("ERR wrong number of arguments for 'ttl' command")
	}

	var key string = args[0]

	obj := Get(key)

	if obj == nil {
		c.Write(Marshal(Value{Type: INTEGER, Num: -2}))
		return nil
	}

	if obj.ExpiresAt == -1 {
		c.Write(Marshal(Value{Type: INTEGER, Num: -1}))
		return nil

	}

	durationMs := obj.ExpiresAt - time.Now().UnixMilli()

	if durationMs < 0 {
		c.Write(Marshal(Value{Type: INTEGER, Num: -2}))
		return nil
	}

	c.Write(Marshal(Value{Type: INTEGER, Num: int(durationMs / 1000)}))

	return nil
}
