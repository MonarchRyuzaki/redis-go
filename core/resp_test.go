package core_test

import (
	"fmt"
	"testing"

	"github.com/MonarchRyuzaki/redis-go/core"
)

func TestSimpleStringDecode(t *testing.T) {
	cases := map[string]string{
		"+OK\r\n": "OK",
	}
	for k, v := range cases {
		value, _, _ := core.Unmarshal([]byte(k))
		if v != value.Str {
			t.Fail()
		}
	}
}

func TestError(t *testing.T) {
	cases := map[string]string{
		"-Error message\r\n": "Error message",
	}
	for k, v := range cases {
		value, _, _ := core.Unmarshal([]byte(k))
		if v != value.Str {
			t.Fail()
		}
	}
}

func TestInt64(t *testing.T) {
	cases := map[string]int64{
		":0\r\n":    0,
		":1000\r\n": 1000,
	}
	for k, v := range cases {
		value, _, _ := core.Unmarshal([]byte(k))
		if v != int64(value.Num) {
			t.Fail()
		}
	}
}

func TestBulkStringDecode(t *testing.T) {
	cases := map[string]string{
		"$5\r\nhello\r\n": "hello",
		"$0\r\n\r\n":      "",
	}
	for k, v := range cases {
		value, _, _ := core.Unmarshal([]byte(k))
		if v != value.Bulk {
			t.Fail()
		}
	}
}

func TestArrayDecode(t *testing.T) {
	cases := map[string][]interface{}{
		"*0\r\n":                                                   {},
		"*2\r\n$5\r\nhello\r\n$5\r\nworld\r\n":                     {"hello", "world"},
		"*3\r\n:1\r\n:2\r\n:3\r\n":                                 {int64(1), int64(2), int64(3)},
		"*5\r\n:1\r\n:2\r\n:3\r\n:4\r\n$5\r\nhello\r\n":            {int64(1), int64(2), int64(3), int64(4), "hello"},
		"*2\r\n*3\r\n:1\r\n:2\r\n:3\r\n*2\r\n+Hello\r\n-World\r\n": {[]interface{}{int64(1), int64(2), int64(3)}, []interface{}{"Hello", "World"}},
	}

	var toVal func(core.Value) interface{}
	toVal = func(v core.Value) interface{} {
		switch v.Type {
		case core.STRING, core.ERROR:
			return v.Str
		case core.INTEGER:
			return int64(v.Num)
		case core.BULK:
			return v.Bulk
		case core.ARRAY:
			var res []interface{}
			for _, val := range v.Array {
				res = append(res, toVal(val))
			}
			return res
		}
		return nil
	}

	for k, v := range cases {
		value, _, _ := core.Unmarshal([]byte(k))
		array := value.Array
		if len(array) != len(v) {
			t.Errorf("key %s: expected length %d, got %d", k, len(v), len(array))
			continue
		}
		for i := range array {
			got := toVal(array[i])
			if fmt.Sprintf("%v", v[i]) != fmt.Sprintf("%v", got) {
				t.Errorf("key %s, index %d: expected %v, got %v", k, i, v[i], got)
			}
		}
	}
}
