//go:build !agent

package db

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func init() {
	RegisterDialector(DBTypePostgres, func(dsn string) gorm.Dialector {
		return postgres.Open(dsn)
	})
}
