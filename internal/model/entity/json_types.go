package entity

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Int64List stores []int64 as JSON text in the database while exposing an array in the API.
type Int64List []int64

func (l Int64List) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]int64(l))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (l *Int64List) Scan(value interface{}) error {
	if l == nil {
		return nil
	}
	if value == nil {
		*l = Int64List{}
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("unsupported Int64List scan type %T", value)
	}
	if len(raw) == 0 {
		*l = Int64List{}
		return nil
	}
	var items []int64
	if err := json.Unmarshal(raw, &items); err != nil {
		return err
	}
	*l = Int64List(items)
	return nil
}
