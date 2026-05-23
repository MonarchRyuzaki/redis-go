package server

import (
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/MonarchRyuzaki/redis-go/config"
	"github.com/MonarchRyuzaki/redis-go/core"
)

func readCommand(c net.Conn) (*core.RedisCmd, error) {
	// TODO: Max read in one shot is 512 bytes
	// To allow input > 512 bytes, then repeated read until
	// we get EOF or designated delimiter
	var buf []byte = make([]byte, 512)
	n, err := c.Read(buf[:])
	if err != nil {
		return nil, err
	}
	tokens, _, err := core.Unmarshal(buf[:n])
	if err != nil {
		return nil, err
	}

	args := make([]string, 0, len(tokens.Array)-1)
	for _, v := range tokens.Array[1:] {
		args = append(args, v.Bulk)
	}

	return &core.RedisCmd{
		Cmd:  strings.ToUpper(tokens.Array[0].Bulk),
		Args: args,
	}, nil
}

func respond(cmd *core.RedisCmd, c net.Conn) error {
	if err := core.EvalAndRespond(cmd, c); err != nil {
		c.Write(core.Marshal(core.Value{Type: core.ERROR, Str: err.Error()}))
	}
	return nil
}

func RunSyncTCPServer() {
	log.Println("starting a synchronous TCP server on", config.Host, config.Port)

	var con_clients int = 0

	// listening to the configured host:port
	lsnr, err := net.Listen("tcp", config.Host+":"+strconv.Itoa(config.Port))
	if err != nil {
		panic(err)
	}

	for {
		// blocking call: waiting for the new client to connect
		c, err := lsnr.Accept()
		if err != nil {
			panic(err)
		}

		// increment the number of concurrent clients
		con_clients += 1
		log.Println("client connected with address:", c.RemoteAddr(), "concurrent clients", con_clients)

		for {
			// over the socket, continuously read the command and print it out
			cmd, err := readCommand(c)
			if err != nil {
				c.Close()
				con_clients -= 1
				log.Println("client disconnected", c.RemoteAddr(), "concurrent clients", con_clients)
				if err == io.EOF {
					break
				}
				log.Println("err", err)
			}
			respond(cmd, c)
		}
	}
}
