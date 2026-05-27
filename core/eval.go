package core

import (
	"bytes"
	"fmt"
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
		case "INCR":
			buf.Write(evalINCR(cmd.Args))
		case "INFO":
			buf.Write(evalINFO(cmd.Args))
		case "CLIENT":
			buf.Write(evalCLIENT(cmd.Args))
		case "LATENCY":
			buf.Write(evalLATENCY(cmd.Args))
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
	oType, oEnc := deduceTypeEncoding(value)

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

	Put(key, NewObj(value, exDurationMs, oType, oEnc))
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

func evalINCR(args []string) []byte {
	if len(args) != 1 {
		return Marshal(Value{Type: ERROR, Str: "ERR wrong number of arguments for 'incr' command"})
	}

	var key string = args[0]
	obj := Get(key)
	if obj == nil {
		obj = NewObj("0", -1, OBJ_TYPE_STRING, OBJ_ENCODING_INT)
		Put(key, obj)
	}

	if err := assertType(obj.TypeEncoding, OBJ_TYPE_STRING); err != nil {
		return Marshal(Value{Type: ERROR, Str: err.Error()})
	}

	if err := assertEncoding(obj.TypeEncoding, OBJ_ENCODING_INT); err != nil {
		return Marshal(Value{Type: ERROR, Str: err.Error()})
	}

	i, _ := strconv.ParseInt(obj.Value.(string), 10, 64)
	i++
	obj.Value = strconv.FormatInt(i, 10)

	return Marshal(Value{Type: INTEGER, Num: int(i)})
}

func evalINFO(args []string) []byte {
	var info []byte
	buf := bytes.NewBuffer(info)
	buf.WriteString("# Keyspace\r\n")
	for i := range KeyspaceStat {
		buf.WriteString(fmt.Sprintf("db%d:keys=%d,expires=0,avg_ttl=0\r\n", i, KeyspaceStat[i]["keys"]))
	}
	return Marshal(Value{Type: BULK, Bulk: buf.String()})
}

func evalCLIENT(args []string) []byte {
	return Marshal(Value{Type: STRING, Str: "OK"})
}

func evalLATENCY(args []string) []byte {
	return Marshal(Value{Type: ARRAY, Array: make([]Value, 0)})
}
