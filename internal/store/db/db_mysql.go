//go:build !agent

package db

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func init() {
	RegisterDialector(DBTypeMySQL, func(dsn string) gorm.Dialector {
		return mysql.Open(dsn)
	})
}
