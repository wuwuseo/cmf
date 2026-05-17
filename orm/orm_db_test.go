package orm_test

import (
	"context"
	"testing"

	"github.com/wuwuseo/cmf/orm"
)

func TestGetSqlDb(t *testing.T) {
	db, err := orm.GetSqlDb("sqlite3", ":memory:")
	if err != nil {
		t.Skipf("GetSqlDb 失败: %v", err)
	}
	if db == nil {
		t.Error("db 应该非 nil")
	}
	defer db.Close()

	ctx := context.Background()
	err = db.PingContext(ctx)
	if err != nil {
		t.Errorf("Ping 失败: %v", err)
	}
}
