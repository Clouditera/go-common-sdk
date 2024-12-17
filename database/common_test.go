package database

import (
	"context"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestQueryKeyValueString(t *testing.T) {
	conds := NewQueryConds("id", "123").Add("name", "test")
	require.Equal(t, "[id=123 AND name=test]", fmt.Sprintf("%v", conds))
}

type MysqlContainer struct {
	container testcontainers.Container
	URI       string
}

// 创建 Mysql 测试容器
// nolint:errcheck
func CreateTestContainer(ctx context.Context) (*MysqlContainer, error) {
	// 容器配置
	req := testcontainers.ContainerRequest{
		Image:        "rd.clouditera.com/infra/mysql:5.7",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "gznPzkTJ8xEgGZO6",
			"MYSQL_DATABASE":      "biz",
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("mysqld: ready for connections"),
			wait.ForListeningPort("3306/tcp"),
		),
	}

	// 启动容器
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %v", err)
	}

	// 获取映射端口
	mappedPort, err := container.MappedPort(ctx, "3306")
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get mapped port: %v", err)
	}

	// 构建 Mysql URI
	uri := fmt.Sprintf("root:gznPzkTJ8xEgGZO6@tcp(localhost:%s)/biz?charset=utf8mb4&parseTime=True&loc=Local", mappedPort.Port())
	return &MysqlContainer{
		container: container,
		URI:       uri,
	}, nil
}

// 清理容器
// nolint:errcheck
func (r *MysqlContainer) Terminate(ctx context.Context) {
	r.container.Terminate(ctx)
}

func TestDatabaseOperation_Integration(t *testing.T) {
	logrus.SetLevel(logrus.InfoLevel)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	container, err := CreateTestContainer(ctx)
	require.NoError(t, err)
	defer container.Terminate(ctx)

	db, err := OpenDatabase(container.URI)
	require.NoError(t, err)

	type User struct {
		Id       int    `db:"id"`
		Name     string `db:"name"`
		Password string `db:"password"`
	}

	err = db.AutoMigrate(&User{})
	require.NoError(t, err)

	err = CreateItem(db, &User{Id: 1, Name: "test", Password: "123456"})
	require.NoError(t, err)

	user, found, err := GetItemById[User](db, "1")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, user.Name, "test")

	_, err = UpdateItemById[User](db, "1", map[string]interface{}{"name": "test2"})
	require.NoError(t, err)

	user, found, err = GetItemById[User](db, "1")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, user.Name, "test2")

	err = CreateItem(db, &User{Id: 2, Name: "test3", Password: "1234567"})
	require.NoError(t, err)

	user, found, err = GetItemEx[User](db, NewQueryConds("id", "2").Add("password", "1234567"))
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, user.Name, "test3")

	conds := NewQueryConds("id", "2")
	values := NewFieldValues("name", "test4")
	affected, err := UpdateItem[User](db, conds, values)
	require.NoError(t, err)
	require.Equal(t, affected, int64(1))

	user, found, err = GetItemById[User](db, "2")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, user.Name, "test4")

	count, err := GetItemsCountByConds[User](db, NewQueryConds("id", "2"))
	require.NoError(t, err)
	require.Equal(t, count, int64(1))

	conds = NewQueryConds("id != ?", "2")
	affected, err = UpdateItem[User](db, conds, map[string]interface{}{"name": "test5"})
	require.NoError(t, err)
	require.Equal(t, affected, int64(1))

	user, found, err = GetItemById[User](db, "1")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, user.Name, "test5")

	user, found, err = GetItemById[User](db, "2")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, user.Name, "test4")
}
