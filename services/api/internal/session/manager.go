package session

import (
	"fmt"
	"net/http"
	"time"

	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"
	"gorm.io/gorm"
)

func New(database *gorm.DB, secure bool) (*scs.SessionManager, error) {
	sqlDatabase, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("get session database: %w", err)
	}
	manager := scs.New()
	manager.Store = postgresstore.New(sqlDatabase)
	manager.Lifetime = 24 * time.Hour
	manager.Cookie.HttpOnly = true
	manager.Cookie.SameSite = http.SameSiteLaxMode
	manager.Cookie.Secure = secure
	return manager, nil
}
