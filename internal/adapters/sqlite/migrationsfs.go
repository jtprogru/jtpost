package sqlite

import "io/fs"

// MigrationsFS возвращает встроенную FS с миграциями для использования
// внешним кодом (cli/migrate_db.go).
func MigrationsFS() fs.FS { return migrationsFS }
