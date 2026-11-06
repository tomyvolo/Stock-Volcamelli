package registro

import (
	"context"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// MySQL completa id y created_at automáticamente.
type Log struct {
	TrackID string `gorm:"size:32;not null;default:'';index:idx_app_logs_track_id"`
	Stage   string `gorm:"size:20;not null;default:'api';index:idx_app_logs_stage"`
	Level   string `gorm:"size:10;not null"`
	Message string `gorm:"type:text;not null"`
}

type Store struct{ db *gorm.DB }

func Connect() (*Store, error) {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = "demo:demo@tcp(localhost:3306)/observabilidad?parseTime=true"
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.Table("app_logs").AutoMigrate(&Log{}); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// writeTimeout es lo máximo que un pedido espera a que se guarde un log. El gateway les da 2 segundos a los
// servicios, y un MySQL de logs caído o saturado no tiene que hacer fallar un pedido.
const writeTimeout = 200 * time.Millisecond

// Write guarda un log en la tabla que lee Grafana, con el track_id y la etapa cuando corresponda.
// Si no se puede guardar a tiempo, el log completo queda en la consola y el pedido sigue.
func (store *Store) Write(trackID, stage, level, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	err := store.db.WithContext(ctx).Table("app_logs").Create(&Log{TrackID: trackID, Stage: stage, Level: level, Message: message}).Error
	if err != nil {
		log.Printf("no se pudo guardar el log en MySQL: %v. Log: track_id=%q etapa=%s nivel=%s %s", err, trackID, stage, level, message)
	}
}

// Escribir es lo que usa el centro (puertos.Registro): todo lo de Transferencias va con la etapa "transferencias".
func (store *Store) Escribir(trackID, nivel, mensaje string) {
	store.Write(trackID, "transferencias", nivel, mensaje)
}
