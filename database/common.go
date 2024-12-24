package database

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"source.gitlab.clouditera.com/clouditera/go-common-sdk/logutil"
)

// debug is a global variable to control whether to print debug logs.
var debug bool

func SetDebug(d bool) {
	debug = d
}

type QueryKeyValue struct {
	ColName  string
	ColValue interface{}
}

func (kv QueryKeyValue) String() string {
	return fmt.Sprintf("%s=%v", kv.ColName, kv.ColValue)
}

type QueryConds []QueryKeyValue

func NewQueryConds(colName string, colValue interface{}) QueryConds {
	return QueryConds{{ColName: colName, ColValue: colValue}}
}

func (qc QueryConds) Add(colName string, colValue interface{}) QueryConds {
	return append(qc, QueryKeyValue{ColName: colName, ColValue: colValue})
}

func (qc QueryConds) String() string {
	var strs []string
	for _, val := range qc {
		strs = append(strs, val.String())
	}

	return fmt.Sprintf("[%s]", strings.Join(strs, " AND "))
}

type FieldValues map[string]interface{}

func (fv FieldValues) String() string {
	var strs []string
	for key, val := range fv {
		strs = append(strs, fmt.Sprintf("%s=%v", key, val))
	}

	return fmt.Sprintf("{%s}", strings.Join(strs, ", "))
}

func (fv FieldValues) Add(key string, val interface{}) FieldValues {
	fv[key] = val
	return fv
}

func (fv FieldValues) AsMap() map[string]interface{} {
	return map[string]interface{}(fv)
}

func NewFieldValues(colName string, colValue interface{}) FieldValues {
	return FieldValues{colName: colValue}
}

type QueryOption struct {
	Offset   int    // 偏移量
	Limit    int    // 限制
	Order    string // 排序
	PageId   int    // 页码
	PageSize int    // 每页大小
}

func (qo QueryOption) GetOrder() string {
	if strings.HasPrefix(qo.Order, "-") {
		return fmt.Sprintf("%s %s", qo.Order[1:], "DESC")
	} else {
		return fmt.Sprintf("%s %s", qo.Order, "ASC")
	}
}

// Sql returns the sql string of the query option.
func (qo QueryOption) Sql() (sql string, err error) {
	if qo.Order != "" {
		sql += fmt.Sprintf("ORDER BY %s", qo.GetOrder())
	}

	if qo.Limit > 0 {
		if sql != "" {
			sql += " "
		}
		sql += fmt.Sprintf("LIMIT %d", qo.Limit)
	}

	if qo.Offset > 0 {
		if sql != "" {
			sql += " "
		}
		sql += fmt.Sprintf("OFFSET %d", qo.Offset)
	}

	if qo.PageSize > 0 {
		if qo.PageId > 0 {
			if sql != "" {
				sql += " "
			}
			sql += fmt.Sprintf("LIMIT %d OFFSET %d", qo.PageSize, (qo.PageId-1)*qo.PageSize)
		} else {
			return sql, logutil.LogError("WrapDB: invalid query option: %s", qo)
		}
	} else if qo.PageId < 0 {
		return sql, logutil.LogError("WrapDB: invalid query option: %s", qo)
	}

	return sql, nil
}

func (qo QueryOption) WrapDB(tx *gorm.DB) (*gorm.DB, error) {
	if qo.Offset > 0 {
		tx = tx.Offset(qo.Offset)
	}

	if qo.Limit > 0 {
		tx = tx.Limit(qo.Limit)
	}

	if qo.Order != "" {
		tx = tx.Order(qo.GetOrder())
	}

	if qo.PageSize > 0 {
		if qo.PageId > 0 {
			tx = tx.Offset((qo.PageId - 1) * qo.PageSize).Limit(qo.PageSize)
		} else {
			return tx, logutil.LogError("WrapDB: invalid query option: %v", qo)
		}
	} else if qo.PageId < 0 {
		return tx, logutil.LogError("WrapDB: invalid query option: %v", qo)
	}

	return tx, nil
}

