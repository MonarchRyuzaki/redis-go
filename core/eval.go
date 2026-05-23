package core

import (
	"errors"
	"log"
	"net"
)

func EvalAndRespond(cmd *RedisCmd, c net.Conn) error {
	log.Println("command: ", cmd)
	switch cmd.Cmd {
	case "PING":
		return evalPing(cmd.Args, c)
	default:
		return evalPing(cmd.Args, c)
	}
}

func evalPing(args []string, c net.Conn) error {
	var b []byte

	if len(args) >= 2 {
		return errors.New("ERR wrong number of arguments for 'ping' command")
	}

	if len(args) == 0{
		b = Marshal(Value{Type: STRING, Str: "PONG"})
		} else {
		b = Marshal(Value{Type: STRING, Str: args[0]})
	}

	_, err := c.Write(b)
	return err
}