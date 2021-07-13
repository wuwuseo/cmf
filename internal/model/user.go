package model

import "github.com/jinzhu/gorm"

type User struct {
}

func NewUser() User {
	return User{}
}

func (t User) Count(db *gorm.DB) (int, error) {
	var count int
	if err := db.Model(&t).Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}