func (qo QueryOption) String() (res string) {
	res = "{"
	if qo.Offset > 0 {
		res += fmt.Sprintf("offset=%d, ", qo.Offset)
	}

	if qo.Limit > 0 {
		res += fmt.Sprintf("limit=%d, ", qo.Limit)
	}

	if qo.Order != "" {
		res += fmt.Sprintf("order=%s, ", qo.Order)
	}

	if qo.PageId > 0 {
		res += fmt.Sprintf("pageId=%d, ", qo.PageId)
	}

	if qo.PageSize > 0 {
		res += fmt.Sprintf("pageSize=%d, ", qo.PageSize)
	}

	res += "}"
	return res
}

func OpenDatabase(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  logger.Warn,
				IgnoreRecordNotFoundError: !debug,
				Colorful:                  true,
			},
		),
	})
	if err != nil {
		return nil, logutil.LogError("gorm.Open: dsn=%s, err=%v", dsn, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, logutil.LogError("db.DB: err=%v", err)
	}

	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	return db, nil
}

func WrapDB(db *gorm.DB) *gorm.DB {
	if debug {
		return db.Debug()
	}

	return db
}

// CreateItem create item
func CreateItem[T any](db *gorm.DB, item T) (err error) {
	if err := WrapDB(db).Create(&item).Error; err != nil {
		return logutil.LogError("CreateItem: item=%v, err=%v", item, err)
	}

	return nil
}

// CreateItems create items. Batch size is 1000.
func CreateItems[T any](db *gorm.DB, items []T) (err error) {
	if len(items) == 0 {
		return nil
	}

	// FIXME: 可能需要根据实际情况调整batchSize
	const batchSize = 1000

	// For small batches, create directly
	if len(items) <= batchSize {
		if err := WrapDB(db).Create(&items).Error; err != nil {
			return logutil.LogError("CreateItems: items=%v, err=%v", items, err)
		}
		return nil
	}

	// For large batches, create in chunks
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}

		var batchItems = items[i:end]
		if err := WrapDB(db).Create(&batchItems).Error; err != nil {
			return logutil.LogError("CreateItems: batch=%v, err=%v", batchItems, err)
		}
	}

	return nil
}

// UpdateItem update item with fields values. It returns the number of rows affected and an error if any.
func UpdateItem[T any](db *gorm.DB, conds QueryConds, values FieldValues) (affected int64, err error) {
	var item T
	tx := WrapDB(db).Model(&item).Where("TRUE")
	for i := 0; i < len(conds); i++ {
		tx = tx.Where(conds[i].ColName, conds[i].ColValue)
	}

	result := tx.Updates(values.AsMap())
	if result.RowsAffected == 0 {
		logrus.Warnf("UpdateBy: conds=%s, values=%s, no rows affected", conds, values)
	}
	return result.RowsAffected, nil
}

// UpdateItemById update item by id with fields values. It returns the number of rows affected and an error if any.
func UpdateItemById[T any](db *gorm.DB, id string, values FieldValues) (affected int64, err error) {
	return UpdateItem[T](db, NewQueryConds("id", id), values)
}

// UpdatesOrInsert update item or insert item if id not exists. It returns the number of rows affected and an error
// if any. Caller must be aware all fields of item T will be updated even not set.
func UpdatesOrInsert[T any](db *gorm.DB, item T) (affected int64, err error) {
	result := WrapDB(db).Model(&item).Save(&item)
	if err := result.Error; err != nil {
		return 0, logutil.LogError("UpdatesOrInsert: err=%v", err)
	}

	return result.RowsAffected, nil
}

// GetItem get one item with given conditions and options.
func GetItem[T any](db *gorm.DB, conds QueryConds, require bool, options ...QueryOption) (item T, found bool, err error) {
	tx := WrapDB(db)
	for _, cond := range conds {
		tx = tx.Where(cond.ColName, cond.ColValue)
	}

	for _, option := range options {
		if tx, err = option.WrapDB(tx); err != nil {
			return item, false, err
		}
	}

	if err := tx.Take(&item).Error; err != nil {
		if require {
			return item, false, logutil.LogError("GetItem: conds=%s, options=%v, err=%v", conds, options, err)
		} else {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return item, false, nil
			} else {
				return item, false, logutil.LogError("GetItem: conds=%s, options=%v, err=%v", conds, options, err)
			}
		}
	}

	return item, true, nil
}

