package facade

type DataManager[T any] interface {
	SaveData(tableName, key string, value T) error
	LoadData(tableName, key string) (T, error)
	GetKey(item ...string) string
}
