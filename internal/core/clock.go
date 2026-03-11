package core

import "time"

// Clock интерфейс для работы со временем (удобно для тестирования).
type Clock interface {
	Now() time.Time
}

// SystemClock реализация Clock через time.Now().
type SystemClock struct{}

// Now возвращает текущее время.
func (c SystemClock) Now() time.Time {
	return time.Now()
}