// Get get items with given conditions and options.
func GetItems[T any](db *gorm.DB, conds QueryConds, require bool, options ...QueryOption) (items []T, found bool, err error) {
	tx := WrapDB(db)
	for _, cond := range conds {
		tx = tx.Where(cond.ColName, cond.ColValue)
	}

	for _, option := range options {
		if tx, err = option.WrapDB(tx); err != nil {
			return items, false, err
		}
	}

	if err := tx.Find(&items).Error; err != nil {
		if require {
			return items, false, logutil.LogError("GetItems: conds=%s, options=%v, err=%v", conds, options, err)
		} else {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return items, false, nil
			} else {
				return items, false, logutil.LogError("GetItems: conds=%s, options=%v, err=%v", conds, options, err)
			}
		}
	}

	return items, true, nil
}

// GetItemColumn get one item's column with given conditions and options.
func GetItemColumn[T any, V any](db *gorm.DB, conds QueryConds, require bool, options ...QueryOption) (item V, found bool, err error) {
	var table T
	tx := WrapDB(db).Model(&table)
	for _, cond := range conds {
		tx = tx.Where(cond.ColName, cond.ColValue)
	}

	for _, option := range options {
		if tx, err = option.WrapDB(tx); err != nil {
			return item, false, err
		}
	}

	if err := tx.Take(&item).Error; err != nil {
		if require {
			return item, false, logutil.LogError("GetItemColumn: conds=%s, options=%v, err=%v", conds, options, err)
		} else {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return item, false, nil
			} else {
				return item, false, logutil.LogError("GetItemColumn: conds=%s, options=%v, err=%v", conds, options, err)
			}
		}
	}

	return item, true, nil
}

// GetItemsColumn get items' column with given conditions and options.
func GetItemsColumn[T any, V any](db *gorm.DB, conds QueryConds, require bool, options ...QueryOption) (items []V, found bool, err error) {
	var table T
	tx := WrapDB(db).Model(&table)
	for _, cond := range conds {
		tx = tx.Where(cond.ColName, cond.ColValue)
	}

	for _, option := range options {
		if tx, err = option.WrapDB(tx); err != nil {
			return items, false, err
		}
	}

	if err := tx.Find(&items).Error; err != nil {
		if require {
			return items, false, logutil.LogError("GetItemsColumn: conds=%s, options=%v, err=%v", conds, options, err)
		} else {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return items, false, nil
			} else {
				return items, false, logutil.LogError("GetItemsColumn: conds=%s, options=%v, err=%v", conds, options, err)
			}
		}
	}

	return items, true, nil
}

// GetItemEx is a simple wrapper of GetItem but set require to false.
func GetItemEx[T any](db *gorm.DB, conds QueryConds, options ...QueryOption) (item T, found bool, err error) {
	return GetItem[T](db, conds, false, options...)
}

// GetItemsEx is a simple wrapper of GetItems but set require to false.
func GetItemsEx[T any](db *gorm.DB, conds QueryConds, options ...QueryOption) (items []T, found bool, err error) {
	return GetItems[T](db, conds, false, options...)
}

// GetAllItems get all items.
func GetAllItems[T any](db *gorm.DB) (items []T, err error) {
	items, _, err = GetItemsEx[T](db, QueryConds{})
	return items, err
}

// GetItemsColumnEx is a simple wrapper of GetItemsColumn but set require to false.
func GetItemsColumnEx[T any, V any](db *gorm.DB, conds QueryConds, options ...QueryOption) (items []V, found bool, err error) {
	return GetItemsColumn[T, V](db, conds, false, options...)
}

// GetItemById get item by id
func GetItemById[T any](db *gorm.DB, id string) (item T, found bool, err error) {
	return GetItem[T](db, NewQueryConds("id", id), false)
}

// GetRequiredItemById get item by id. If no item found, it will return an error.
func GetRequiredItemById[T any](db *gorm.DB, id string) (item T, err error) {
	item, _, err = GetItem[T](db, NewQueryConds("id", id), true)
	return item, err
}

