package database

import (
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ConnectDB devuelve la conexión de GORM al MySQL de Inventario.
// En Docker, DB_DSN apunta al servicio mysql-inventario.
func ConnectDB() *gorm.DB {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "inventario:inventario_local@tcp(localhost:3306)/inventario?parseTime=true"
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("no se pudo conectar al MySQL de Inventario: %v", err)
	}
	log.Println("Conectado a MySQL!")
	return db
}
