package database

import (
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ConnectDB devuelve la conexión de GORM al MySQL de Sucursales.
// En Docker, DB_DSN apunta al servicio mysql-sucursales.
func ConnectDB() *gorm.DB {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "sucursales:sucursales_local@tcp(localhost:3306)/sucursales?parseTime=true"
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("no se pudo conectar al MySQL de Sucursales: %v", err)
	}
	log.Println("Conectado a MySQL!")
	return db
}