// GetRequiredColumnById get column by id. If no item found, it will return an error.
func GetRequiredColumnById[T any, V any](db *gorm.DB, id string) (item V, err error) {
	item, _, err = GetItemColumn[T, V](db, NewQueryConds("id", id), true)
	return item, err
}

// GetItemColumnById get item column by id
func GetItemColumnById[T any, V any](db *gorm.DB, id string) (item V, found bool, err error) {
	return GetItemColumn[T, V](db, NewQueryConds("id", id), false)
}

// GetItemColumnByConds get item column by key value conditions
func GetItemColumnByConds[T any, V any](db *gorm.DB, conds QueryConds) (item V, found bool, err error) {
	return GetItemColumn[T, V](db, conds, false)
}

// GetRequiredItemColumn get required column by key value conditions
func GetRequiredItemColumn[T any, V any](db *gorm.DB, conds QueryConds) (item V, err error) {
	item, _, err = GetItemColumn[T, V](db, conds, true)
	return item, err
}

// GetRequiredItemsColumn get required column by key value conditions
func GetRequiredItemsColumn[T any, V any](db *gorm.DB, conds QueryConds, options ...QueryOption) (items []V, err error) {
	items, _, err = GetItemsColumn[T, V](db, conds, true, options...)
	return items, err
}

// GetRequiredItemByCond get required item with key value condition
func GetRequiredItemByCond[T any](db *gorm.DB, key string, val any) (item T, err error) {
	item, _, err = GetItem[T](db, NewQueryConds(key, val), true)
	return item, err
}

// GetItemsCountByConds get items count with key value array conditions
func GetItemsCountByConds[T any](db *gorm.DB, conds QueryConds) (count int64, err error) {
	var table T
	tx := WrapDB(db).Model(&table)
	for i := 0; i < len(conds); i++ {
		tx = tx.Where(conds[i].ColName, conds[i].ColValue)
	}

	if err = tx.Count(&count).Error; err != nil {
		return 0, logutil.LogError("GetItemsCountByConds: conds=%s, err=%v", conds, err)
	}

	return count, nil
}

// GetItemsCountByQuery get items count by query condition
func GetItemsCountByQuery[T any](db *gorm.DB, query interface{}, args ...interface{}) (count int64, err error) {
	var table T
	tx := WrapDB(db).Model(&table).Where(query, args...)
	if err = tx.Count(&count).Error; err != nil {
		return 0, logutil.LogError("GetItemsCountByRawQuery: query=%s, args=%v, err=%v", query, args, err)
	}

	return count, nil
}

// GetItemsCountByRawQuery get items count by raw sql query
func GetItemsCountByRawQuery[T any](db *gorm.DB, query string, args ...interface{}) (count int64, err error) {
	tx := WrapDB(db).Raw(query, args...)
	if err = tx.Count(&count).Error; err != nil {
		return 0, logutil.LogError("GetItemsCountByRawQuery: query=%s, args=%v, err=%v", query, args, err)
	}

	return count, nil
}

// GetRequiredItemAndUpdateById get item and update by id
func GetRequiredItemAndUpdateById[T any](db *gorm.DB, id string, values FieldValues) (item T, err error) {
	item, _, err = GetItem[T](db, NewQueryConds("id", id), true)
	if err != nil {
		return item, logutil.LogError("GetRequiredItemAndUpdateById: id=%s, err=%v", id, err)
	}

	_, err = UpdateItem[T](db, NewQueryConds("id", id), values)
	return item, err
}

// GetRequiredItemColumnAndUpdateById get item column and update by id. Since T and V are different, we need to use Model
// to specify the table.
func GetRequiredItemColumnAndUpdateById[T any, V any](db *gorm.DB, id string, values FieldValues) (item V, err error) {
	item, _, err = GetItemColumn[T, V](db, NewQueryConds("id", id), true)
	if err != nil {
		return item, logutil.LogError("GetRequiredItemColumnAndUpdateById: id=%s, err=%v", id, err)
	}

	_, err = UpdateItem[T](db, NewQueryConds("id", id), values)
	return item, err
}

