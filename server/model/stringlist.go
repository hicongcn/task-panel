package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// StringList 以 JSON 文本形式存储的字符串数组(用于标签等)。
// GORM 存 text 列,JSON 序列化时输出数组。
type StringList []string

// Value 实现 driver.Valuer(GORM 写库)。
// 注意:必须返回 string 而非 []byte——纯 Go 驱动(modernc/glebarez)会把 []byte
// 按 BLOB 绑定存储,导致 TEXT 列的存储类别变成 blob,LIKE 等文本匹配全部失效。
func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan 实现 sql.Scanner(GORM 读库)。
func (s *StringList) Scan(value interface{}) error {
	if value == nil {
		*s = StringList{}
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("StringList: 不支持的数据库类型")
	}
	var list []string
	if err := json.Unmarshal(b, &list); err != nil {
		return err
	}
	*s = StringList(list)
	return nil
}
