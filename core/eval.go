package core

import (
	"bytes"
	"fmt"
	"log"
	"strconv"
	"time"
)

var txnCommands map[string]bool

func init() {
	txnCommands = map[string]bool{"EXEC": true, "DISCARD": true}
}

func EvalAndRespond(cmds RedisCmds, c *Client) {

	var response []byte
	buf := bytes.NewBuffer(response)

	for _, cmd := range cmds {
		log.Println("command: ", cmd)
		// if txn is not in progress, then we can simply
		// execute the command and add the response to the buffer
		if !c.isTxn {
			executeCommandToBuffer(cmd, buf, c)
			continue
		}

		// if the txn is in progress, we enqueue the command
		// and add the QUEUED response to the buffer
		if !txnCommands[cmd.Cmd] {
			// if the command is queuable the enqueu
			c.TxnQueue(cmd)
			buf.Write(Marshal(Value{Type: STRING, Str: "QUEUED"}))
		} else {
			// if txn is active and the command is non-queuable
			// ex: EXEC, DISCARD
			// we execute the command and gather the response in buffer
			executeCommandToBuffer(cmd, buf, c)
		}
	}

	c.Write(buf.Bytes())
}

func executeCommand(cmd *RedisCmd, c *Client) []byte {
	switch cmd.Cmd {
	case "PING":
		return evalPING(cmd.Args)
	case "SET":
		return evalSET(cmd.Args)
	case "GET":
		return evalGET(cmd.Args)
	case "TTL":
		return evalTTL(cmd.Args)
	case "DEL":
		return evalDEL(cmd.Args)
	case "EXPIRE":
		return evalEXPIRE(cmd.Args)
	case "BGREWRITEAOF":
		return evalBGREWRITEAOF(cmd.Args)
	case "INCR":
		return evalINCR(cmd.Args)
	case "INFO":
		return evalINFO(cmd.Args)
	case "CLIENT":
		return evalCLIENT(cmd.Args)
	case "LATENCY":
		return evalLATENCY(cmd.Args)
	case "LRU":
		return evalLRU(cmd.Args)
	case "SLEEP":
		return evalSLEEP(cmd.Args)
	case "MULTI":
		c.TxnBegin()
		return evalMULTI(cmd.Args)
	case "EXEC":
		if !c.isTxn {
			return Marshal(Value{Type: STRING, Str: "ERR EXEC without MULTI"})
		}
		return c.TxnExec()
	case "DISCARD":
		if !c.isTxn {
			return Marshal(Value{Type: STRING, Str: "ERR DISCARD without MULTI"})
		}
		c.TxnDiscard()
		return Marshal(Value{Type: STRING, Str: "OK"})
	default:
		return evalPING(cmd.Args)
	}
}

func executeCommandToBuffer(cmd *RedisCmd, buf *bytes.Buffer, c *Client) {
	buf.Write(executeCommand(cmd, c))
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

	if hasExpired(obj) {
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

	exp, isExpirySet := getExpiry(obj)
	if !isExpirySet {
		return Marshal(Value{Type: INTEGER, Num: -1})

	}

	if exp < uint64(time.Now().UnixMilli()) {
		return Marshal(Value{Type: INTEGER, Num: -2})

	}

	// compute the time remaining for the key to expire and
	// return the RESP encoded form of it
	durationMs := exp - uint64(time.Now().UnixMilli())

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

	setExpiry(obj, exDurationSec*1000)

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

func evalLRU(args []string) []byte {
	evictAllKeysLRU()
	return Marshal(Value{Type: STRING, Str: "OK"})
}

func evalSLEEP(args []string) []byte {
	if len(args) != 1 {
		return Marshal(Value{Type: ERROR, Str: "ERR wrong number of arguments for 'SLEEP' command"})
	}

	durationSec, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return Marshal(Value{Type: ERROR, Str: "ERR value is not an integer or out of range"})
	}
	time.Sleep(time.Duration(durationSec) * time.Second)
	return Marshal(Value{Type: STRING, Str: "OK"})
}

func evalMULTI(args []string) []byte {
	return Marshal(Value{Type: STRING, Str: "OK"})
}
