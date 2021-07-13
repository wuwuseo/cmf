package dao

import "github.com/wuwuseo/cmf/internal/model"

func (d *Dao) CountUser() (int, error) {
	user := model.User{}
	return user.Count(d.engine)
}
