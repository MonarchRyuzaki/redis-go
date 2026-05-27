package core

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/MonarchRyuzaki/redis-go/config"
)

// TODO: Support Expiration
// TODO: Support non-kv data structures
// TODO: Support sync write
func dumpKey(fp *os.File, key string, obj *Obj) {
	cmd := fmt.Sprintf("SET %s %s", key, obj.Value)
	tokens := strings.Split(cmd, " ")

	arr := make([]Value, 0, len(tokens))
	for _, t := range tokens {
		arr = append(arr, Value{Type: BULK, Bulk: t})
	}

	fp.Write(Marshal(Value{Type: ARRAY, Array: arr}))
}

// TODO: To to new and switch
func DumpAllAOF() {
	fp, err := os.OpenFile(config.AOFFile, os.O_CREATE|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		fmt.Print("error", err)
		return
	}
	log.Println("rewriting AOF file at", config.AOFFile)
	for k, obj := range store {
		dumpKey(fp, k, obj)
	}
	log.Println("AOF file rewrite complete")
}