// GetItemByQuery get item by query
func GetItemByQuery[T any](db *gorm.DB, query interface{}, args ...interface{}) (item T, found bool, err error) {
	if err := WrapDB(db).Where(query, args...).Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return item, false, nil
		} else {
			return item, false, logutil.LogError("GetItemByQuery: query=%v, args=%v, err=%v", query, args, err)
		}
	}

	return item, true, nil
}

// GetRequiredItemByQuery get required item by query condition. If not found, return error.
func GetRequiredItemByQuery[T any](db *gorm.DB, query interface{}, args ...interface{}) (item T, err error) {
	item, found, err := GetItemByQuery[T](db, query, args...)
	if err != nil {
		return item, err
	}

	if !found {
		return item, logutil.LogError("Record not found: query=%v, args=%v", query, args)
	}

	return item, nil
}

// GetItemCountByQuery get item count by query condition
func GetItemCountByQuery[T any](db *gorm.DB, query interface{}, args ...interface{}) (count int64, err error) {
	var table T
	tx := WrapDB(db).Model(&table).Where(query, args...)
	if err = tx.Count(&count).Error; err != nil {
		return 0, logutil.LogError("GetItemCountByQuery: query=%v, args=%v, err=%v", query, args, err)
	}

	return count, nil
}

// GetItemByRawQuery get item by raw sql query
func GetItemByRawQuery[T any](db *gorm.DB, query string, args ...interface{}) (item T, found bool, err error) {
	if err := WrapDB(db).Raw(query, args...).Scan(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return item, false, nil
		} else {
			return item, false, logutil.LogError("GetItemByRawQuery: query=%v, args=%v, err=%v", query, args, err)
		}
	}

	return item, true, nil
}

// GetItemsByRawQuery get items by raw sql query
func GetItemsByRawQuery[T any](db *gorm.DB, option QueryOption, query string, args ...interface{}) (items []T, found bool, err error) {
	qoSql, err := option.Sql()
	if err != nil {
		return items, false, err
	}

	tx := WrapDB(db).Raw(query+" "+qoSql, args...)
	if tx, err = option.WrapDB(tx); err != nil {
		return items, false, err
	}

	if err := tx.Scan(&items).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return items, false, nil
		} else {
			return items, false, logutil.LogError("GetItemsByRawQuery: query=%v, args=%v, err=%v", query, args, err)
		}
	}

	return items, true, nil
}

// DeleteItemById delete item by id
func DeleteItemById[T any](db *gorm.DB, id string) (affected int64, err error) {
	var item T
	result := WrapDB(db).Where("id", id).Delete(&item)
	if result.Error != nil {
		return 0, logutil.LogError("DeleteItemById: id=%s, err=%v", id, result.Error)
	}

	if result.RowsAffected == 0 {
		logrus.Warnf("DeleteItemById: id=%s, no rows affected", id)
	}

	return result.RowsAffected, nil
}

// DeleteItemByConds delete item by key value conditions
func DeleteItemByConds[T any](db *gorm.DB, conds QueryConds) (affected int64, err error) {
	var item T
	tx := WrapDB(db)
	for i := 0; i < len(conds); i++ {
		tx = tx.Where(conds[i].ColName, conds[i].ColValue)
	}

	if err := tx.Delete(&item).Error; err != nil {
		return 0, logutil.LogError("DeleteItemByCond: conds=%s, err=%v", conds, err)
	}

	if tx.RowsAffected == 0 {
		logrus.Warnf("DeleteItemByCond: conds=%s, no rows affected", conds)
	}

	return tx.RowsAffected, nil
}

// DeleteItemByRawQuery delete item by raw sql query condition
func DeleteItemByRawQuery[T any](db *gorm.DB, query string, args ...interface{}) (affected int64, err error) {
	var item T
	result := WrapDB(db).Delete(&item).Where(query, args...)
	if result.Error != nil {
		return 0, logutil.LogError("DeleteItemByRawQuery: query=%s, args=%v, err=%v", query, args, result.Error)
	}

	if result.RowsAffected == 0 {
		logrus.Warnf("DeleteItemByRawQuery: query=%s, args=%v, no rows affected", query, args)
	}

	return result.RowsAffected, nil
}
