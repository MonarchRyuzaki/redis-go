package server

import (
	"io"
	"strings"

	"github.com/MonarchRyuzaki/redis-go/core"
)

func readCommand(c io.ReadWriter) (core.RedisCmds, error) {
	// TODO: Max read in one shot is 512 bytes
	// To allow input > 512 bytes, then repeated read until
	// we get EOF or designated delimiter
	var buf []byte = make([]byte, 512)
	n, err := c.Read(buf[:])
	if err != nil {
		return nil, err
	}
	tokensArray, _, err := core.UnmarshalMany(buf[:n])
	if err != nil {
		return nil, err
	}

	var args []string

	redisCmds := make(core.RedisCmds, 0, len(tokensArray))
	for i := 0; i < len(tokensArray); i++ {
		if len(tokensArray[i].Array) == 0 {
			continue
		}
		args = make([]string, 0, len(tokensArray[i].Array))
		for _, v := range tokensArray[i].Array[1:] {
			args = append(args, v.Bulk)

		}
		redisCmds = append(redisCmds, &core.RedisCmd{
			Cmd:  strings.ToUpper(tokensArray[i].Array[0].Bulk),
			Args: args,
		})
	}

	return redisCmds, nil
}

func respond(cmds core.RedisCmds, c *core.Client) {
	core.EvalAndRespond(cmds, c)
}

// func RunSyncTCPServer() {
// 	log.Println("starting a synchronous TCP server on", config.Host, config.Port)

// 	var con_clients int = 0

// 	// listening to the configured host:port
// 	lsnr, err := net.Listen("tcp", config.Host+":"+strconv.Itoa(config.Port))
// 	if err != nil {
// 		log.Println(err)
// 		return
// 	}

// 	for {
// 		// blocking call: waiting for the new client to connect
// 		c, err := lsnr.Accept()
// 		if err != nil {
// 			log.Println(err)
// 			return
// 		}

// 		// increment the number of concurrent clients
// 		con_clients += 1
// 		log.Println("client connected with address:", c.RemoteAddr(), "concurrent clients", con_clients)

// 		for {
// 			// over the socket, continuously read the command and print it out
// 			cmds, err := readCommand(c)
// 			if err != nil {
// 				c.Close()
// 				con_clients -= 1
// 				log.Println("client disconnected", c.RemoteAddr(), "concurrent clients", con_clients)
// 				if err == io.EOF {
// 					break
// 				}
// 				log.Println("err", err)
// 			}
// 			respond(cmds, c)
// 		}
// 	}
// }
