package repo

import "gorm.io/gorm"

// DB is a type alias for *gorm.DB. It exists so that api/* handlers can
// reference the database handle without importing gorm.io/gorm directly.
// All GORM methods are still accessible on DB — use handler-audit (T6.4.1)
// to prevent direct GORM calls from creeping back into handlers.
type DB = *gorm.DB

// NewDB wraps a *gorm.DB into a DB (identity, for clarity at call sites).
func NewDB(g *gorm.DB) DB { return g }
