package facade

import "context"

type DataManager[T any] interface {
	GetData(ctx context.Context, factory func() T, key ...string) (T, bool)
	SaveData(ctx context.Context, data T, key ...string) error
}
