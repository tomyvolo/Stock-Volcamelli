package database

import (
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ConnectDB devuelve la conexión de GORM al MySQL de Usuarios.
// En Docker, DB_DSN apunta al servicio mysql-usuarios.
func ConnectDB() *gorm.DB {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "usuarios:usuarios_local@tcp(localhost:3306)/usuarios?parseTime=true"
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("no se pudo conectar al MySQL de Usuarios: %v", err)
	}
	log.Println("Conectado a MySQL!")
	return db
}
