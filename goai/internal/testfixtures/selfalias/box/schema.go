package box

import models "github.com/yetiz-org/gone/goai/internal/testfixtures/collision/second/models"

type User struct {
	OwnID string `json:"own_id"`
}

type Box struct {
	Data models.User `json:"data"`
}
