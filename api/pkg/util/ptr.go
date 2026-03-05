package util

func Ptr[T any](v T) *T {
	return &v
}

func DerefPtr[T any](p *T, defaultValue T) T {
	if p == nil {
		return defaultValue
	}
	return *p
}
