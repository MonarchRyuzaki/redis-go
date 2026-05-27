package core

import (
	"bytes"
	"io"
	"log"
	"strconv"
	"time"
)

func EvalAndRespond(cmds RedisCmds, c io.ReadWriter) {

	var response []byte
	buf := bytes.NewBuffer(response)

	for _, cmd := range cmds {
		log.Println("command: ", cmd)
		switch cmd.Cmd {
		case "PING":
			buf.Write(evalPING(cmd.Args))
		case "SET":
			buf.Write(evalSET(cmd.Args))
		case "GET":
			buf.Write(evalGET(cmd.Args))
		case "TTL":
			buf.Write(evalTTL(cmd.Args))
		case "DEL":
			buf.Write(evalDEL(cmd.Args))
		case "EXPIRE":
			buf.Write(evalEXPIRE(cmd.Args))
		case "BGREWRITEAOF":
			buf.Write(evalBGREWRITEAOF(cmd.Args))
		default:
			buf.Write(evalPING(cmd.Args))
		}
	}

	c.Write(buf.Bytes())
}

func evalPING(args []string) []byte {
	if len(args) >= 2 {
		return Marshal(Value{Type: ERROR, Str: "ERR wrong number of arguments for 'ping' command"})
	}

	if len(args) == 0 {
		return Marshal(Value{Type: STRING, Str: "PONG"})
	} else {
		return Marshal(Value{Type: STRING, Str: args[0]})
	}
}

func evalSET(args []string) []byte {
	if len(args) <= 1 {
		return Marshal(Value{Type: ERROR, Str: "ERR wrong number of arguments for 'set' command"})
	}

	var key, value string
	var exDurationMs int64 = -1

	key, value = args[0], args[1]

	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "EX", "ex":
			i++
			if i == len(args) {
				return Marshal(Value{Type: ERROR, Str: "ERR syntax error"})
			}

			exDurationSec, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return Marshal(Value{Type: ERROR, Str: "ERR value is not an integer or out of range"})
			}

			exDurationMs = exDurationSec * 1000
		default:
			return Marshal(Value{Type: ERROR, Str: "ERR Syntax error"})
		}
	}

	Put(key, NewObj(value, exDurationMs))
	return Marshal(Value{Type: STRING, Str: "OK"})
}

func evalGET(args []string) []byte {
	if len(args) != 1 {
		return Marshal(Value{Type: ERROR, Str: "ERR wrong number of arguments for 'get' command"})
	}
	var key string = args[0]

	obj := Get(key)

	if obj == nil {
		return Marshal(Value{Type: NULL_BULK})
	}

	if obj.ExpiresAt != -1 && obj.ExpiresAt <= time.Now().UnixMilli() {
		return Marshal(Value{Type: NULL_BULK})
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

	return Marshal(Value{Type: BULK, Bulk: bulk})
}

func evalTTL(args []string) []byte {
	if len(args) != 1 {
		return Marshal(Value{Type: ERROR, Str: "ERR wrong number of arguments for 'ttl' command"})
	}

	var key string = args[0]

	obj := Get(key)

	if obj == nil {
		return Marshal(Value{Type: INTEGER, Num: -2})
	}

	if obj.ExpiresAt == -1 {
		return Marshal(Value{Type: INTEGER, Num: -1})

	}

	durationMs := obj.ExpiresAt - time.Now().UnixMilli()

	if durationMs < 0 {
		return Marshal(Value{Type: INTEGER, Num: -2})

	}

	return Marshal(Value{Type: INTEGER, Num: int(durationMs / 1000)})

}

func evalDEL(args []string) []byte {
	var countDeleted int = 0
	for _, key := range args {
		if ok := Del(key); ok {
			countDeleted++
		}
	}

	return Marshal(Value{Type: INTEGER, Num: countDeleted})
}

func evalEXPIRE(args []string) []byte {
	if len(args) <= 1 {
		Marshal(Value{Type: ERROR, Str: "ERR wrong number of arguments for 'expire' command"})
	}

	var key string = args[0]
	exDurationSec, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		Marshal(Value{Type: ERROR, Str: "ERR value is not an integer or out of range"})
	}

	obj := Get(key)

	// 0 if the timeout was not set. e.g. key doesn't exist, or operation skipped due to the provided arguments
	if obj == nil {
		return Marshal(Value{Type: INTEGER, Num: 0})

	}

	obj.ExpiresAt = time.Now().UnixMilli() + exDurationSec*1000

	// 1 if the timeout was set.
	return Marshal(Value{Type: INTEGER, Num: 1})
}

// TODO: Make it async by forking a new process
func evalBGREWRITEAOF(args []string) []byte {
	DumpAllAOF()
	return Marshal(Value{Type: STRING, Str: "OK"})
}
